package session

import "context"

type SessionRepository interface {
	Save(ctx context.Context, session *Session) error
	FindByID(ctx context.Context, id string) (*Session, error)
	FindByMission(ctx context.Context, missionID string) ([]*Session, error)
	FindByTarget(ctx context.Context, targetID string) ([]*Session, error)
	FindActive(ctx context.Context) ([]*Session, error)
	Update(ctx context.Context, session *Session) error
	Delete(ctx context.Context, id string) error
}
