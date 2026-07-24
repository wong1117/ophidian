package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/execution"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type NucleiPlugin struct {
	binPath        string
	args           []string
	templatePath   string
	defaultTimeout time.Duration
}

func NewNucleiPlugin() *NucleiPlugin {
	return &NucleiPlugin{
		binPath:        "nuclei",
		args:           []string{"-j", "-silent"},
		templatePath:   envOr("NUCLEI_TEMPLATE_PATH", "/root/nuclei-templates"),
		defaultTimeout: 5 * time.Minute,
	}
}

func (p *NucleiPlugin) Name() string {
	return "nuclei"
}

func (p *NucleiPlugin) Run(ctx context.Context, req execution.ToolRequest) (*execution.ToolResult, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("nuclei plugin: target is empty")
	}

	binPath, err := exec.LookPath(p.binPath)
	if err != nil {
		return &execution.ToolResult{
			Evidence: "nuclei binary not found",
			Metadata: map[string]string{
				"tool":   p.Name(),
				"target": target,
				"status": "unavailable",
			},
			Statistics: execution.ToolStatistics{
				Errors: 1,
			},
		}, nil
	}

	timeout := req.Options.Timeout
	if timeout <= 0 {
		timeout = p.defaultTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	args := append(p.args, "-u", target)
	if p.templatePath != "" {
		if _, err := os.Stat(p.templatePath); err == nil {
			args = append(args, "-t", p.templatePath)
		}
	}
	if len(req.Options.Arguments) > 0 {
		args = append(req.Options.Arguments, "-u", target)
	}

	startedAt := time.Now().UTC()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = req.Options.WorkingDirectory
	if len(req.Options.Environment) > 0 {
		cmd.Env = os.Environ()
		for key, value := range req.Options.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("nuclei plugin: %w: %s", err, stderr.String())
		}
		return nil, fmt.Errorf("nuclei plugin: %w", err)
	}
	completedAt := time.Now().UTC()

	output := strings.TrimSpace(stdout.String())
	artifacts, evidence := parseNucleiOutput(output)
	findings := len(artifacts)

	return &execution.ToolResult{
		Evidence:  evidence,
		Artifacts: artifacts,
		Metadata: map[string]string{
			"tool":         p.Name(),
			"target":       target,
			"duration":     completedAt.Sub(startedAt).String(),
			"completed_at": completedAt.Format(time.RFC3339),
		},
		Statistics: execution.ToolStatistics{
			TargetsScanned: 1,
			Findings:       findings,
		},
	}, nil
}

type nucleiResult struct {
	TemplateID string `json:"template-id"`
	Name       string `json:"name"`
	Severity   string `json:"severity"`
	Matched    string `json:"matched-at"`
	Type       string `json:"type"`
	Info       struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"info"`
}

func parseNucleiOutput(output string) ([]execution.ToolArtifact, string) {
	var artifacts []execution.ToolArtifact
	var summaries []string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var result nucleiResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue
		}

		severity := result.Severity
		if severity == "" {
			severity = result.Info.Name
			if severity == "" {
				severity = "unknown"
			}
		}

		artifact := execution.ToolArtifact{
			Name: result.TemplateID,
			Type: "vulnerability",
			Metadata: map[string]string{
				"template_id": result.TemplateID,
				"name":        result.Name,
				"severity":    result.Severity,
				"matched_at":  result.Matched,
				"description": result.Info.Description,
			},
		}
		artifacts = append(artifacts, artifact)

		summary := fmt.Sprintf("[%s] %s", result.Severity, result.Name)
		if result.Matched != "" {
			summary += fmt.Sprintf(" (matched: %s)", result.Matched)
		}
		summaries = append(summaries, summary)
	}

	if len(summaries) == 0 {
		summaries = append(summaries, "nuclei: no vulnerabilities found or output not available")
	}

	return artifacts, strings.Join(summaries, "\n")
}
