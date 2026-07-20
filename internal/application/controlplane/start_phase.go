package controlplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type StartPhaseUseCase struct {
	missionRepo mission.MissionRepository
	eventStore  EventStore
}

func NewStartPhaseUseCase(repo mission.MissionRepository, es EventStore) *StartPhaseUseCase {
	return &StartPhaseUseCase{missionRepo: repo, eventStore: es}
}

func (uc *StartPhaseUseCase) Execute(ctx context.Context, missionID string, phase common.Phase) error {
	m, err := uc.missionRepo.FindByID(ctx, missionID)
	if err != nil {
		return err
	}

	agg := mission.NewMissionAggregate(m)
	if err := agg.TransitionPhase(phase, phase, "phase started"); err != nil {
		return err
	}

	if err := uc.missionRepo.Update(ctx, m); err != nil {
		return err
	}

	for _, event := range agg.Events {
		if err := uc.eventStore.Append(ctx, event); err != nil {
			return err
		}
	}

	return nil
}
