package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/execution"
)

type SubfinderPlugin struct {
	binPath        string
	args           []string
	defaultTimeout time.Duration
}

func NewSubfinderPlugin() *SubfinderPlugin {
	return &SubfinderPlugin{
		binPath:        "subfinder",
		args:           []string{"-silent", "-timeout", "30"},
		defaultTimeout: 5 * time.Minute,
	}
}

func (p *SubfinderPlugin) Name() string {
	return "subfinder"
}

func (p *SubfinderPlugin) Run(ctx context.Context, req execution.ToolRequest) (*execution.ToolResult, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("subfinder plugin: target is empty")
	}

	binPath, err := exec.LookPath(p.binPath)
	if err != nil {
		return &execution.ToolResult{
			Evidence: "subfinder binary not found",
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

	args := append(p.args, "-d", target)
	if len(req.Options.Arguments) > 0 {
		args = append(req.Options.Arguments, "-d", target)
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
			return nil, fmt.Errorf("subfinder plugin: %w: %s", err, stderr.String())
		}
		return nil, fmt.Errorf("subfinder plugin: %w", err)
	}
	completedAt := time.Now().UTC()

	output := strings.TrimSpace(stdout.String())
	artifacts, subdomainCount := parseSubfinderOutput(output)

	return &execution.ToolResult{
		Evidence:  output,
		Artifacts: artifacts,
		Metadata: map[string]string{
			"tool":         p.Name(),
			"target":       target,
			"duration":     completedAt.Sub(startedAt).String(),
			"completed_at": completedAt.Format(time.RFC3339),
		},
		Statistics: execution.ToolStatistics{
			TargetsScanned: 1,
			Findings:       subdomainCount,
		},
	}, nil
}

func parseSubfinderOutput(output string) ([]execution.ToolArtifact, int) {
	var artifacts []execution.ToolArtifact
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		artifacts = append(artifacts, execution.ToolArtifact{
			Name: line,
			Type: "subdomain",
			Metadata: map[string]string{
				"domain": line,
			},
		})
	}
	return artifacts, len(artifacts)
}

func countNonEmptyLines(output string) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}
