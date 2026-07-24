package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/execution"
)

type Runner interface {
	execution.ExternalTool
}

type NmapRunner struct {
	binPath        string
	args           []string
	defaultTimeout time.Duration
}

func NewNmapRunner() *NmapRunner {
	return &NmapRunner{
		binPath:        "nmap",
		args:           []string{"-sV", "-Pn", "--top-ports", "100", "--host-timeout", "15s"},
		defaultTimeout: 60 * time.Second,
	}
}

func (r *NmapRunner) Name() string {
	return "nmap"
}

func (r *NmapRunner) Run(ctx context.Context, req execution.ToolRequest) (*execution.ToolResult, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("nmap runner: target is empty")
	}

	timeout := req.Options.Timeout
	if timeout <= 0 {
		timeout = r.defaultTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	args := append(r.args, target)
	if len(req.Options.Arguments) > 0 {
		args = append(req.Options.Arguments, target)
	}

	startedAt := time.Now().UTC()
	cmd := exec.CommandContext(ctx, r.binPath, args...)
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
			return nil, fmt.Errorf("nmap runner: %w: %s", err, stderr.String())
		}
		return nil, fmt.Errorf("nmap runner: %w", err)
	}
	completedAt := time.Now().UTC()

	output := stdout.String()
	if output == "" && stderr.Len() > 0 {
		output = stderr.String()
	}

	evidence := strings.TrimSpace(output)
	artifacts := parseNmapOutput(evidence)

	return &execution.ToolResult{
		Evidence:  evidence,
		Artifacts: artifacts,
		Metadata: map[string]string{
			"tool":         r.Name(),
			"target":       target,
			"duration":     completedAt.Sub(startedAt).String(),
			"completed_at": completedAt.Format(time.RFC3339),
		},
		Statistics: execution.ToolStatistics{
			TargetsScanned: 1,
			Findings:       len(artifacts),
		},
	}, nil
}

var nmapPortRe = regexp.MustCompile(`^(\d+)/(\w+)\s+open\s+(\S+)\s*(.*)`)

func parseNmapOutput(output string) []execution.ToolArtifact {
	var artifacts []execution.ToolArtifact
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		matches := nmapPortRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		port := matches[1]
		protocol := matches[2]
		service := strings.TrimSpace(matches[3])
		version := strings.TrimSpace(matches[4])
		if version == "" {
			version = "(unknown)"
		}

		artifacts = append(artifacts, execution.ToolArtifact{
			Name: fmt.Sprintf("%s/%s", port, protocol),
			Type: "open_port",
			Metadata: map[string]string{
				"port":     port,
				"protocol": protocol,
				"service":  service,
				"version":  version,
			},
		})
	}
	return artifacts
}
