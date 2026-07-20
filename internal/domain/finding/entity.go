package finding

import "github.com/ophidian/ophidian/internal/domain/common"

type Finding struct {
	ID          common.ID
	MissionID   common.ID
	TargetID    common.ID
	Title       string
	Description string
	Severity    common.Severity
	CVSS        float64
	CWE         string
	CVE         string
	Confidence  ConfidenceLevel
	Status      FindingStatus
	Evidence    []Evidence
	CreatedAt   common.UTCTime
	UpdatedAt   common.UTCTime
}

type FindingStatus string

const (
	FindingNew       FindingStatus = "NEW"
	FindingStatusConfirmed FindingStatus = "CONFIRMED"
	FindingDismissed FindingStatus = "DISMISSED"
	FindingRemediated FindingStatus = "REMEDIATED"
)

type Evidence struct {
	ID        common.ID
	FindingID common.ID
	Type      EvidenceType
	Data      []byte
	Source    string
	Timestamp common.UTCTime
	Hash      string
}

type EvidenceType string

const (
	EvidenceScreenshot EvidenceType = "SCREENSHOT"
	EvidenceLog        EvidenceType = "LOG"
	EvidencePacket     EvidenceType = "PACKET"
	EvidenceFile       EvidenceType = "FILE"
	EvidenceCommand    EvidenceType = "COMMAND"
)
