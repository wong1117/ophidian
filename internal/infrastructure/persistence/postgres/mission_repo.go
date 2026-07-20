package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type MissionRepository struct {
	pool *pgxpool.Pool
}

func NewMissionRepository(pool *pgxpool.Pool) *MissionRepository {
	return &MissionRepository{pool: pool}
}

func (r *MissionRepository) Save(ctx context.Context, m *mission.Mission) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO missions (id, name, status, target, roe, created_at, updated_at, started_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		m.ID, m.Name, m.Status, m.Target, m.RoE, m.CreatedAt, m.UpdatedAt, m.StartedBy,
	)
	return err
}

func (r *MissionRepository) FindByID(ctx context.Context, id string) (*mission.Mission, error) {
	return nil, nil
}

func (r *MissionRepository) FindAll(ctx context.Context, filter mission.MissionFilter) ([]*mission.Mission, error) {
	return nil, nil
}

func (r *MissionRepository) Update(ctx context.Context, m *mission.Mission) error {
	return nil
}

func (r *MissionRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *MissionRepository) SaveTask(ctx context.Context, task *mission.Task) error {
	return nil
}

func (r *MissionRepository) FindTaskByID(ctx context.Context, id string) (*mission.Task, error) {
	return nil, nil
}

func (r *MissionRepository) FindTasksByMission(ctx context.Context, missionID string) ([]*mission.Task, error) {
	return nil, nil
}

func (r *MissionRepository) UpdateTask(ctx context.Context, task *mission.Task) error {
	return nil
}
