package cognitive

import (
	"encoding/json"
	"fmt"
	"strings"
)

type DecisionParser struct{}

func NewDecisionParser() *DecisionParser {
	return &DecisionParser{}
}

type rawDecision struct {
	Reasoning          string          `json:"reasoning"`
	Action             string          `json:"action"`
	TargetURL          string          `json:"target_url"`
	Method             string          `json:"method"`
	PayloadType        string          `json:"payload_type"`
	Payload            string          `json:"payload"`
	Confidence         float64         `json:"confidence"`
	ExpectedIndicators json.RawMessage `json:"expected_indicators,omitempty"`
}

func (p *DecisionParser) Parse(raw string) (*AIDecision, error) {
	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM output")
	}

	var d rawDecision
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return nil, fmt.Errorf("parse decision JSON: %w", err)
	}

	d.Action = strings.ToUpper(strings.TrimSpace(d.Action))

	if err := p.validate(&d); err != nil {
		return nil, fmt.Errorf("validate decision: %w", err)
	}

	indicators := parseIndicators(d.ExpectedIndicators)

	return &AIDecision{
		Reasoning:          d.Reasoning,
		Action:             d.Action,
		TargetURL:          d.TargetURL,
		Method:             d.Method,
		PayloadType:        d.PayloadType,
		Payload:            d.Payload,
		Confidence:         d.Confidence,
		ExpectedIndicators: indicators,
	}, nil
}

func parseIndicators(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var asArray []string
	if err := json.Unmarshal(raw, &asArray); err == nil {
		return asArray
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil && strings.TrimSpace(asString) != "" {
		return []string{strings.TrimSpace(asString)}
	}

	return nil
}

func (p *DecisionParser) validate(d *rawDecision) error {
	validActions := map[string]bool{
		"CRAWL": true, "SUBMIT_FORM": true, "EXPLOIT": true,
		"ESCALATE": true, "STOP": true,
	}
	if !validActions[d.Action] {
		return fmt.Errorf("invalid action: %s (valid: CRAWL, SUBMIT_FORM, EXPLOIT, ESCALATE, STOP)", d.Action)
	}
	if d.Action != "STOP" && d.TargetURL == "" {
		return fmt.Errorf("target_url is required for action %s", d.Action)
	}
	if d.Method == "" {
		d.Method = "GET"
	} else {
		d.Method = strings.ToUpper(d.Method)
	}
	if d.Confidence == 0 {
		d.Confidence = 0.5
	}
	return nil
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	if idx := strings.Index(raw, "{"); idx >= 0 {
		raw = raw[idx:]
	}

	depth := 0
	end := -1
	for i, ch := range raw {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if end > 0 {
		return raw[:end]
	}
	return ""
}
