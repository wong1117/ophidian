package mission

import "context"

type MissionRepository interface {
	Save(ctx context.Context, mission *Mission) error
	FindByID(ctx context.Context, id string) (*Mission, error)
	FindAll(ctx context.Context, filter MissionFilter) ([]*Mission, error)
	Update(ctx context.Context, mission *Mission) error
	Delete(ctx context.Context, id string) error
	SaveTask(ctx context.Context, task *Task) error
	FindTaskByID(ctx context.Context, id string) (*Task, error)
	FindTasksByMission(ctx context.Context, missionID string) ([]*Task, error)
	UpdateTask(ctx context.Context, task *Task) error
}

type MissionFilter struct {
	Status *MissionStatus
	Limit  int
	Offset int
}
