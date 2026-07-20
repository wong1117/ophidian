package controlplane

import (
	"context"
	"log"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type CreateMissionUseCase struct {
	missionRepo mission.MissionRepository
	eventStore  EventStore
	dispatcher  EventDispatcher
}

func NewCreateMissionUseCase(
	repo mission.MissionRepository,
	es EventStore,
	dispatcher EventDispatcher,
) *CreateMissionUseCase {
	return &CreateMissionUseCase{
		missionRepo: repo,
		eventStore:  es,
		dispatcher:  dispatcher,
	}
}

func (uc *CreateMissionUseCase) Execute(ctx context.Context, req mission.CreateMissionRequest) (*mission.Mission, error) {
	if err := mission.ValidateRoE(req.RoE, req.Target); err != nil {
		return nil, err
	}

	m := &mission.Mission{
		ID:         common.NewID(),
		Name:       req.Name,
		Target:     req.Target,
		Objectives: req.Objectives,
		RoE:        req.RoE,
		Status:     mission.MissionDraft,
		CreatedAt:  common.Now(),
		UpdatedAt:  common.Now(),
		StartedBy:  req.StartedBy,
	}

	agg := mission.NewMissionAggregate(m)
	if err := agg.Start(); err != nil {
		return nil, err
	}

	if err := uc.missionRepo.Save(ctx, m); err != nil {
		return nil, err
	}

	for _, event := range agg.Events {
		if err := uc.eventStore.Append(ctx, event); err != nil {
			return nil, err
		}
	}

	if uc.dispatcher != nil {
		for _, event := range agg.Events {
			if err := uc.dispatcher.Dispatch(ctx, event); err != nil {
				log.Printf("WARNING: failed to dispatch event: %v", err)
			}
		}
	}

	return m, nil
}
