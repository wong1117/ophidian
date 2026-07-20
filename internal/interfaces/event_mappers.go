package interfaces

import (
	"encoding/json"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type EventMessage struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

func MapMissionEventToNATS(event mission.DomainEvent) ([]byte, error) {
	msg := EventMessage{
		Type:      event.EventType(),
		Timestamp: event.OccurredAt().Unix(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	msg.Payload = payload
	return json.Marshal(msg)
}
