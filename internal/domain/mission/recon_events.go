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
