package target

import "github.com/ophidian/ophidian/internal/domain/common"

type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() common.UTCTime
	AggregateID() string
	Version() int
}

type TargetDiscovered struct {
	TargetID  common.ID
	IP        string
	Hostnames []string
	Timestamp common.UTCTime
}

func (e TargetDiscovered) EventID() string            { return e.TargetID.String() }
func (e TargetDiscovered) EventType() string           { return "TargetDiscovered" }
func (e TargetDiscovered) OccurredAt() common.UTCTime  { return e.Timestamp }
func (e TargetDiscovered) AggregateID() string         { return e.TargetID.String() }
func (e TargetDiscovered) Version() int                { return 1 }

type ServiceDetected struct {
	TargetID  common.ID
	Service   Service
	Timestamp common.UTCTime
}

func (e ServiceDetected) EventID() string            { return e.TargetID.String() }
func (e ServiceDetected) EventType() string          { return "ServiceDetected" }
func (e ServiceDetected) OccurredAt() common.UTCTime { return e.Timestamp }
func (e ServiceDetected) AggregateID() string        { return e.TargetID.String() }
func (e ServiceDetected) Version() int               { return 1 }
