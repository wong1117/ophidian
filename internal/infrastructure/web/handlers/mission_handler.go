package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ophidian/ophidian/internal/application/controlplane"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type MissionHandler struct {
	createUC       *controlplane.CreateMissionUseCase
	orchestrateUC  *controlplane.OrchestrateMissionUseCase
	missionRepo    mission.MissionRepository
}

func NewMissionHandler(
	createUC *controlplane.CreateMissionUseCase,
	orchestrateUC *controlplane.OrchestrateMissionUseCase,
	missionRepo mission.MissionRepository,
) *MissionHandler {
	return &MissionHandler{
		createUC:      createUC,
		orchestrateUC: orchestrateUC,
		missionRepo:   missionRepo,
	}
}

func (h *MissionHandler) Create(c echo.Context) error {
	var req dto.CreateMissionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	m, err := h.createUC.Execute(context.Background(), mission.CreateMissionRequest{
		Name:       req.Name,
		Target:     toDomainTarget(req.Target),
		Objectives: toDomainObjectives(req.Objectives),
		RoE:        toDomainRoE(req.RoE),
		StartedBy:  req.StartedBy,
	})
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, mapMissionToDTO(m))
}

func (h *MissionHandler) Get(c echo.Context) error {
	id := c.Param("id")
	m, err := h.missionRepo.FindByID(context.Background(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapMissionToDTO(m))
}

func (h *MissionHandler) List(c echo.Context) error {
	ctx := context.Background()
	missions, err := h.missionRepo.FindAll(ctx, mission.MissionFilter{})
	if err != nil {
		return err
	}

	result := make([]dto.MissionResponse, len(missions))
	for i, m := range missions {
		result[i] = *mapMissionToDTO(m)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *MissionHandler) Start(c echo.Context) error {
	id := c.Param("id")
	m, err := h.missionRepo.FindByID(context.Background(), id)
	if err != nil {
		return err
	}

	agg := mission.NewMissionAggregate(m)
	if err := agg.Start(); err != nil {
		return err
	}
	if err := h.missionRepo.Update(context.Background(), m); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapMissionToDTO(m))
}

func (h *MissionHandler) Abort(c echo.Context) error {
	id := c.Param("id")
	resp, err := h.orchestrateUC.Execute(context.Background(), controlplane.OrchestrateRequest{
		MissionID: id,
		Action:    controlplane.ActionFail,
		Reason:    "user aborted",
		UpdatedBy: "api",
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp.Mission)
}

func toDomainTarget(t dto.TargetDTO) mission.Target {
	return mission.Target{
		Name:    t.Name,
		IPs:     t.IPs,
		Domains: t.Domains,
		CIDRs:   t.CIDRs,
	}
}

func toDomainObjectives(objs []dto.ObjectiveDTO) []mission.Objective {
	result := make([]mission.Objective, len(objs))
	for i, o := range objs {
		result[i] = mission.Objective{
			Description: o.Description,
			Priority:    o.Priority,
		}
	}
	return result
}

func toDomainRoE(r dto.RoEDTO) mission.RoEConstraints {
	return mission.RoEConstraints{
		MaxSeverity:      mission.Severity(r.MaxSeverity),
		AllowDestructive: r.AllowDestructive,
		AllowPersistence: r.AllowPersistence,
		AllowExfiltration: r.AllowExfiltration,
		MaxTargets:       r.MaxTargets,
	}
}

func mapMissionToDTO(m *mission.Mission) *dto.MissionResponse {
	return &dto.MissionResponse{
		ID:        m.ID.String(),
		Name:      m.Name,
		Status:    string(m.Status),
		CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
