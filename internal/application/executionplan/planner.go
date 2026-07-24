package executionplan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/execution"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type EventStoreReader interface {
	LoadAllEvents(ctx context.Context, from, to time.Time) ([]StoredEvent, error)
}

type StoredEvent struct {
	ID          string
	AggregateID string
	EventType   string
	Payload     json.RawMessage
}

type Planner struct {
	eventStore EventStoreReader
}

func NewPlanner(eventStore EventStoreReader) *Planner {
	return &Planner{eventStore: eventStore}
}

type ExecutionPlan struct {
	MissionID      string
	ApprovalID     string
	Target         string
	PayloadType    string
	PayloadCode    string
	ToolName       string
	Recommendation string
}

type exploitConfig struct {
	payloadType string
	toolName    string
	reason      string
}

func (p *Planner) CreatePlan(ctx context.Context, approval mission.OperatorApprovalEvent) (*ExecutionPlan, error) {
	if approval.Decision != common.PlanAccepted {
		return nil, fmt.Errorf("planner: approval is not ACCEPTED, got %s", approval.Decision)
	}

	reconResult, err := p.findMultiToolRecon(ctx, approval.MissionID.String())
	if err != nil {
		return nil, fmt.Errorf("planner: find recon evidence: %w", err)
	}
	if reconResult == nil {
		return nil, fmt.Errorf("planner: no MultiToolReconCompleted event found for mission %s", approval.MissionID)
	}

	aiRec, err := p.findAIRecommendation(ctx, approval.MissionID.String(), approval.SourceEventID)
	if err != nil {
		return nil, fmt.Errorf("planner: find AI recommendation: %w", err)
	}

	recommendation := ""
	if aiRec != nil {
		recommendation = aiRec.Recommendation
	}

	config := p.selectExploit(reconResult)

	plan := &ExecutionPlan{
		MissionID:      approval.MissionID.String(),
		ApprovalID:     approval.EventID(),
		Target:         reconResult.Target,
		PayloadType:    config.payloadType,
		ToolName:       config.toolName,
		Recommendation: recommendation,
	}

	return plan, nil
}

func (p *Planner) selectExploit(reconResult *execution.MultiToolReconResult) exploitConfig {
	var openPorts []execution.ToolArtifact
	var vulnerabilities []execution.ToolArtifact
	var httpProfiles []execution.ToolArtifact
	var techStack []string
	var webserver string

	for _, result := range reconResult.Results {
		for _, artifact := range result.Artifacts {
			switch artifact.Type {
			case "open_port":
				openPorts = append(openPorts, artifact)
			case "vulnerability":
				vulnerabilities = append(vulnerabilities, artifact)
			case "http_transaction", "http_endpoint":
				httpProfiles = append(httpProfiles, artifact)
				if server := artifact.Metadata["webserver"]; server != "" {
					if webserver == "" {
						webserver = strings.ToLower(server)
					}
				}
				if tech := artifact.Metadata["tech"]; tech != "" {
					techStack = append(techStack, strings.ToLower(tech))
				}
			}
		}
	}

	if len(httpProfiles) > 0 {
		techStr := strings.Join(techStack, " ")
		switch {
		case strings.Contains(techStr, "php") || strings.Contains(webserver, "apache"):
			return exploitConfig{payloadType: "WEBSHELL", toolName: "nuclei", reason: "PHP/Apache detected"}
		case strings.Contains(techStr, "iis") || strings.Contains(techStr, "asp"):
			return exploitConfig{payloadType: "WEBSHELL", toolName: "nuclei", reason: "IIS/ASP.NET detected"}
		case strings.Contains(techStr, "java") || strings.Contains(webserver, "tomcat"):
			return exploitConfig{payloadType: "WEBSHELL", toolName: "nuclei", reason: "Java/Tomcat detected"}
		case strings.Contains(techStr, "python"):
			return exploitConfig{payloadType: "SSTI", toolName: "nuclei", reason: "Python web server — possible Jinja2/Flask SSTI"}
		default:
			return exploitConfig{payloadType: "REVERSE_SHELL", toolName: "nuclei", reason: "HTTP service detected"}
		}
	}

	if len(vulnerabilities) > 0 {
		highestSeverity := "LOW"
		for _, v := range vulnerabilities {
			sev := strings.ToUpper(v.Metadata["severity"])
			if severityRank(sev) > severityRank(highestSeverity) {
				highestSeverity = sev
			}
		}
		switch highestSeverity {
		case "CRITICAL":
			return exploitConfig{payloadType: "COMMAND_INJECTION", toolName: "nuclei", reason: "Critical vulnerability found — direct exploit"}
		case "HIGH":
			return exploitConfig{payloadType: "REVERSE_SHELL", toolName: "nuclei", reason: "High severity vulnerability"}
		default:
			return exploitConfig{payloadType: "REVERSE_SHELL", toolName: "nmap", reason: "Vulnerability detected, recon first"}
		}
	}

	if len(openPorts) > 0 {
		for _, port := range openPorts {
			service := strings.ToLower(port.Metadata["service"])
			if service == "http" || service == "https" {
				return exploitConfig{payloadType: "WEBSHELL", toolName: "nuclei", reason: "HTTP port open"}
			}
			if service == "ssh" || service == "ftp" || service == "rdp" {
				return exploitConfig{payloadType: "REVERSE_SHELL", toolName: "nmap", reason: fmt.Sprintf("%s service on port %s", service, port.Metadata["port"])}
			}
		}
	}

	return exploitConfig{payloadType: "REVERSE_SHELL", toolName: "nmap", reason: "no specific vulnerability — recon first"}
}

func severityRank(s string) int {
	switch s {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}

func (p *Planner) findMultiToolRecon(ctx context.Context, missionID string) (*execution.MultiToolReconResult, error) {
	from := time.Now().UTC().Add(-48 * time.Hour)
	to := time.Now().UTC()
	events, err := p.eventStore.LoadAllEvents(ctx, from, to)
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		if event.EventType != "MultiToolReconCompleted" {
			continue
		}
		var multiEvent execution.MultiToolReconCompletedEvent
		if err := json.Unmarshal(event.Payload, &multiEvent); err != nil {
			continue
		}
		if multiEvent.Result.MissionID == missionID {
			return &multiEvent.Result, nil
		}
	}
	return nil, nil
}

func (p *Planner) findAIRecommendation(ctx context.Context, missionID, sourceEventID string) (*mission.AIRecommendationEvent, error) {
	from := time.Now().UTC().Add(-48 * time.Hour)
	to := time.Now().UTC()
	events, err := p.eventStore.LoadAllEvents(ctx, from, to)
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		if event.EventType != "AIRecommendationGenerated" {
			continue
		}
		var rec mission.AIRecommendationEvent
		if err := json.Unmarshal(event.Payload, &rec); err != nil {
			continue
		}
		if rec.MissionID.String() == missionID && rec.SourceEventID == sourceEventID {
			return &rec, nil
		}
	}
	return nil, nil
}
