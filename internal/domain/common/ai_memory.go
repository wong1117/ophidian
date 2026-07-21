package common

import "time"

type MemoryEntryType string

const (
	MemoryMissionSuccess MemoryEntryType = "MISSION_SUCCESS"
	MemoryMissionFailure MemoryEntryType = "MISSION_FAILURE"
	MemoryTechniqueOk    MemoryEntryType = "TECHNIQUE_OK"
	MemoryTechniqueFail  MemoryEntryType = "TECHNIQUE_FAIL"
	MemoryEnvironment    MemoryEntryType = "ENVIRONMENT"
	MemoryTargetProfile  MemoryEntryType = "TARGET_PROFILE"
)

type MemoryEntry struct {
	ID         ID
	Type       MemoryEntryType
	Content    string
	Tags       []string
	MissionID  string
	TargetOS   string
	TargetEnv  string
	Technique  string
	CVE        string
	Severity   string
	Confidence float64
	Success    bool
	Context    map[string]interface{}
	Embedding  []float32
	CreatedAt  time.Time
	ExpiresAt  *time.Time
}
