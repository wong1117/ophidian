package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/session"
)

type SessionRepository struct {
	deps RepoDeps
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{deps: repoDepsFromPool(pool)}
}

func NewSessionRepositoryWithDeps(deps RepoDeps) *SessionRepository {
	return &SessionRepository{deps: deps}
}

func (r *SessionRepository) Save(ctx context.Context, s *session.Session) error {
	metadataJSON := marshalJSON(s.Metadata)

	_, err := r.deps.Exec(ctx,
		`INSERT INTO sessions (id, mission_id, target_id, type, protocol, host, port, "user",
		 privilege_level, status, encryption, established_at, last_active_at, closed_at, metadata, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 1)`,
		s.ID, s.MissionID, s.TargetID, s.Type, s.Protocol, s.Host, s.Port, s.User,
		s.PrivilegeLevel, s.Status, s.Encryption, s.EstablishedAt, s.LastActiveAt,
		s.ClosedAt, metadataJSON,
	)
	return wrapSaveError(err, "session")
}

func (r *SessionRepository) FindByID(ctx context.Context, id string) (*session.Session, error) {
	var s session.Session
	var metadataJSON []byte

	err := r.deps.QueryRow(ctx,
		`SELECT id, mission_id, target_id, type, protocol, host, port, "user",
		 privilege_level, status, encryption, established_at, last_active_at, closed_at, metadata
		 FROM sessions WHERE id = $1`, id,
	).Scan(&s.ID, &s.MissionID, &s.TargetID, &s.Type, &s.Protocol, &s.Host,
		&s.Port, &s.User, &s.PrivilegeLevel, &s.Status, &s.Encryption,
		&s.EstablishedAt, &s.LastActiveAt, &s.ClosedAt, &metadataJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: session %s not found", common.ErrSessionNotFound, id)
		}
		return nil, fmt.Errorf("find session by id: %w", err)
	}

	if len(metadataJSON) > 0 && string(metadataJSON) != "null" {
		if err := unmarshalJSON(metadataJSON, &s.Metadata); err != nil {
			return nil, fmt.Errorf("find session: unmarshal metadata: %w", err)
		}
	}

	return &s, nil
}

func (r *SessionRepository) FindByMission(ctx context.Context, missionID string) ([]*session.Session, error) {
	return r.findSessions(ctx, `SELECT id, mission_id, target_id, type, protocol, host, port, "user",
		 privilege_level, status, encryption, established_at, last_active_at, closed_at, metadata
		 FROM sessions WHERE mission_id = $1 ORDER BY established_at DESC`, missionID)
}

func (r *SessionRepository) FindByTarget(ctx context.Context, targetID string) ([]*session.Session, error) {
	return r.findSessions(ctx, `SELECT id, mission_id, target_id, type, protocol, host, port, "user",
		 privilege_level, status, encryption, established_at, last_active_at, closed_at, metadata
		 FROM sessions WHERE target_id = $1 ORDER BY established_at DESC`, targetID)
}

func (r *SessionRepository) FindActive(ctx context.Context) ([]*session.Session, error) {
	return r.findSessions(ctx, `SELECT id, mission_id, target_id, type, protocol, host, port, "user",
		 privilege_level, status, encryption, established_at, last_active_at, closed_at, metadata
		 FROM sessions WHERE status = $1 ORDER BY established_at DESC`, session.SessionActive)
}

func (r *SessionRepository) findSessions(ctx context.Context, query string, args ...interface{}) ([]*session.Session, error) {
	rows, err := r.deps.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*session.Session
	for rows.Next() {
		var s session.Session
		var metadataJSON []byte
		if err := rows.Scan(&s.ID, &s.MissionID, &s.TargetID, &s.Type, &s.Protocol,
			&s.Host, &s.Port, &s.User, &s.PrivilegeLevel, &s.Status, &s.Encryption,
			&s.EstablishedAt, &s.LastActiveAt, &s.ClosedAt, &metadataJSON); err != nil {
			return nil, fmt.Errorf("find sessions: scan: %w", err)
		}
		if len(metadataJSON) > 0 && string(metadataJSON) != "null" {
			json.Unmarshal(metadataJSON, &s.Metadata)
		}
		sessions = append(sessions, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find sessions: rows: %w", err)
	}

	return sessions, nil
}

func (r *SessionRepository) Update(ctx context.Context, s *session.Session) error {
	metadataJSON := marshalJSON(s.Metadata)

	tag, err := r.deps.Exec(ctx,
		`UPDATE sessions SET "user" = $1, privilege_level = $2, status = $3,
		 encryption = $4, last_active_at = $5, closed_at = $6, metadata = $7,
		 version = version + 1
		 WHERE id = $8`,
		s.User, s.PrivilegeLevel, s.Status, s.Encryption,
		s.LastActiveAt, s.ClosedAt, metadataJSON, s.ID,
	)
	if err != nil {
		return wrapUpdateError(err, "session")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: session %s not found for update", common.ErrSessionNotFound, s.ID)
	}
	return nil
}

func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.deps.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return wrapDeleteError(err, "session")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: session %s not found", common.ErrSessionNotFound, id)
	}
	return nil
}
