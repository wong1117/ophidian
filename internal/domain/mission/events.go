package mission

import "github.com/ophidian/ophidian/internal/domain/common"

type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() common.UTCTime
	AggregateID() string
	Version() int
}

type MissionStarted struct {
	MissionID  common.ID
	Target     Target
	Objectives []Objective
	RoE        RoEConstraints
	StartedAt  common.UTCTime
	StartedBy  string
}

func (e MissionStarted) EventID() string     { return e.MissionID.String() }
func (e MissionStarted) EventType() string   { return "MissionStarted" }
func (e MissionStarted) OccurredAt() common.UTCTime { return e.StartedAt }
func (e MissionStarted) AggregateID() string { return e.MissionID.String() }
func (e MissionStarted) Version() int        { return 1 }

type PhaseTransitioned struct {
	MissionID   common.ID
	FromPhase   common.Phase
	ToPhase     common.Phase
	Reason      string
	TriggeredBy string
	Timestamp   common.UTCTime
}

func (e PhaseTransitioned) EventID() string          { return e.MissionID.String() }
func (e PhaseTransitioned) EventType() string        { return "PhaseTransitioned" }
func (e PhaseTransitioned) OccurredAt() common.UTCTime { return e.Timestamp }
func (e PhaseTransitioned) AggregateID() string     { return e.MissionID.String() }
func (e PhaseTransitioned) Version() int             { return 1 }

type TaskDispatched struct {
	MissionID    common.ID
	TaskID       common.ID
	TaskType     string
	Parameters   map[string]interface{}
	ToPlane      common.PlaneType
	DispatchedAt common.UTCTime
}

func (e TaskDispatched) EventID() string            { return e.TaskID.String() }
func (e TaskDispatched) EventType() string          { return "TaskDispatched" }
func (e TaskDispatched) OccurredAt() common.UTCTime  { return e.DispatchedAt }
func (e TaskDispatched) AggregateID() string         { return e.MissionID.String() }
func (e TaskDispatched) Version() int               { return 1 }

type TaskCompleted struct {
	MissionID   common.ID
	TaskID      common.ID
	Status      common.TaskStatus
	Result      *TaskResult
	Duration    int
	CompletedAt common.UTCTime
}

func (e TaskCompleted) EventID() string            { return e.TaskID.String() }
func (e TaskCompleted) EventType() string          { return "TaskCompleted" }
func (e TaskCompleted) OccurredAt() common.UTCTime { return e.CompletedAt }
func (e TaskCompleted) AggregateID() string        { return e.MissionID.String() }
func (e TaskCompleted) Version() int               { return 1 }

type AIRecommendationReceived struct {
	MissionID      common.ID
	PlanID         common.ID
	Recommendation StrategyRecommendation
	Confidence     float64
	ReceivedAt     common.UTCTime
}

func (e AIRecommendationReceived) EventID() string            { return e.PlanID.String() }
func (e AIRecommendationReceived) EventType() string          { return "AIRecommendationReceived" }
func (e AIRecommendationReceived) OccurredAt() common.UTCTime { return e.ReceivedAt }
func (e AIRecommendationReceived) AggregateID() string        { return e.MissionID.String() }
func (e AIRecommendationReceived) Version() int               { return 1 }

type MissionStateChanged struct {
	MissionID  common.ID
	FromStatus MissionStatus
	ToStatus   MissionStatus
	Reason     string
	UpdatedBy  string
	Timestamp  common.UTCTime
}

func (e MissionStateChanged) EventID() string            { return e.MissionID.String() }
func (e MissionStateChanged) EventType() string          { return "MissionStateChanged" }
func (e MissionStateChanged) OccurredAt() common.UTCTime { return e.Timestamp }
func (e MissionStateChanged) AggregateID() string        { return e.MissionID.String() }
func (e MissionStateChanged) Version() int               { return 1 }

type AIPlanDecision struct {
	MissionID common.ID
	PlanID    common.ID
	Decision  common.PlanDecision
	Reason    string
	DecidedAt common.UTCTime
}

func (e AIPlanDecision) EventID() string            { return e.PlanID.String() }
func (e AIPlanDecision) EventType() string          { return "AIPlanDecision" }
func (e AIPlanDecision) OccurredAt() common.UTCTime { return e.DecidedAt }
func (e AIPlanDecision) AggregateID() string        { return e.MissionID.String() }
func (e AIPlanDecision) Version() int               { return 1 }
