package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/execution"
)

type HttpxPlugin struct {
	binPath        string
	args           []string
	defaultTimeout time.Duration
}

func NewHttpxPlugin() *HttpxPlugin {
	return &HttpxPlugin{
		binPath:        "httpx",
		args:           []string{"-silent", "-json", "-title", "-status-code", "-tech-detect", "-no-color", "-content-length", "-content-type", "-webserver", "-response-time"},
		defaultTimeout: 3 * time.Minute,
	}
}

func (p *HttpxPlugin) Name() string {
	return "httpx"
}

type httpxResult struct {
	URL           string   `json:"url"`
	Status        int      `json:"status_code"`
	Title         string   `json:"title"`
	Webserver     string   `json:"webserver"`
	Tech          []string `json:"tech"`
	ContentLength int      `json:"content_length"`
	ContentType   string   `json:"content_type"`
	ResponseTime  string   `json:"response_time"`
}

func (p *HttpxPlugin) Run(ctx context.Context, req execution.ToolRequest) (*execution.ToolResult, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("httpx plugin: target is empty")
	}

	binPath, err := exec.LookPath(p.binPath)
	if err != nil {
		return &execution.ToolResult{
			Evidence: "httpx binary not found",
			Metadata: map[string]string{
				"tool":   p.Name(),
				"target": target,
				"status": "unavailable",
			},
			Statistics: execution.ToolStatistics{Errors: 1},
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
			return nil, fmt.Errorf("httpx plugin: %w: %s", err, stderr.String())
		}
		return nil, fmt.Errorf("httpx plugin: %w", err)
	}
	completedAt := time.Now().UTC()

	output := strings.TrimSpace(stdout.String())
	artifacts, summary := p.parseHttpxOutput(output, target)

	return &execution.ToolResult{
		Evidence:  summary,
		Artifacts: artifacts,
		Metadata: map[string]string{
			"tool":         p.Name(),
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

func (p *HttpxPlugin) parseHttpxOutput(output, target string) ([]execution.ToolArtifact, string) {
	var artifacts []execution.ToolArtifact
	var summaries []string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var result httpxResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue
		}

		artifact := execution.ToolArtifact{
			Name: result.URL,
			Type: "http_endpoint",
			Metadata: map[string]string{
				"url":            result.URL,
				"status_code":    strconv.Itoa(result.Status),
				"title":          result.Title,
				"webserver":      result.Webserver,
				"tech":           strings.Join(result.Tech, ", "),
				"content_length": strconv.Itoa(result.ContentLength),
				"content_type":   result.ContentType,
				"response_time":  result.ResponseTime,
			},
		}
		artifacts = append(artifacts, artifact)

		techStr := ""
		if len(result.Tech) > 0 {
			techStr = " [" + strings.Join(result.Tech, ", ") + "]"
		}
		titleStr := result.Title
		if titleStr == "" {
			titleStr = "(no title)"
		}
		summaries = append(summaries, fmt.Sprintf("%s [%d] %s%s", result.URL, result.Status, titleStr, techStr))
	}

	if len(summaries) == 0 {
		summaries = append(summaries, fmt.Sprintf("httpx: no HTTP response from %s", target))
	}

	return artifacts, strings.Join(summaries, "\n")
}
