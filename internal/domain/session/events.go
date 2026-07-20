package session

import "github.com/ophidian/ophidian/internal/domain/common"

type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() common.UTCTime
	AggregateID() string
	Version() int
}

type SessionEstablished struct {
	SessionID common.ID
	TargetID  common.ID
	Type      SessionType
	Timestamp common.UTCTime
}

func (e SessionEstablished) EventID() string            { return e.SessionID.String() }
func (e SessionEstablished) EventType() string          { return "SessionEstablished" }
func (e SessionEstablished) OccurredAt() common.UTCTime { return e.Timestamp }
func (e SessionEstablished) AggregateID() string        { return e.SessionID.String() }
func (e SessionEstablished) Version() int               { return 1 }

type SessionLost struct {
	SessionID common.ID
	Timestamp common.UTCTime
	Reason    string
}

func (e SessionLost) EventID() string            { return e.SessionID.String() }
func (e SessionLost) EventType() string          { return "SessionLost" }
func (e SessionLost) OccurredAt() common.UTCTime { return e.Timestamp }
func (e SessionLost) AggregateID() string        { return e.SessionID.String() }
func (e SessionLost) Version() int               { return 1 }
