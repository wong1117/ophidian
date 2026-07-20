package cleanup

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type ArtifactType string

const (
	ArtifactEventLog     ArtifactType = "EVENT_LOG"
	ArtifactTempFile     ArtifactType = "TEMP_FILE"
	ArtifactCommandHist  ArtifactType = "COMMAND_HISTORY"
	ArtifactProcess      ArtifactType = "PROCESS"
	ArtifactRegistry     ArtifactType = "REGISTRY"
	ArtifactService      ArtifactType = "SERVICE"
	ArtifactScheduleTask ArtifactType = "SCHEDULED_TASK"
	ArtifactPrefetch     ArtifactType = "PREFETCH"
	ArtifactAmsi         ArtifactType = "AMSI"
	ArtifactETW          ArtifactType = "ETW"
)

type CleanupTarget struct {
	Type      ArtifactType
	Path      string
	Pattern   string
	OS        string
	RiskLevel int
}

type CleanupAction struct {
	ID          common.ID
	MissionID   string
	SessionID   string
	Targets     []CleanupTarget
	Status      CleanupStatus
	CompletedAt *common.UTCTime
	Results     []CleanupResult
}

type CleanupResult struct {
	Target  CleanupTarget
	Success bool
	Error   string
	Detail  string
}

type CleanupStatus string

const (
	CleanupPending   CleanupStatus = "PENDING"
	CleanupRunning   CleanupStatus = "RUNNING"
	CleanupCompleted CleanupStatus = "COMPLETED"
	CleanupPartial   CleanupStatus = "PARTIAL"
	CleanupFailed    CleanupStatus = "FAILED"
)

type Executor interface {
	ExecuteCleanup(ctx context.Context, action *CleanupAction) error
}

type CleanupManager struct {
	executor Executor
}

func NewCleanupManager(executor Executor) *CleanupManager {
	return &CleanupManager{executor: executor}
}

func (cm *CleanupManager) BuildCleanupPlan(os string) []CleanupTarget {
	targets := []CleanupTarget{
		{Type: ArtifactTempFile, Path: "/tmp/", OS: os, RiskLevel: 1},
		{Type: ArtifactCommandHist, Pattern: "~/.bash_history", OS: os, RiskLevel: 2},
		{Type: ArtifactProcess, Pattern: "suspicious", OS: os, RiskLevel: 3},
	}

	if os == "windows" {
		targets = append(targets,
			CleanupTarget{Type: ArtifactEventLog, Path: "Security", OS: "windows", RiskLevel: 5},
			CleanupTarget{Type: ArtifactEventLog, Path: "System", OS: "windows", RiskLevel: 4},
			CleanupTarget{Type: ArtifactEventLog, Path: "Application", OS: "windows", RiskLevel: 3},
			CleanupTarget{Type: ArtifactTempFile, Path: "%TEMP%\\", OS: "windows", RiskLevel: 1},
			CleanupTarget{Type: ArtifactTempFile, Path: "%APPDATA%\\Temp\\", OS: "windows", RiskLevel: 1},
			CleanupTarget{Type: ArtifactPrefetch, Path: "C:\\Windows\\Prefetch\\", OS: "windows", RiskLevel: 4},
			CleanupTarget{Type: ArtifactRegistry, Path: "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", OS: "windows", RiskLevel: 3},
			CleanupTarget{Type: ArtifactAmsi, Path: "amsi", OS: "windows", RiskLevel: 5},
			CleanupTarget{Type: ArtifactETW, Path: "etw", OS: "windows", RiskLevel: 5},
			CleanupTarget{Type: ArtifactService, Path: "", OS: "windows", RiskLevel: 4},
			CleanupTarget{Type: ArtifactScheduleTask, Path: "", OS: "windows", RiskLevel: 3},
		)
	} else {
		targets = append(targets,
			CleanupTarget{Type: ArtifactCommandHist, Path: "~/.bash_history", OS: "linux", RiskLevel: 2},
			CleanupTarget{Type: ArtifactCommandHist, Path: "~/.zsh_history", OS: "linux", RiskLevel: 2},
			CleanupTarget{Type: ArtifactCommandHist, Path: "/var/log/auth.log", OS: "linux", RiskLevel: 5},
			CleanupTarget{Type: ArtifactCommandHist, Path: "/var/log/syslog", OS: "linux", RiskLevel: 4},
			CleanupTarget{Type: ArtifactCommandHist, Path: "/var/log/messages", OS: "linux", RiskLevel: 4},
		)
	}

	return targets
}

func (cm *CleanupManager) BuildCleanupAction(missionID, sessionID string, targets []CleanupTarget) *CleanupAction {
	return &CleanupAction{
		ID:        common.NewID(),
		MissionID: missionID,
		SessionID: sessionID,
		Targets:   targets,
		Status:    CleanupPending,
	}
}

func (cm *CleanupManager) Execute(ctx context.Context, action *CleanupAction) error {
	action.Status = CleanupRunning
	return cm.executor.ExecuteCleanup(ctx, action)
}
