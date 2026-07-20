package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, target string) (string, error)
}

type NmapRunner struct {
	binPath string
	args    []string
}

func NewNmapRunner() *NmapRunner {
	return &NmapRunner{
		binPath: "nmap",
		args:    []string{"-sV", "-Pn", "--top-ports", "100"},
	}
}

func (r *NmapRunner) Run(ctx context.Context, target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("nmap runner: target is empty")
	}

	args := append(r.args, target)
	cmd := exec.CommandContext(ctx, r.binPath, args...)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("nmap runner: %w: %s", err, stderr.String())
		}
		return "", fmt.Errorf("nmap runner: %w", err)
	}

	output := stdout.String()
	if output == "" && stderr.Len() > 0 {
		output = stderr.String()
	}

	return strings.TrimSpace(output), nil
}
