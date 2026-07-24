package mission

import (
	"fmt"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type OperatorApprovalEvent struct {
	MissionID     common.ID
	SourceEventID string
	Decision      common.PlanDecision
	OperatorNote  string
	ApprovedAt    common.UTCTime
}

func (e OperatorApprovalEvent) EventID() string {
	return fmt.Sprintf("%s-approval-%s", e.MissionID, e.SourceEventID)
}

func (e OperatorApprovalEvent) EventType() string { return "OperatorApproval" }

func (e OperatorApprovalEvent) OccurredAt() common.UTCTime { return e.ApprovedAt }

func (e OperatorApprovalEvent) AggregateID() string { return e.MissionID.String() }

func (e OperatorApprovalEvent) Version() int { return 1 }
