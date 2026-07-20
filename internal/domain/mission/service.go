package mission

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type MissionService interface {
	CreateMission(ctx context.Context, req CreateMissionRequest) (*Mission, error)
	StartMission(ctx context.Context, id string) error
	PauseMission(ctx context.Context, id string) error
	AbortMission(ctx context.Context, id string, reason string) error
	CompleteMission(ctx context.Context, id string) error
	TransitionPhase(ctx context.Context, missionID string, toPhase common.Phase, triggeredBy, reason string) error
	DispatchTask(ctx context.Context, task *Task) error
	CompleteTask(ctx context.Context, taskID string, result *TaskResult) error
	FailTask(ctx context.Context, taskID string, err error) error
	GetMissionStatus(ctx context.Context, id string) (*Mission, error)
}

type CreateMissionRequest struct {
	Name       string
	Target     Target
	Objectives []Objective
	RoE        RoEConstraints
	StartedBy  string
}
