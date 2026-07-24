package mission

import (
	"encoding/json"
	"fmt"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type AdaptiveActionEvent struct {
	Payload json.RawMessage
}

func (e AdaptiveActionEvent) EventID() string {
	return fmt.Sprintf("adaptive-%d", common.Now().Time.UnixNano())
}

func (e AdaptiveActionEvent) EventType() string { return "AdaptiveAction" }

func (e AdaptiveActionEvent) OccurredAt() common.UTCTime { return common.Now() }

func (e AdaptiveActionEvent) AggregateID() string { return "adaptive" }

func (e AdaptiveActionEvent) Version() int { return 1 }
