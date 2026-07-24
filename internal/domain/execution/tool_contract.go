package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type ToolRequest struct {
	MissionID string
	Target    string
	Options   ToolOptions
	Metadata  map[string]string
}

type ToolOptions struct {
	Timeout          time.Duration
	Arguments        []string
	Environment      map[string]string
	WorkingDirectory string
	Parallelism      int
	Custom           map[string]any
}

type ToolResult struct {
	Evidence   string
	Artifacts  []ToolArtifact
	Metadata   map[string]string
	Statistics ToolStatistics
}

type ToolArtifact struct {
	Name     string
	Path     string
	Type     string
	Checksum string
	Metadata map[string]string
}

type ToolStatistics struct {
	TargetsScanned int
	Findings       int
	Errors         int
	Warnings       int
	Custom         map[string]int
}

type ExternalTool interface {
	Name() string
	Run(ctx context.Context, req ToolRequest) (*ToolResult, error)
}

type MultiToolReconResult struct {
	MissionID   string
	Target      string
	StartedAt   common.UTCTime
	CompletedAt common.UTCTime
	Duration    time.Duration
	Results     []ToolResult
}

type MultiToolReconCompletedEvent struct {
	Result MultiToolReconResult
}

func (e MultiToolReconCompletedEvent) EventID() string {
	return fmt.Sprintf("%s-multitool-recon-%s", e.Result.MissionID, e.Result.Target)
}

func (e MultiToolReconCompletedEvent) EventType() string {
	return "MultiToolReconCompleted"
}

func (e MultiToolReconCompletedEvent) OccurredAt() common.UTCTime {
	return e.Result.CompletedAt
}

func (e MultiToolReconCompletedEvent) AggregateID() string {
	return e.Result.MissionID
}

func (e MultiToolReconCompletedEvent) Version() int {
	return 1
}
