package finding

import "github.com/ophidian/ophidian/internal/domain/common"

type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() common.UTCTime
	AggregateID() string
	Version() int
}

type FindingDiscovered struct {
	FindingID common.ID
	TargetID  common.ID
	Title     string
	Severity  common.Severity
	Timestamp common.UTCTime
}

func (e FindingDiscovered) EventID() string            { return e.FindingID.String() }
func (e FindingDiscovered) EventType() string          { return "FindingDiscovered" }
func (e FindingDiscovered) OccurredAt() common.UTCTime { return e.Timestamp }
func (e FindingDiscovered) AggregateID() string        { return e.FindingID.String() }
func (e FindingDiscovered) Version() int               { return 1 }

type FindingConfirmed struct {
	FindingID common.ID
	Timestamp common.UTCTime
}

func (e FindingConfirmed) EventID() string            { return e.FindingID.String() }
func (e FindingConfirmed) EventType() string          { return "FindingConfirmed" }
func (e FindingConfirmed) OccurredAt() common.UTCTime { return e.Timestamp }
func (e FindingConfirmed) AggregateID() string        { return e.FindingID.String() }
func (e FindingConfirmed) Version() int               { return 1 }
