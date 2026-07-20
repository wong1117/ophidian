package audit

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type UpcasterChain struct {
	mu        sync.RWMutex
	upcasters map[string]map[int]Upcaster
}

func NewUpcasterChain() *UpcasterChain {
	return &UpcasterChain{upcasters: make(map[string]map[int]Upcaster)}
}

type Upcaster func(raw json.RawMessage) (json.RawMessage, error)

func (u *UpcasterChain) Register(eventType string, fromVersion int, upcaster Upcaster) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.upcasters[eventType] == nil {
		u.upcasters[eventType] = make(map[int]Upcaster)
	}
	u.upcasters[eventType][fromVersion] = upcaster
}

func (u *UpcasterChain) Upcast(eventType string, version int, payload json.RawMessage) (json.RawMessage, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	upcasters, ok := u.upcasters[eventType]
	if !ok {
		return payload, nil
	}

	versions := make([]int, 0, len(upcasters))
	for v := range upcasters {
		if v >= version {
			versions = append(versions, v)
		}
	}
	sort.Ints(versions)

	current := payload
	for _, v := range versions {
		var err error
		current, err = upcasters[v](current)
		if err != nil {
			return nil, fmt.Errorf("upcast %s v%d->v%d: %w", eventType, v, v+1, err)
		}
	}

	return current, nil
}

func (u *UpcasterChain) LatestVersion(eventType string) int {
	u.mu.RLock()
	defer u.mu.RUnlock()

	upcasters, ok := u.upcasters[eventType]
	if !ok {
		return 1
	}
	max := 0
	for v := range upcasters {
		if v > max {
			max = v
		}
	}
	return max + 1
}

type EventMetadata struct {
	ID             string          `json:"id"`
	AggregateID    string          `json:"aggregate_id"`
	AggregateType  string          `json:"aggregate_type"`
	EventType      string          `json:"event_type"`
	Version        int             `json:"version"`
	SchemaVersion  int             `json:"schema_version"`
	OccurredAt     time.Time       `json:"occurred_at"`
	CorrelationID  string          `json:"correlation_id,omitempty"`
	CausationID    string          `json:"causation_id,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

type Envelope struct {
	Meta    EventMetadata   `json:"meta"`
	Payload json.RawMessage `json:"payload"`
}

func Wrap(meta EventMetadata, payload interface{}) (*Envelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("wrap event: %w", err)
	}
	return &Envelope{Meta: meta, Payload: data}, nil
}

func (e *Envelope) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(e.Payload, v)
}
