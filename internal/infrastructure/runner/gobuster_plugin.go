package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/execution"
)

type GobusterPlugin struct {
	binPath        string
	args           []string
	wordlistPath   string
	defaultTimeout time.Duration
}

func NewGobusterPlugin() *GobusterPlugin {
	return &GobusterPlugin{
		binPath:        "gobuster",
		args:           []string{"dir", "-q", "--no-color", "--no-progress"},
		wordlistPath:   envOr("GOBUSTER_WORDLIST", "/usr/share/wordlists/dirb/common.txt"),
		defaultTimeout: 5 * time.Minute,
	}
}

func (p *GobusterPlugin) Name() string {
	return "gobuster"
}

func (p *GobusterPlugin) Run(ctx context.Context, req execution.ToolRequest) (*execution.ToolResult, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("gobuster plugin: target is empty")
	}

	binPath, err := exec.LookPath(p.binPath)
	if err != nil {
		return &execution.ToolResult{
			Evidence: "gobuster binary not found",
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

	wordlist := p.resolveWordlist()

	timeout := req.Options.Timeout
	if timeout <= 0 {
		timeout = p.defaultTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	args := append(p.args, "-u", target, "-w", wordlist)
	if len(req.Options.Arguments) > 0 {
		args = req.Options.Arguments
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
			return nil, fmt.Errorf("gobuster plugin: %w: %s", err, stderr.String())
		}
		return nil, fmt.Errorf("gobuster plugin: %w", err)
	}
	completedAt := time.Now().UTC()

	output := strings.TrimSpace(stdout.String())
	dirCount := countNonEmptyLines(output)

	return &execution.ToolResult{
		Evidence: output,
		Metadata: map[string]string{
			"tool":         p.Name(),
			"target":       target,
			"wordlist":     wordlist,
			"duration":     completedAt.Sub(startedAt).String(),
			"completed_at": completedAt.Format(time.RFC3339),
		},
		Statistics: execution.ToolStatistics{
			TargetsScanned: 1,
			Findings:       dirCount,
		},
	}, nil
}

func (p *GobusterPlugin) resolveWordlist() string {
	if _, err := os.Stat(p.wordlistPath); err == nil {
		return p.wordlistPath
	}
	alt := "/usr/share/wordlists/dirb/common.txt"
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return "/usr/share/dirb/wordlists/common.txt"
}
