package arsenal

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPortScanner_Scan(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				conn.Write([]byte("SSH-2.0-OpenSSH_8.9\r\n"))
				conn.Close()
			}
		}
	}()

	time.Sleep(10 * time.Millisecond)

	cfg := ScanConfig{
		Target:     "127.0.0.1",
		Ports:      []int{port},
		Timeout:    1 * time.Second,
		Concurrent: 10,
	}

	scanner := NewPortScanner(cfg)
	results := scanner.Scan(context.Background())

	assert.GreaterOrEqual(t, len(results), 1)
	assert.True(t, results[0].Open)
}

func TestPortScanner_ScanRange(t *testing.T) {
	cfg := ScanConfig{
		Target:     "127.0.0.1",
		Timeout:    500 * time.Millisecond,
		Concurrent: 50,
	}
	scanner := NewPortScanner(cfg)
	results := scanner.ScanRange(context.Background(), []int{9999, 9998})
	assert.Empty(t, results)
}

func TestTCPConnect(t *testing.T) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	open, latency := TCPConnect("127.0.0.1", port, time.Second)
	assert.True(t, open)
	assert.Greater(t, latency, time.Duration(0))
}

func TestTCPBanner(t *testing.T) {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Write([]byte("Apache/2.4.41\r\n"))
			time.Sleep(50 * time.Millisecond)
			conn.Close()
		}
	}()

	banner, err := TCPBanner("127.0.0.1", port, time.Second)
	assert.NoError(t, err)
	assert.Contains(t, banner, "Apache")
}

func TestCommonPorts(t *testing.T) {
	ports := CommonPorts()
	assert.Greater(t, len(ports), 10)
	assert.Contains(t, ports, 80)
	assert.Contains(t, ports, 443)
	assert.Contains(t, ports, 22)
}

func TestGuessService(t *testing.T) {
	assert.Equal(t, "http", guessService(80))
	assert.Equal(t, "ssh", guessService(22))
	assert.Equal(t, "redis", guessService(6379))
	assert.Equal(t, "unknown", guessService(12345))
}

func TestWAFFingerprinter_Fingerprint(t *testing.T) {
	fp := NewWAFFingerprinter()

	headers := map[string]string{
		"Server":  "cloudflare",
		"CF-RAY":  "abc123",
	}
	body := "Attention Required! | Cloudflare"
	result := fp.Fingerprint(headers, body, 403)

	assert.True(t, result.Name == "Cloudflare" || result.Name == "Fortinet FortiWeb")
	assert.Greater(t, result.Confidence, 0.2)
}

func TestWAFFingerprinter_Unknown(t *testing.T) {
	fp := NewWAFFingerprinter()
	headers := map[string]string{"Server": "nginx"}
	result := fp.Fingerprint(headers, "", 200)

	assert.False(t, result.Detected)
	assert.Equal(t, "Unknown", result.Name)
}

func TestWAFMutator_Mutate(t *testing.T) {
	m := NewWAFMutator()
	variants := m.Mutate("SELECT * FROM users")

	assert.Greater(t, len(variants), 3)
	for _, v := range variants {
		assert.NotEmpty(t, v)
	}
}

func TestWAFMutator_MutateBatch(t *testing.T) {
	m := NewWAFMutator()
	results := m.MutateBatch([]string{"test1", "test2"})

	assert.Len(t, results, 2)
	assert.Greater(t, len(results["test1"]), 1)
}

func TestSQLiGenerator(t *testing.T) {
	g := &SQLiGenerator{}
	payloads := g.Generate("test")
	assert.Greater(t, len(payloads), 1)
	assert.Equal(t, "SQLi Payload Generator", g.Name())
	assert.Equal(t, "INJECTION", g.Category())
}

func TestXSSGenerator(t *testing.T) {
	g := &XSSGenerator{}
	payloads := g.Generate("test")
	assert.Greater(t, len(payloads), 1)
}

func TestSandbox_Execute(t *testing.T) {
	s := NewSandbox()
	s.Load("payload-a")
	s.Load("payload-b")

	results := s.Execute(context.Background(), func(payload string) (string, error) {
		return "processed:" + payload, nil
	})

	assert.Len(t, results, 2)
	assert.Equal(t, "payload-a", results[0].Payload)
	assert.Equal(t, "processed:payload-a", results[0].Output)
	assert.Empty(t, results[0].Error)
	assert.Empty(t, s.payloads)
}

func TestSandbox_ContextCancellation(t *testing.T) {
	s := NewSandbox()
	s.Load("slow-payload")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := s.Execute(ctx, func(payload string) (string, error) {
		time.Sleep(time.Second)
		return payload, nil
	})

	assert.Len(t, results, 1)
	assert.NotEmpty(t, results[0].Error)
}

func BenchmarkPortScanner_Scan(b *testing.B) {
	cfg := ScanConfig{
		Target:     "127.0.0.1",
		Ports:      CommonPorts(),
		Timeout:    500 * time.Millisecond,
		Concurrent: 50,
	}
	scanner := NewPortScanner(cfg)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.Scan(ctx)
	}
}
