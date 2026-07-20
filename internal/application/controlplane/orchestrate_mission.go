package controlplane

import (
	"context"
	"fmt"
	"os"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/interfaces/dto"
)

type OrchestrateMissionUseCase struct {
	missionRepo mission.MissionRepository
	eventStore  EventStore
	dispatcher  EventDispatcher
}

func NewOrchestrateMissionUseCase(
	missionRepo mission.MissionRepository,
	eventStore EventStore,
	dispatcher EventDispatcher,
) *OrchestrateMissionUseCase {
	return &OrchestrateMissionUseCase{
		missionRepo: missionRepo,
		eventStore:  eventStore,
		dispatcher:  dispatcher,
	}
}

type OrchestrateRequest struct {
	MissionID string
	Action    LifecycleAction
	UpdatedBy string
	Reason    string
}

type LifecycleAction string

const (
	ActionPlan   LifecycleAction = "PLAN"
	ActionReady  LifecycleAction = "READY"
	ActionRun    LifecycleAction = "RUN"
	ActionComplete LifecycleAction = "COMPLETE"
	ActionFail   LifecycleAction = "FAIL"
)

type OrchestrateResponse struct {
	Mission *dto.MissionLifecycleResponse
}

func (uc *OrchestrateMissionUseCase) Execute(ctx context.Context, req OrchestrateRequest) (*OrchestrateResponse, error) {
	if req.MissionID == "" {
		return nil, fmt.Errorf("%w: mission id is required", common.ErrInvalidID)
	}
	if req.Action == "" {
		return nil, fmt.Errorf("%w: lifecycle action is required", common.ErrInvalidState)
	}

	m, err := uc.missionRepo.FindByID(ctx, req.MissionID)
	if err != nil {
		return nil, fmt.Errorf("fetch mission: %w", err)
	}

	agg := mission.NewMissionAggregate(m)

	if err := uc.applyAction(agg, req); err != nil {
		return nil, err
	}

	if err := uc.missionRepo.Update(ctx, m); err != nil {
		return nil, fmt.Errorf("persist mission: %w", err)
	}

	for _, event := range agg.Events {
		if err := uc.eventStore.Append(ctx, event); err != nil {
			return nil, fmt.Errorf("append event: %w", err)
		}
	}

	if uc.dispatcher != nil {
		for _, event := range agg.Events {
			if err := uc.dispatcher.Dispatch(ctx, event); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: failed to dispatch event: %v\n", err)
			}
		}
	}

	return &OrchestrateResponse{
		Mission: mapToMissionLifecycleDTO(m),
	}, nil
}

func (uc *OrchestrateMissionUseCase) applyAction(agg *mission.MissionAggregate, req OrchestrateRequest) error {
	switch req.Action {
	case ActionPlan:
		return agg.TransitionToPlanning(req.UpdatedBy)
	case ActionReady:
		return agg.TransitionToReady(req.UpdatedBy)
	case ActionRun:
		return agg.TransitionToRunning(req.UpdatedBy)
	case ActionComplete:
		return agg.Complete(req.UpdatedBy)
	case ActionFail:
		return agg.Fail(req.Reason, req.UpdatedBy)
	default:
		return fmt.Errorf("%w: unknown lifecycle action %s", common.ErrInvalidState, req.Action)
	}
}

func mapToMissionLifecycleDTO(m *mission.Mission) *dto.MissionLifecycleResponse {
	return &dto.MissionLifecycleResponse{
		ID:        m.ID.String(),
		Name:      m.Name,
		Status:    string(m.Status),
		StartedBy: m.StartedBy,
		CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: m.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
