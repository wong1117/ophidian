package redis

import (
	"context"
	"encoding/json"
	"time"
	"github.com/redis/go-redis/v9"
	"github.com/ophidian/ophidian/internal/domain/session"
)

type SessionStore struct {
	client *redis.Client
}

func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

func (s *SessionStore) SaveSession(ctx context.Context, sess *session.Session, ttl time.Duration) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, "session:"+sess.ID.String(), data, ttl).Err()
}

func (s *SessionStore) GetSession(ctx context.Context, id string) (*session.Session, error) {
	data, err := s.client.Get(ctx, "session:"+id).Bytes()
	if err != nil {
		return nil, err
	}
	var sess session.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}
