package controlplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type TaskDispatcher interface {
	Dispatch(ctx context.Context, task *mission.Task) error
}

type DispatchTaskUseCase struct {
	missionRepo mission.MissionRepository
	dispatcher  TaskDispatcher
	eventStore  EventStore
}

func NewDispatchTaskUseCase(repo mission.MissionRepository, d TaskDispatcher, es EventStore) *DispatchTaskUseCase {
	return &DispatchTaskUseCase{missionRepo: repo, dispatcher: d, eventStore: es}
}

func (uc *DispatchTaskUseCase) Execute(ctx context.Context, taskID string) error {
	task, err := uc.missionRepo.FindTaskByID(ctx, taskID)
	if err != nil {
		return err
	}

	task.Status = mission.TaskRunning
	started := common.Now()
	task.StartedAt = &started

	if err := uc.missionRepo.UpdateTask(ctx, task); err != nil {
		return err
	}

	if err := uc.dispatcher.Dispatch(ctx, task); err != nil {
		task.Status = mission.TaskFailed
		_ = uc.missionRepo.UpdateTask(ctx, task)
		return err
	}

	event := mission.TaskDispatched{
		MissionID:    task.MissionID,
		TaskID:       task.ID,
		TaskType:     string(task.Type),
		Parameters:   task.Parameters,
		ToPlane:      common.PlaneExecution,
		DispatchedAt: common.Now(),
	}
	return uc.eventStore.Append(ctx, event)
}
