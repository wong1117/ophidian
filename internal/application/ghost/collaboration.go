package ghost

import (
	"context"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type Operator struct {
	ID       string
	Name     string
	Role     string
	JoinedAt time.Time
}

type SessionEvent struct {
	ID        common.ID
	SessionID string
	Type      EventType
	Operator  string
	Content   string
	Timestamp time.Time
}

type EventType string

const (
	EventMessage    EventType = "MESSAGE"
	EventCommand    EventType = "COMMAND"
	EventOutput     EventType = "OUTPUT"
	EventAnnotation EventType = "ANNOTATION"
	EventJoin       EventType = "JOIN"
	EventLeave      EventType = "LEAVE"
	EventShare      EventType = "SHARE"
)

type GhostSession struct {
	ID        string
	MissionID string
	Operators map[string]*Operator
	Events    []SessionEvent
	Outputs   chan SessionEvent
	mu        sync.RWMutex
	createdAt time.Time
}

type GhostCollaboration struct {
	sessions map[string]*GhostSession
	mu       sync.RWMutex
}

func NewGhostCollaboration() *GhostCollaboration {
	return &GhostCollaboration{
		sessions: make(map[string]*GhostSession),
	}
}

func (gc *GhostCollaboration) CreateSession(ctx context.Context, missionID string) *GhostSession {
	session := &GhostSession{
		ID:        string(common.NewID()),
		MissionID: missionID,
		Operators: make(map[string]*Operator),
		Events:    make([]SessionEvent, 0),
		Outputs:   make(chan SessionEvent, 1000),
		createdAt: time.Now(),
	}
	gc.mu.Lock()
	gc.sessions[session.ID] = session
	gc.mu.Unlock()
	return session
}

func (gc *GhostCollaboration) JoinSession(ctx context.Context, sessionID, operatorID, name, role string) (*GhostSession, error) {
	gc.mu.RLock()
	session, ok := gc.sessions[sessionID]
	gc.mu.RUnlock()
	if !ok {
		return nil, common.ErrSessionNotFound
	}

	session.mu.Lock()
	session.Operators[operatorID] = &Operator{
		ID:       operatorID,
		Name:     name,
		Role:     role,
		JoinedAt: time.Now(),
	}
	session.mu.Unlock()

	session.Events = append(session.Events, SessionEvent{
		ID:        common.NewID(),
		SessionID: sessionID,
		Type:      EventJoin,
		Operator:  name,
		Content:   name + " joined the session",
		Timestamp: time.Now(),
	})

	return session, nil
}

func (gc *GhostCollaboration) LeaveSession(ctx context.Context, sessionID, operatorID string) error {
	gc.mu.RLock()
	session, ok := gc.sessions[sessionID]
	gc.mu.RUnlock()
	if !ok {
		return common.ErrSessionNotFound
	}

	session.mu.Lock()
	op, exists := session.Operators[operatorID]
	session.mu.Unlock()
	if !exists {
		return nil
	}

	session.Events = append(session.Events, SessionEvent{
		ID:        common.NewID(),
		SessionID: sessionID,
		Type:      EventLeave,
		Operator:  op.Name,
		Content:   op.Name + " left the session",
		Timestamp: time.Now(),
	})

	session.mu.Lock()
	delete(session.Operators, operatorID)
	session.mu.Unlock()
	return nil
}

func (gc *GhostCollaboration) SendMessage(ctx context.Context, sessionID, operatorID, content string) error {
	gc.mu.RLock()
	session, ok := gc.sessions[sessionID]
	gc.mu.RUnlock()
	if !ok {
		return common.ErrSessionNotFound
	}

	session.mu.RLock()
	op, exists := session.Operators[operatorID]
	session.mu.RUnlock()
	if !exists {
		return common.ErrUnauthorized
	}

	event := SessionEvent{
		ID:        common.NewID(),
		SessionID: sessionID,
		Type:      EventMessage,
		Operator:  op.Name,
		Content:   content,
		Timestamp: time.Now(),
	}

	session.Events = append(session.Events, event)
	session.Outputs <- event
	return nil
}

func (gc *GhostCollaboration) SendOutput(ctx context.Context, sessionID, content string) {
	gc.mu.RLock()
	session, ok := gc.sessions[sessionID]
	gc.mu.RUnlock()
	if !ok {
		return
	}

	event := SessionEvent{
		ID:        common.NewID(),
		SessionID: sessionID,
		Type:      EventOutput,
		Content:   content,
		Timestamp: time.Now(),
	}

	session.Events = append(session.Events, event)
	session.Outputs <- event
}

func (gc *GhostCollaboration) Subscribe(ctx context.Context, sessionID string) (<-chan SessionEvent, error) {
	gc.mu.RLock()
	session, ok := gc.sessions[sessionID]
	gc.mu.RUnlock()
	if !ok {
		return nil, common.ErrSessionNotFound
	}
	return session.Outputs, nil
}

func (gc *GhostCollaboration) GetSession(ctx context.Context, sessionID string) (*GhostSession, error) {
	gc.mu.RLock()
	session, ok := gc.sessions[sessionID]
	gc.mu.RUnlock()
	if !ok {
		return nil, common.ErrSessionNotFound
	}
	return session, nil
}

func (gc *GhostCollaboration) Annotate(ctx context.Context, sessionID, operatorID, annotation string) error {
	gc.mu.RLock()
	session, ok := gc.sessions[sessionID]
	gc.mu.RUnlock()
	if !ok {
		return common.ErrSessionNotFound
	}

	session.mu.RLock()
	op, exists := session.Operators[operatorID]
	session.mu.RUnlock()
	if !exists {
		return common.ErrUnauthorized
	}

	event := SessionEvent{
		ID:        common.NewID(),
		SessionID: sessionID,
		Type:      EventAnnotation,
		Operator:  op.Name,
		Content:   annotation,
		Timestamp: time.Now(),
	}

	session.Events = append(session.Events, event)
	session.Outputs <- event
	return nil
}
