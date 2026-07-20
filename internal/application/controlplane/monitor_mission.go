package controlplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type MonitorMissionUseCase struct {
	missionRepo mission.MissionRepository
}

func NewMonitorMissionUseCase(repo mission.MissionRepository) *MonitorMissionUseCase {
	return &MonitorMissionUseCase{missionRepo: repo}
}

func (uc *MonitorMissionUseCase) Execute(ctx context.Context, missionID string) (*mission.Mission, error) {
	return uc.missionRepo.FindByID(ctx, missionID)
}

func (uc *MonitorMissionUseCase) ListActive(ctx context.Context) ([]*mission.Mission, error) {
	return uc.missionRepo.FindAll(ctx, mission.MissionFilter{
		Status: &[]mission.MissionStatus{mission.MissionActive}[0],
	})
}
