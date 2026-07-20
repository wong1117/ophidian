package controlplane

import (
	"context"
	"sort"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type ScheduleTaskUseCase struct {
	missionRepo mission.MissionRepository
	eventStore  EventStore
}

func NewScheduleTaskUseCase(repo mission.MissionRepository, es EventStore) *ScheduleTaskUseCase {
	return &ScheduleTaskUseCase{missionRepo: repo, eventStore: es}
}

func (uc *ScheduleTaskUseCase) Execute(ctx context.Context, missionID string, tasks []*mission.Task) error {
	m, err := uc.missionRepo.FindByID(ctx, missionID)
	if err != nil {
		return err
	}

	sorted := sortByPriority(tasks)
	for _, task := range sorted {
		task.Status = mission.TaskPending
		m.Tasks = append(m.Tasks, *task)
		if err := uc.missionRepo.SaveTask(ctx, task); err != nil {
			return err
		}
	}

	m.UpdatedAt = common.Now()
	return uc.missionRepo.Update(ctx, m)
}

func sortByPriority(tasks []*mission.Task) []*mission.Task {
	sorted := make([]*mission.Task, len(tasks))
	copy(sorted, tasks)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority > sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}
