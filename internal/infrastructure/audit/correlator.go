package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type CorrelatedEvent struct {
	Event     Envelope
	Phase     string
	Index     int
	MissionID string
	TargetID  string
}

type KillChainPhase struct {
	Name   string
	Events []CorrelatedEvent
	Start  time.Time
	End    time.Time
}

type CorrelationResult struct {
	MissionID   string
	TargetID    string
	Phases      map[string]*KillChainPhase
	TotalEvents int
	Duration    time.Duration
}

type EventCorrelator struct {
	eventStore EventStore
}

func NewEventCorrelator(eventStore EventStore) *EventCorrelator {
	return &EventCorrelator{eventStore: eventStore}
}

var phaseMapping = map[string]string{
	"recon":          "RECONNAISSANCE",
	"scan":           "RECONNAISSANCE",
	"port_scan":      "RECONNAISSANCE",
	"service_scan":   "RECONNAISSANCE",
	"TargetDiscovered": "RECONNAISSANCE",
	"ServiceDetected":  "RECONNAISSANCE",

	"plan":           "WEAPONIZATION",
	"PlanGenerated":  "WEAPONIZATION",

	"exploit":           "EXPLOITATION",
	"exploit_execute":    "EXPLOITATION",
	"ExploitExecuted":    "EXPLOITATION",
	"SessionEstablished": "EXPLOITATION",
	"SessionCreated": "EXPLOITATION",

	"privilege_escalation": "INSTALLATION",
	"lateral_movement":     "INSTALLATION",

	"data_exfiltration":     "ACTIONS_ON_OBJECTIVE",
	"MissionCompleted":      "ACTIONS_ON_OBJECTIVE",
	"report":                "ACTIONS_ON_OBJECTIVE",
}

var phaseOrder = []string{
	"RECONNAISSANCE",
	"WEAPONIZATION",
	"DELIVERY",
	"EXPLOITATION",
	"INSTALLATION",
	"COMMAND_AND_CONTROL",
	"ACTIONS_ON_OBJECTIVE",
}

func (c *EventCorrelator) Correlate(ctx context.Context, missionID string, from, to time.Time) (*CorrelationResult, error) {
	events, err := c.eventStore.LoadAllEvents(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("correlate: %w", err)
	}

	result := &CorrelationResult{
		MissionID: missionID,
		Phases:    make(map[string]*KillChainPhase),
	}

	if len(events) == 0 {
		return result, nil
	}

	correlated := make([]CorrelatedEvent, 0, len(events))
	for _, evt := range events {
		phase := classifyEvent(evt)
		ce := CorrelatedEvent{
			Event:  evt,
			Phase:  phase,
			Index:  len(correlated),
			MissionID: missionID,
		}
		correlated = append(correlated, ce)

		if result.Phases[phase] == nil {
			result.Phases[phase] = &KillChainPhase{Name: phase}
		}
		result.Phases[phase].Events = append(result.Phases[phase].Events, ce)
	}

	sort.Slice(correlated, func(i, j int) bool {
		return correlated[i].Event.Meta.OccurredAt.Before(correlated[j].Event.Meta.OccurredAt)
	})

	for _, ce := range correlated {
		index := 0
		for i, p := range phaseOrder {
			if p == ce.Phase {
				index = i
				break
			}
		}
		_ = index
	}

	result.TotalEvents = len(events)
	if len(events) > 0 {
		result.Duration = events[len(events)-1].Meta.OccurredAt.Sub(events[0].Meta.OccurredAt)
	}

	return result, nil
}

func classifyEvent(evt Envelope) string {
	et := strings.ToLower(evt.Meta.EventType)
	if phase, ok := phaseMapping[et]; ok {
		return phase
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(evt.Payload, &payload); err == nil {
		if taskType, ok := payload["type"].(string); ok {
			if phase, ok := phaseMapping[strings.ToLower(taskType)]; ok {
				return phase
			}
		}
		if taskType, ok := payload["task_type"].(string); ok {
			if phase, ok := phaseMapping[strings.ToLower(taskType)]; ok {
				return phase
			}
		}
	}

	if strings.Contains(et, "recon") {
		return "RECONNAISSANCE"
	}
	if strings.Contains(et, "exploit") {
		return "EXPLOITATION"
	}
	if strings.Contains(et, "plan") {
		return "WEAPONIZATION"
	}

	return "COMMAND_AND_CONTROL"
}

type KillChainVisualData struct {
	MissionID  string              `json:"mission_id"`
	Phases     []KillChainPhaseData `json:"phases"`
	TotalEvents int                `json:"total_events"`
	Duration   string              `json:"duration"`
}

type KillChainPhaseData struct {
	Name        string   `json:"name"`
	EventCount  int      `json:"event_count"`
	StartTime   string   `json:"start_time"`
	EndTime     string   `json:"end_time"`
	EventIDs    []string `json:"event_ids"`
}

func (c *EventCorrelator) BuildKillChainVisual(ctx context.Context, missionID string, from, to time.Time) (*KillChainVisualData, error) {
	result, err := c.Correlate(ctx, missionID, from, to)
	if err != nil {
		return nil, err
	}

	data := &KillChainVisualData{
		MissionID:   missionID,
		TotalEvents: result.TotalEvents,
		Duration:    result.Duration.String(),
	}

	for _, phaseName := range phaseOrder {
		if phase, ok := result.Phases[phaseName]; ok {
			eventIDs := make([]string, len(phase.Events))
			start := phase.Events[0].Event.Meta.OccurredAt
			end := phase.Events[len(phase.Events)-1].Event.Meta.OccurredAt
			for i, ce := range phase.Events {
				eventIDs[i] = ce.Event.Meta.ID
			}

			data.Phases = append(data.Phases, KillChainPhaseData{
				Name:       phaseName,
				EventCount: len(phase.Events),
				StartTime:  start.Format(time.RFC3339),
				EndTime:    end.Format(time.RFC3339),
				EventIDs:   eventIDs,
			})
		}
	}

	return data, nil
}
