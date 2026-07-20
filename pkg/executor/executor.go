package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type ExternalExecutor interface {
	Name() string
	Execute(ctx context.Context, args []string) (*ExecutionResult, error)
	Version() string
}

type ExecutionResult struct {
	Tool      string
	Command   string
	Args      []string
	ExitCode  int
	Stdout    string
	Stderr    string
	Duration  time.Duration
	StartedAt time.Time
	EndedAt   time.Time
	Events    []OutputEvent
	RawJSON   json.RawMessage
	Error     string
}

type OutputEvent struct {
	Timestamp time.Time
	Stream    string
	Line      string
	Level     string
	Parsed    map[string]interface{}
}

type ExecConfig struct {
	Command string
	Args    []string
	Timeout time.Duration
	Env     map[string]string
	WorkDir string
	Stdin   string
}

func DefaultExecConfig(cmd string, args ...string) ExecConfig {
	return ExecConfig{
		Command: cmd,
		Args:    args,
		Timeout: 5 * time.Minute,
	}
}

type AtomicExecutor struct {
	name    string
	config  ExecConfig
	parser  OutputParser
	timeout time.Duration
}

func NewAtomicExecutor(name string, cfg ExecConfig) *AtomicExecutor {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	return &AtomicExecutor{
		name:    name,
		config:  cfg,
		timeout: cfg.Timeout,
	}
}

func (e *AtomicExecutor) Name() string    { return e.name }
func (e *AtomicExecutor) Version() string { return "" }

func (e *AtomicExecutor) WithParser(p OutputParser) *AtomicExecutor {
	e.parser = p
	return e
}

func (e *AtomicExecutor) Execute(ctx context.Context, args []string) (*ExecutionResult, error) {
	allArgs := append(e.config.Args, args...)
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.config.Command, allArgs...)
	if e.config.WorkDir != "" {
		cmd.Dir = e.config.WorkDir
	}
	for k, v := range e.config.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if e.config.Stdin != "" {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("executor stdin pipe: %w", err)
		}
		go func() {
			defer stdin.Close()
			io.WriteString(stdin, e.config.Stdin)
		}()
	}

	var stdoutBuf, stderrBuf strings.Builder
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	var events []OutputEvent
	var mu sync.Mutex

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("executor start: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")
			if e.parser != nil {
				parsed := e.parser.ParseLine(line, "stdout")
				mu.Lock()
				events = append(events, OutputEvent{
					Timestamp: time.Now(), Stream: "stdout", Line: line, Parsed: parsed,
				})
				mu.Unlock()
			}
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			if e.parser != nil {
				parsed := e.parser.ParseLine(line, "stderr")
				mu.Lock()
				events = append(events, OutputEvent{
					Timestamp: time.Now(), Stream: "stderr", Line: line, Parsed: parsed,
					Level: "ERROR",
				})
				mu.Unlock()
			}
		}
	}()

	wg.Wait()
	duration := time.Since(start)
	end := time.Now()

	exitCode := 0
	waitErr := cmd.Wait()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := &ExecutionResult{
		Tool:      e.name,
		Command:   e.config.Command,
		Args:      allArgs,
		ExitCode:  exitCode,
		Stdout:    stdoutBuf.String(),
		Stderr:    stderrBuf.String(),
		Duration:  duration,
		StartedAt: start,
		EndedAt:   end,
		Events:    events,
	}

	if exitCode != 0 {
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, strings.TrimSpace(stderrBuf.String()))
	}

	if exitCode != 0 {
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, strings.TrimSpace(stderrBuf.String()))
	}

	if ctx.Err() != nil {
		return nil, fmt.Errorf("executor timeout: %w", ctx.Err())
	}

	return result, nil
}

func (e *AtomicExecutor) ForceKill() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kill", "-9")
	_ = cmd
	return nil
}

type OutputParser interface {
	ParseLine(line, stream string) map[string]interface{}
}

type NMAPParser struct{}

func (p *NMAPParser) ParseLine(line, stream string) map[string]interface{} {
	result := map[string]interface{}{"source": "nmap", "stream": stream}
	if strings.Contains(line, "open") {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			result["port"] = parts[0]
			result["state"] = parts[1]
			result["service"] = parts[2]
		}
	}
	if strings.Contains(line, "Nmap scan report") {
		parts := strings.Fields(line)
		for i, p := range parts {
			if strings.HasPrefix(p, "(") && strings.HasSuffix(p, ")") {
				result["host"] = p[1 : len(p)-1]
				break
			}
			_ = i
		}
	}
	return result
}

type SQLMapParser struct{}

func (p *SQLMapParser) ParseLine(line, stream string) map[string]interface{} {
	result := map[string]interface{}{"source": "sqlmap", "stream": stream}
	if strings.Contains(line, "parameter") && strings.Contains(line, "vulnerable") {
		result["vulnerable"] = true
		result["severity"] = "CRITICAL"
	}
	if strings.Contains(line, "[INFO]") {
		result["level"] = "INFO"
	}
	if strings.Contains(line, "[WARNING]") {
		result["level"] = "WARNING"
	}
	return result
}

type NucleiParser struct{}

func (p *NucleiParser) ParseLine(line, stream string) map[string]interface{} {
	result := map[string]interface{}{"source": "nuclei", "stream": stream}
	if strings.Contains(line, "[") && strings.Contains(line, "]") {
		start := strings.Index(line, "[")
		end := strings.Index(line, "]")
		if start >= 0 && end > start {
			severity := line[start+1 : end]
			result["severity"] = severity
			parts := strings.SplitN(line[end+1:], " ", 3)
			if len(parts) >= 3 {
				result["template"] = strings.TrimSpace(parts[1])
				result["target"] = strings.TrimSpace(parts[2])
			}
		}
	}
	return result
}

type GenericParser struct {
	format string
}

func (p *GenericParser) ParseLine(line, stream string) map[string]interface{} {
	return map[string]interface{}{"source": "generic", "stream": stream, "raw": line}
}

type Normalizer struct{}

func (n *Normalizer) ToOphidianEvent(result *ExecutionResult) OphidianToolEvent {
	return OphidianToolEvent{
		ToolName:    result.Tool,
		Command:     result.Command,
		Args:        result.Args,
		ExitCode:    result.ExitCode,
		Success:     result.ExitCode == 0,
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		Duration:    result.Duration,
		StartedAt:   result.StartedAt,
		EndedAt:     result.EndedAt,
		ParsedData:  n.extractParsed(result),
		OutputEvents: result.Events,
		NormalizedAt: time.Now(),
	}
}

func (n *Normalizer) extractParsed(result *ExecutionResult) json.RawMessage {
	var finding OphidianFinding
	finding.Tool = result.Tool
	finding.Duration = result.Duration.String()

	for _, evt := range result.Events {
		if h, ok := evt.Parsed["host"].(string); ok {
			finding.Host = h
		}
		if p, ok := evt.Parsed["port"].(string); ok {
			finding.Ports = append(finding.Ports, p)
		}
		if s, ok := evt.Parsed["service"].(string); ok {
			finding.Services = append(finding.Services, s)
		}
		if v, ok := evt.Parsed["vulnerable"]; ok && v == true {
			finding.Vulnerabilities = append(finding.Vulnerabilities, finding.Tool)
		}
		if s, ok := evt.Parsed["severity"].(string); ok && s != "" {
			finding.Severity = s
		}
		if t, ok := evt.Parsed["template"].(string); ok {
			finding.Template = t
		}
		if tg, ok := evt.Parsed["target"].(string); ok {
			finding.Target = tg
		}
	}

	data, _ := json.Marshal(finding)
	return data
}

type OphidianToolEvent struct {
	ToolName     string          `json:"tool_name"`
	Command      string          `json:"command"`
	Args         []string        `json:"args"`
	ExitCode     int             `json:"exit_code"`
	Success      bool            `json:"success"`
	Stdout       string          `json:"stdout"`
	Stderr       string          `json:"stderr"`
	Duration     time.Duration   `json:"duration"`
	StartedAt    time.Time       `json:"started_at"`
	EndedAt      time.Time       `json:"ended_at"`
	ParsedData   json.RawMessage `json:"parsed_data"`
	OutputEvents []OutputEvent   `json:"output_events"`
	NormalizedAt time.Time       `json:"normalized_at"`
}

type OphidianFinding struct {
	Tool            string   `json:"tool"`
	Host            string   `json:"host,omitempty"`
	Target          string   `json:"target,omitempty"`
	Ports           []string `json:"ports,omitempty"`
	Services        []string `json:"services,omitempty"`
	Vulnerabilities []string `json:"vulnerabilities,omitempty"`
	Severity        string   `json:"severity,omitempty"`
	Template        string   `json:"template,omitempty"`
	Duration        string   `json:"duration"`
}

type PayloadInjector struct{}

func NewPayloadInjector() *PayloadInjector {
	return &PayloadInjector{}
}

func (p *PayloadInjector) Inject(args []string, payloads map[string]string) []string {
	injected := make([]string, len(args))
	copy(injected, args)
	for i := range injected {
		for k, v := range payloads {
			injected[i] = strings.ReplaceAll(injected[i], fmt.Sprintf("{{%s}}", k), v)
		}
	}
	return injected
}

func (p *PayloadInjector) InjectMany(baseArgs []string, batchPayloads []map[string]string) [][]string {
	var result [][]string
	for _, payloads := range batchPayloads {
		result = append(result, p.Inject(baseArgs, payloads))
	}
	return result
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ExternalExecutor
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]ExternalExecutor)}
}

func (r *ToolRegistry) Register(tool ExternalExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (ExternalExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

type NMAPExecutor struct{ *AtomicExecutor }
type SQLMapExecutor struct{ *AtomicExecutor }
type NucleiExecutor struct{ *AtomicExecutor }

func NewNMAPExecutor(target string) *NMAPExecutor {
	cfg := DefaultExecConfig("nmap", "-sV", "-sC", "-oX", "-", target)
	return &NMAPExecutor{NewAtomicExecutor("nmap", cfg).WithParser(&NMAPParser{})}
}

func NewSQLMapExecutor(target string) *SQLMapExecutor {
	cfg := DefaultExecConfig("sqlmap", "-u", target, "--batch", "--random-agent")
	cfg.Timeout = 30 * time.Minute
	return &SQLMapExecutor{NewAtomicExecutor("sqlmap", cfg).WithParser(&SQLMapParser{})}
}

func NewNucleiExecutor(target string) *NucleiExecutor {
	cfg := DefaultExecConfig("nuclei", "-u", target, "-jsonl", "-silent")
	return &NucleiExecutor{NewAtomicExecutor("nuclei", cfg).WithParser(&NucleiParser{})}
}

func NewGenericExecutor(tool string, args ...string) *AtomicExecutor {
	return NewAtomicExecutor(tool, DefaultExecConfig(tool, args...))
}
