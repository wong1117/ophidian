package controlplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type EnforceRoEUseCase struct {
	missionRepo mission.MissionRepository
}

func NewEnforceRoEUseCase(repo mission.MissionRepository) *EnforceRoEUseCase {
	return &EnforceRoEUseCase{missionRepo: repo}
}

func (uc *EnforceRoEUseCase) Execute(ctx context.Context, missionID string, action string, targetIP string) error {
	m, err := uc.missionRepo.FindByID(ctx, missionID)
	if err != nil {
		return err
	}

	if err := mission.ValidateRoE(m.RoE, m.Target); err != nil {
		return err
	}

	return nil
}
