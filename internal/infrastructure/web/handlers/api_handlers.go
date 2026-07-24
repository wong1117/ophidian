package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/application/aiplane"
	"github.com/ophidian/ophidian/internal/application/executionplane/exploit"
	reportApp "github.com/ophidian/ophidian/internal/application/executionplane/report"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/domain/session"
	"github.com/ophidian/ophidian/internal/domain/target"
	"github.com/ophidian/ophidian/internal/infrastructure/dispatcher"
	"github.com/ophidian/ophidian/internal/infrastructure/persistence/postgres"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type ReconHandler struct {
	targetRepo target.TargetRepository
}

func NewReconHandler(targetRepo target.TargetRepository) *ReconHandler {
	return &ReconHandler{targetRepo: targetRepo}
}

func (h *ReconHandler) StartPassive(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "passive recon started"})
}

func (h *ReconHandler) StartActive(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "active recon started"})
}

func (h *ReconHandler) GetResults(c echo.Context) error {
	id := c.Param("id")
	t, err := h.targetRepo.FindByID(context.Background(), id)
	if err != nil {
		return err
	}

	var services []dto.ServiceDTO
	for _, s := range t.Services {
		services = append(services, dto.ServiceDTO{
			Port:     s.Port,
			Protocol: s.Protocol,
			Name:     s.Name,
			Version:  s.Version,
			Banner:   s.Banner,
		})
	}

	return c.JSON(http.StatusOK, dto.ReconResultResponse{
		TargetID: t.ID.String(),
		IPs:      ipToStrings(t.IPs),
		Domains:  domainsToStrings(t.Domains),
		Services: services,
		OS:       t.OS,
		Status:   "completed",
	})
}

func ipToStrings(ips []target.IP) []string {
	r := make([]string, len(ips))
	for i, ip := range ips {
		r[i] = ip.Address
	}
	return r
}

func domainsToStrings(domains []target.Domain) []string {
	r := make([]string, len(domains))
	for i, d := range domains {
		r[i] = d.Name
	}
	return r
}

type ExploitHandler struct {
	matchUC     *exploit.MatchExploitUseCase
	executeUC   *exploit.ExecuteExploitUseCase
	sessionRepo session.SessionRepository
}

func NewExploitHandler(
	matchUC *exploit.MatchExploitUseCase,
	executeUC *exploit.ExecuteExploitUseCase,
	sessionRepo session.SessionRepository,
) *ExploitHandler {
	return &ExploitHandler{matchUC: matchUC, executeUC: executeUC, sessionRepo: sessionRepo}
}

func (h *ExploitHandler) Match(c echo.Context) error {
	var req dto.MatchExploitRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	svc := target.Service{
		Name:     req.Service.Name,
		Version:  req.Service.Version,
		Port:     req.Service.Port,
		Protocol: req.Service.Protocol,
	}

	modules, err := h.matchUC.Execute(context.Background(), svc)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, modules)
}

func (h *ExploitHandler) Execute(c echo.Context) error {
	var req dto.ExecuteExploitRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	resp, err := h.executeUC.Execute(context.Background(), exploit.ExecuteExploitRequest{
		Module: exploit.ExploitModule{
			ID:      req.ModuleID,
			Service: "",
		},
		Target:    req.Target,
		Port:      req.Port,
		MissionID: req.MissionID,
		Options:   req.Options,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp.Result)
}

func (h *ExploitHandler) ListSessions(c echo.Context) error {
	sessions, err := h.sessionRepo.FindActive(context.Background())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, sessions)
}

type AIHandler struct {
	planUC   *aiplane.GeneratePlanUseCase
	planRepo attackplan.AttackPlanRepository
}

func NewAIHandler(planUC *aiplane.GeneratePlanUseCase, planRepo attackplan.AttackPlanRepository) *AIHandler {
	return &AIHandler{planUC: planUC, planRepo: planRepo}
}

func (h *AIHandler) GeneratePlan(c echo.Context) error {
	var req dto.GeneratePlanRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.planUC.Execute(context.Background(), aiplane.ExecuteRequest{MissionID: req.MissionID})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp.Plan)
}

func (h *AIHandler) GetPlan(c echo.Context) error {
	id := c.Param("id")
	p, err := h.planRepo.FindByID(context.Background(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, p)
}

func (h *AIHandler) Correlate(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "correlation requested"})
}

type ReportHandler struct {
	reportUC *reportApp.GenerateReportUseCase
}

func NewReportHandler(reportUC *reportApp.GenerateReportUseCase) *ReportHandler {
	return &ReportHandler{reportUC: reportUC}
}

func (h *ReportHandler) Generate(c echo.Context) error {
	var req dto.GenerateReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.reportUC.Execute(context.Background(), reportApp.GenerateReportRequest{
		MissionID: req.MissionID,
		Format:    req.Format,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp.Report)
}

func (h *ReportHandler) Export(c echo.Context) error {
	format := c.Param("format")
	return c.JSON(http.StatusOK, map[string]string{
		"format": format,
		"status": "exported",
	})
}

type RecommendationHandler struct {
	eventStore *postgres.EventStore
	dispatcher *dispatcher.HTTPEventDispatcher
}

func NewRecommendationHandler(eventStore *postgres.EventStore, dispatcher *dispatcher.HTTPEventDispatcher) *RecommendationHandler {
	return &RecommendationHandler{eventStore: eventStore, dispatcher: dispatcher}
}

func (h *RecommendationHandler) List(c echo.Context) error {
	ctx := context.Background()
	from := time.Now().UTC().Add(-24 * time.Hour)
	to := time.Now().UTC()

	records, err := h.eventStore.LoadAllEvents(ctx, from, to)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to load events: %v", err))
	}

	var recs []dto.RecommendationDTO
	for _, r := range records {
		if r.EventType != "AIRecommendationGenerated" {
			continue
		}
		var rec mission.AIRecommendationEvent
		if err := json.Unmarshal(r.Payload, &rec); err != nil {
			continue
		}
		recs = append(recs, dto.RecommendationDTO{
			EventID:        r.ID,
			MissionID:      rec.MissionID.String(),
			SourceEventID:  rec.SourceEventID,
			Recommendation: rec.Recommendation,
			Confidence:     rec.Confidence,
			GeneratedAt:    rec.GeneratedAt.Time,
		})
	}

	if recs == nil {
		recs = []dto.RecommendationDTO{}
	}
	return c.JSON(http.StatusOK, recs)
}

func (h *RecommendationHandler) Approve(c echo.Context) error {
	var req dto.ApprovalRequestDTO
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.MissionID == "" || req.SourceEventID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "mission_id and source_event_id are required")
	}

	decision := common.PlanDecision(req.Decision)
	if decision != common.PlanAccepted && decision != common.PlanRejected && decision != common.PlanModified {
		return echo.NewHTTPError(http.StatusBadRequest, "decision must be ACCEPTED, REJECTED, or MODIFIED")
	}

	approval := mission.OperatorApprovalEvent{
		MissionID:     common.ID(req.MissionID),
		SourceEventID: req.SourceEventID,
		Decision:      decision,
		OperatorNote:  req.OperatorNote,
		ApprovedAt:    common.Now(),
	}

	payloadBytes, _ := json.Marshal(approval)
	record := postgres.EventRecord{
		ID:            approval.EventID(),
		AggregateID:   approval.AggregateID(),
		AggregateType: "mission",
		EventType:     approval.EventType(),
		Payload:       payloadBytes,
		OccurredAt:    approval.ApprovedAt.Time,
	}

	ctx := context.Background()
	if err := h.eventStore.Append(ctx, -1, record); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to append approval event: %v", err))
	}

	if h.dispatcher != nil {
		if err := h.dispatcher.Dispatch(ctx, approval); err != nil {
			fmt.Printf("SERVER: WARNING: failed to dispatch approval to worker: %v\n", err)
		}
	}

	return c.JSON(http.StatusOK, dto.ApprovalResponseDTO{
		Status:  "appended",
		EventID: approval.EventID(),
	})
}
