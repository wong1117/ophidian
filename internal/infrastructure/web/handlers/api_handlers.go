package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/application/aiplane"
	"github.com/ophidian/ophidian/internal/application/executionplane/exploit"
	reportApp "github.com/ophidian/ophidian/internal/application/executionplane/report"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/session"
	"github.com/ophidian/ophidian/internal/domain/target"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type ReconHandler struct {
	targetRepo  target.TargetRepository
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
	matchUC   *exploit.MatchExploitUseCase
	executeUC *exploit.ExecuteExploitUseCase
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
	planUC *aiplane.GeneratePlanUseCase
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
