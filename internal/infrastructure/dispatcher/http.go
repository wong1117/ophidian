package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type HTTPEventDispatcher struct {
	workerURL string
	client    *http.Client
}

func NewHTTPEventDispatcher(workerURL string) *HTTPEventDispatcher {
	return &HTTPEventDispatcher{
		workerURL: workerURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type EventEnvelope struct {
	EventType   string          `json:"event_type"`
	AggregateID string          `json:"aggregate_id"`
	Payload     json.RawMessage `json:"payload"`
}

func (d *HTTPEventDispatcher) Dispatch(ctx context.Context, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	envelope := EventEnvelope{
		Payload: payload,
	}

	if de, ok := event.(interface {
		EventType() string
		AggregateID() string
	}); ok {
		envelope.EventType = de.EventType()
		envelope.AggregateID = de.AggregateID()
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.workerURL+"/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch to worker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("worker rejected event (status %d): %s", resp.StatusCode, string(errBody))
	}

	log.Printf("event dispatched to worker: %s %s", envelope.EventType, envelope.AggregateID)
	return nil
}
