package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

const reconCompletedEventType = "ReconCompleted"

type StoredEvent struct {
	ID          string
	AggregateID string
	EventType   string
	Payload     json.RawMessage
	OccurredAt  time.Time
}

type EventStream interface {
	LoadEventsSince(ctx context.Context, since time.Time) ([]StoredEvent, error)
	Append(ctx context.Context, event interface{}) error
}

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type EventSubscriber struct {
	events       EventStream
	llm          LLMClient
	pollInterval time.Duration
	logger       *log.Logger
}

func NewEventSubscriber(events EventStream, llm LLMClient, pollInterval time.Duration, logger *log.Logger) *EventSubscriber {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	if logger == nil {
		logger = log.Default()
	}
	return &EventSubscriber{events: events, llm: llm, pollInterval: pollInterval, logger: logger}
}

func (s *EventSubscriber) Run(ctx context.Context) {
	if s.events == nil || s.llm == nil {
		s.logger.Printf("AI-SUBSCRIBER/WARN: disabled: event stream or llm client not configured")
		return
	}

	s.logger.Printf("AI Subscriber starting...")
	s.logger.Printf("AI-SUBSCRIBER: polling EventStore every %s", s.pollInterval)
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	lastSeen := time.Now().UTC().Add(-24 * time.Hour)
	processed := make(map[string]struct{})

	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("AI-SUBSCRIBER: stopped")
			return
		case <-ticker.C:
			lastSeen = s.pollOnce(ctx, lastSeen, processed)
		}
	}
}

func (s *EventSubscriber) pollOnce(ctx context.Context, lastSeen time.Time, processed map[string]struct{}) time.Time {
	events, err := s.events.LoadEventsSince(ctx, lastSeen)
	if err != nil {
		s.logger.Printf("AI SUBSCRIBER ERROR: failed to load events: %v", err)
		return lastSeen
	}
	s.logger.Printf("AI-SUBSCRIBER: polled %d event(s) since %s", len(events), lastSeen.Format(time.RFC3339))

	newLastSeen := lastSeen
	for _, event := range events {
		if event.OccurredAt.After(newLastSeen) {
			newLastSeen = event.OccurredAt
		}
		if event.EventType != reconCompletedEventType {
			continue
		}
		if _, ok := processed[event.ID]; ok {
			continue
		}
		processed[event.ID] = struct{}{}
		s.logger.Printf("AI-SUBSCRIBER: ReconCompleted detected: id=%s aggregate=%s", event.ID, event.AggregateID)
		if err := s.handleReconCompleted(ctx, event); err != nil {
			s.logger.Printf("AI SUBSCRIBER ERROR: recon event %s failed: %v", event.ID, err)
		}
	}

	if newLastSeen.After(lastSeen) {
		return newLastSeen.Add(time.Nanosecond)
	}
	return newLastSeen
}

func (s *EventSubscriber) handleReconCompleted(ctx context.Context, event StoredEvent) error {
	var recon mission.ReconCompletedEvent
	if err := decodeStoredPayload(event.Payload, &recon); err != nil {
		return fmt.Errorf("unmarshal recon payload: %w", err)
	}
	if strings.TrimSpace(recon.RawOutput) == "" {
		return fmt.Errorf("nmap output is empty")
	}

	prompt := fmt.Sprintf("Berikan 3 saran eksploitasi berdasarkan hasil Nmap ini:\n\n%s", recon.RawOutput)
	s.logger.Printf("AI-SUBSCRIBER: calling LLM for mission=%s target=%s output_len=%d", recon.MissionID, recon.Target, len(recon.RawOutput))
	answer, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to call LLM: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return fmt.Errorf("llm returned empty recommendation")
	}

	recommendation := mission.AIRecommendationEvent{
		MissionID:      recon.MissionID,
		SourceEventID:  event.ID,
		Recommendation: answer,
		Confidence:     0.5,
		GeneratedAt:    common.Now(),
	}
	if err := s.events.Append(ctx, recommendation); err != nil {
		return fmt.Errorf("append AI recommendation: %w", err)
	}

	s.logger.Printf("AI-SUBSCRIBER: AIRecommendationGenerated appended: mission=%s source=%s", recon.MissionID, event.ID)
	return nil
}

func decodeStoredPayload(payload json.RawMessage, dst interface{}) error {
	if err := json.Unmarshal(payload, dst); err == nil {
		return nil
	}

	var encoded string
	if err := json.Unmarshal(payload, &encoded); err != nil {
		return err
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode base64 payload: %w", err)
	}
	if err := json.Unmarshal(decoded, dst); err != nil {
		return fmt.Errorf("unmarshal decoded payload: %w", err)
	}
	return nil
}
