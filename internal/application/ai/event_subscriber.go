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
	"github.com/ophidian/ophidian/internal/domain/execution"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

const (
	multiToolReconCompletedEventType = "MultiToolReconCompleted"
)

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
		if !isSupportedReconEvent(event.EventType) {
			continue
		}
		if _, ok := processed[event.ID]; ok {
			continue
		}
		processed[event.ID] = struct{}{}

		s.logger.Printf("AI-SUBSCRIBER: %s detected: id=%s aggregate=%s", event.EventType, event.ID, event.AggregateID)
		if err := s.handleMultiToolReconCompleted(ctx, event); err != nil {
			s.logger.Printf("AI SUBSCRIBER ERROR: %s event %s failed: %v", event.EventType, event.ID, err)
		}
	}

	if newLastSeen.After(lastSeen) {
		return newLastSeen.Add(time.Nanosecond)
	}
	return newLastSeen
}

func isSupportedReconEvent(eventType string) bool {
	return eventType == multiToolReconCompletedEventType
}

func (s *EventSubscriber) handleMultiToolReconCompleted(ctx context.Context, event StoredEvent) error {
	var recon execution.MultiToolReconCompletedEvent
	if err := decodeStoredPayload(event.Payload, &recon); err != nil {
		return fmt.Errorf("unmarshal multi-tool recon payload: %w", err)
	}
	if strings.TrimSpace(recon.Result.MissionID) == "" {
		return fmt.Errorf("mission id is empty")
	}
	if strings.TrimSpace(recon.Result.Target) == "" {
		return fmt.Errorf("target is empty")
	}
	if len(recon.Result.Results) == 0 {
		return fmt.Errorf("multi-tool recon results are empty")
	}

	prompt, evidenceLen := buildMultiToolReconPrompt(recon.Result)
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("multi-tool evidence is empty")
	}

	s.logger.Printf("AI-SUBSCRIBER: calling LLM for mission=%s target=%s tool_results=%d evidence_len=%d", recon.Result.MissionID, recon.Result.Target, len(recon.Result.Results), evidenceLen)
	answer, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to call LLM: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return fmt.Errorf("llm returned empty recommendation")
	}

	recommendation := mission.AIRecommendationEvent{
		MissionID:      common.ID(recon.Result.MissionID),
		SourceEventID:  event.ID,
		Recommendation: answer,
		Confidence:     0,
		GeneratedAt:    common.Now(),
	}
	if err := s.events.Append(ctx, recommendation); err != nil {
		return fmt.Errorf("append AI recommendation: %w", err)
	}

	s.logger.Printf("AI-SUBSCRIBER: AIRecommendationGenerated appended: mission=%s source=%s", recon.Result.MissionID, event.ID)
	return nil
}

func buildMultiToolReconPrompt(result execution.MultiToolReconResult) (string, int) {
	var b strings.Builder
	fmt.Fprintf(&b, "Saya memiliki hasil scanning multi-tool untuk target %s:\n\n", result.Target)

	totalEvidenceLen := 0
	for i, toolResult := range result.Results {
		evidence := strings.TrimSpace(toolResult.Evidence)
		if evidence == "" && len(toolResult.Artifacts) == 0 {
			continue
		}

		totalEvidenceLen += len(evidence)
		fmt.Fprintf(&b, "- Tool %s: %s\n", toolName(toolResult, i), evidence)

		for _, artifact := range toolResult.Artifacts {
			fmt.Fprintf(&b, "    [artifact] %s (%s): ", artifact.Name, artifact.Type)
			var pairs []string
			for key, value := range artifact.Metadata {
				if value != "" && len(value) < 512 {
					pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
				}
			}
			fmt.Fprintf(&b, "%s\n", strings.Join(pairs, ", "))
		}
	}

	if totalEvidenceLen == 0 {
		return "", 0
	}
	fmt.Fprintf(&b, "\nBerikan 3 rekomendasi eksploitasi berdasarkan data gabungan di atas. Analisis kerentanan berdasarkan versi software, port yang terbuka, teknologi yang terdeteksi, header keamanan yang hilang, dan subdomain yang ditemukan.")
	return b.String(), totalEvidenceLen
}

func toolName(result execution.ToolResult, index int) string {
	for _, key := range []string{"tool", "tool_name", "name"} {
		if value := strings.TrimSpace(result.Metadata[key]); value != "" {
			return value
		}
	}
	return fmt.Sprintf("#%d", index+1)
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
