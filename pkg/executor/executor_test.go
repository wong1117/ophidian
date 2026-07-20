package executor

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAtomicExecutor_Echo(t *testing.T) {
	cfg := DefaultExecConfig("echo", "hello")
	exe := NewAtomicExecutor("echo-test", cfg)

	result, err := exe.Execute(context.Background(), []string{"world"})

	assert.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello world")
	assert.True(t, result.Duration > 0)
}

func TestAtomicExecutor_WithParser(t *testing.T) {
	cfg := DefaultExecConfig("echo", "Nmap scan report for 192.168.1.1")
	cfg.Timeout = 5 * time.Second
	exe := NewAtomicExecutor("nmap-parser-test", cfg).WithParser(&NMAPParser{})

	result, err := exe.Execute(context.Background(), nil)
	assert.NoError(t, err)
	assert.Greater(t, len(result.Events), 0)
}

func TestAtomicExecutor_Timeout(t *testing.T) {
	cfg := DefaultExecConfig("sleep", "10")
	cfg.Timeout = 50 * time.Millisecond
	exe := NewAtomicExecutor("timeout-test", cfg)

	_, err := exe.Execute(context.Background(), nil)

	assert.Error(t, err)
}

func TestAtomicExecutor_FailedCommand(t *testing.T) {
	cfg := DefaultExecConfig("nonexistent-command-xyz")
	cfg.Timeout = 1 * time.Second
	exe := NewAtomicExecutor("fail-test", cfg)

	_, err := exe.Execute(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "executor start")
}

func TestNMAPParser_ParseLine(t *testing.T) {
	p := &NMAPParser{}

	t.Run("open port", func(t *testing.T) {
		result := p.ParseLine("80/tcp open http", "stdout")
		assert.Equal(t, "80/tcp", result["port"])
		assert.Equal(t, "open", result["state"])
		assert.Equal(t, "http", result["service"])
	})

	t.Run("scan report", func(t *testing.T) {
		result := p.ParseLine("Nmap scan report for example.com (192.168.1.1)", "stdout")
		assert.Equal(t, "192.168.1.1", result["host"])
	})
}

func TestSQLMapParser_ParseLine(t *testing.T) {
	p := &SQLMapParser{}

	t.Run("vulnerable param", func(t *testing.T) {
		result := p.ParseLine("sqlmap identified the following injection point with a total of 1 HTTP(s) requests: parameter 'id' is vulnerable", "stdout")
		assert.Equal(t, true, result["vulnerable"])
	})

	t.Run("info", func(t *testing.T) {
		result := p.ParseLine("[INFO] testing connection to the target URL", "stdout")
		assert.Equal(t, "INFO", result["level"])
	})
}

func TestNucleiParser_ParseLine(t *testing.T) {
	p := &NucleiParser{}

	result := p.ParseLine("[CRITICAL] cve-2024-0001 https://example.com/vuln", "stdout")
	assert.Equal(t, "CRITICAL", result["severity"])
	assert.Equal(t, "cve-2024-0001", result["template"])
	assert.Equal(t, "https://example.com/vuln", result["target"])
}

func TestPayloadInjector_Inject(t *testing.T) {
	inj := NewPayloadInjector()
	args := []string{"-u", "{{target}}", "-p", "{{port}}", "--data", "{{payload}}"}

	result := inj.Inject(args, map[string]string{
		"target":  "example.com",
		"port":    "8080",
		"payload": "' OR 1=1",
	})

	assert.Equal(t, "-u", result[0])
	assert.Equal(t, "example.com", result[1])
	assert.Equal(t, "8080", result[3])
	assert.Equal(t, "' OR 1=1", result[5])
}

func TestPayloadInjector_InjectMany(t *testing.T) {
	inj := NewPayloadInjector()
	baseArgs := []string{"-u", "{{target}}"}

	results := inj.InjectMany(baseArgs, []map[string]string{
		{"target": "site-a.com"},
		{"target": "site-b.com"},
		{"target": "site-c.com"},
	})

	assert.Len(t, results, 3)
	assert.Equal(t, "site-a.com", results[0][1])
	assert.Equal(t, "site-c.com", results[2][1])
}

func TestNormalizer_ToOphidianEvent(t *testing.T) {
	n := &Normalizer{}
	result := &ExecutionResult{
		Tool:     "nmap",
		Command:  "nmap",
		Args:     []string{"-sV", "localhost"},
		ExitCode: 0,
		Stdout:   "Port 80 open",
		Events: []OutputEvent{
			{Stream: "stdout", Parsed: map[string]interface{}{"port": "80", "state": "open", "service": "http"}},
			{Stream: "stdout", Parsed: map[string]interface{}{"host": "localhost"}},
		},
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	event := n.ToOphidianEvent(result)

	assert.True(t, event.Success)
	assert.Equal(t, "nmap", event.ToolName)
	assert.NotNil(t, event.ParsedData)

	var finding OphidianFinding
	json.Unmarshal(event.ParsedData, &finding)
	assert.Equal(t, "80", finding.Ports[0])
	assert.Equal(t, "http", finding.Services[0])
	assert.Equal(t, "localhost", finding.Host)
}

func TestToolRegistry_RegisterAndGet(t *testing.T) {
	reg := NewToolRegistry()

	exe := NewAtomicExecutor("custom-tool", DefaultExecConfig("echo"))
	reg.Register(exe)

	found, ok := reg.Get("custom-tool")
	assert.True(t, ok)
	assert.Equal(t, "custom-tool", found.Name())

	_, ok = reg.Get("nonexistent")
	assert.False(t, ok)

	list := reg.List()
	assert.Contains(t, list, "custom-tool")
}

func TestExecutorFactory(t *testing.T) {
	nmap := NewNMAPExecutor("localhost")
	assert.NotNil(t, nmap)
	assert.Equal(t, "nmap", nmap.Name())

	sqlmap := NewSQLMapExecutor("http://localhost")
	assert.NotNil(t, sqlmap)
	assert.Equal(t, "sqlmap", sqlmap.Name())

	nuclei := NewNucleiExecutor("http://localhost")
	assert.NotNil(t, nuclei)
	assert.Equal(t, "nuclei", nuclei.Name())

	generic := NewGenericExecutor("custom-tool", "-flag", "value")
	assert.NotNil(t, generic)
	assert.Equal(t, "custom-tool", generic.Name())
}

func TestAtomicExecutor_Stdin(t *testing.T) {
	cfg := DefaultExecConfig("cat")
	cfg.Stdin = "test input"
	exe := NewAtomicExecutor("stdin-test", cfg)

	result, err := exe.Execute(context.Background(), nil)

	assert.NoError(t, err)
	assert.Contains(t, result.Stdout, "test input")
}

func TestAtomicExecutor_Env(t *testing.T) {
	cfg := DefaultExecConfig("sh", "-c", "echo $TEST_ENV")
	cfg.Env = map[string]string{"TEST_ENV": "hello-env"}
	cfg.Timeout = 5 * time.Second
	exe := NewAtomicExecutor("env-test", cfg)

	result, err := exe.Execute(context.Background(), nil)

	assert.NoError(t, err)
	assert.Contains(t, result.Stdout, "hello-env")
}

func BenchmarkAtomicExecutor_Echo(b *testing.B) {
	cfg := DefaultExecConfig("echo", "hello")
	exe := NewAtomicExecutor("bench", cfg)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exe.Execute(ctx, nil)
	}
}

// verify that system tools are available for tests
func init() {
	if _, err := exec.LookPath("echo"); err != nil {
		// Skip tests that need echo
	}
}
