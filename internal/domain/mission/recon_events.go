package mission

import (
	"fmt"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type ReconCompletedEvent struct {
	MissionID   common.ID
	Target      string
	RawOutput   string
	Status      common.TaskStatus
	StartedAt   common.UTCTime
	CompletedAt common.UTCTime
}

func (e ReconCompletedEvent) EventID() string {
	return fmt.Sprintf("%s-recon-%s", e.MissionID, e.Target)
}

func (e ReconCompletedEvent) EventType() string          { return "ReconCompleted" }
func (e ReconCompletedEvent) OccurredAt() common.UTCTime { return e.CompletedAt }
func (e ReconCompletedEvent) AggregateID() string        { return e.MissionID.String() }
func (e ReconCompletedEvent) Version() int               { return 1 }

type AIRecommendationEvent struct {
	MissionID      common.ID
	SourceEventID  string
	Recommendation string
	Confidence     float64
	GeneratedAt    common.UTCTime
}

func (e AIRecommendationEvent) EventID() string {
	return fmt.Sprintf("%s-ai-recommendation-%s", e.MissionID, e.SourceEventID)
}

func (e AIRecommendationEvent) EventType() string          { return "AIRecommendationGenerated" }
func (e AIRecommendationEvent) OccurredAt() common.UTCTime { return e.GeneratedAt }
func (e AIRecommendationEvent) AggregateID() string        { return e.MissionID.String() }
func (e AIRecommendationEvent) Version() int               { return 1 }
