package attackplan

import "github.com/ophidian/ophidian/internal/domain/common"

type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() common.UTCTime
	AggregateID() string
	Version() int
}

type PlanGenerated struct {
	PlanID    common.ID
	MissionID common.ID
	Graph     AttackGraph
	Timestamp common.UTCTime
}

func (e PlanGenerated) EventID() string            { return e.PlanID.String() }
func (e PlanGenerated) EventType() string          { return "PlanGenerated" }
func (e PlanGenerated) OccurredAt() common.UTCTime { return e.Timestamp }
func (e PlanGenerated) AggregateID() string        { return e.PlanID.String() }
func (e PlanGenerated) Version() int               { return 1 }

type PathSelected struct {
	PlanID    common.ID
	PathIndex int
	Timestamp common.UTCTime
}

func (e PathSelected) EventID() string            { return e.PlanID.String() }
func (e PathSelected) EventType() string          { return "PathSelected" }
func (e PathSelected) OccurredAt() common.UTCTime { return e.Timestamp }
func (e PathSelected) AggregateID() string         { return e.PlanID.String() }
func (e PathSelected) Version() int               { return 1 }
