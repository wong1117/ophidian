package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/session"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Save(ctx context.Context, s *session.Session) error {
	return nil
}

func (r *SessionRepository) FindByID(ctx context.Context, id string) (*session.Session, error) {
	return nil, nil
}

func (r *SessionRepository) FindByMission(ctx context.Context, missionID string) ([]*session.Session, error) {
	return nil, nil
}

func (r *SessionRepository) FindByTarget(ctx context.Context, targetID string) ([]*session.Session, error) {
	return nil, nil
}

func (r *SessionRepository) FindActive(ctx context.Context) ([]*session.Session, error) {
	return nil, nil
}

func (r *SessionRepository) Update(ctx context.Context, s *session.Session) error {
	return nil
}

func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	return nil
}
