package arsenal

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

type PortResult struct {
	Port    int
	Open    bool
	Service string
	Banner  string
	Latency time.Duration
}

type ScanConfig struct {
	Target    string
	Ports     []int
	Timeout   time.Duration
	Concurrent int
}

func DefaultScanConfig(target string) ScanConfig {
	return ScanConfig{
		Target:     target,
		Ports:      CommonPorts(),
		Timeout:    2 * time.Second,
		Concurrent: 100,
	}
}

func CommonPorts() []int {
	return []int{21, 22, 23, 25, 53, 80, 110, 111, 135, 139, 143, 443, 445, 993, 995,
		1723, 3306, 3389, 5432, 5900, 6379, 8080, 8443, 8888, 9000, 9090, 27017}
}

func TCPConnect(host string, port int, timeout time.Duration) (bool, time.Duration) {
	start := time.Now()
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false, time.Since(start)
	}
	conn.Close()
	return true, time.Since(start)
}

func TCPBanner(host string, port int, timeout time.Duration) (string, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "", fmt.Errorf("banner grab: %w", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(buf[:n])), nil
}

type PortScanner struct {
	config ScanConfig
}

func NewPortScanner(cfg ScanConfig) *PortScanner {
	if cfg.Concurrent <= 0 {
		cfg.Concurrent = 100
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	return &PortScanner{config: cfg}
}

func (s *PortScanner) Scan(ctx context.Context) []PortResult {
	var results []PortResult
	var mu sync.Mutex

	sem := make(chan struct{}, s.config.Concurrent)
	var wg sync.WaitGroup

	for _, port := range s.config.Ports {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()

			open, latency := TCPConnect(s.config.Target, p, s.config.Timeout)
			if !open {
				return
			}

			result := PortResult{Port: p, Open: true, Latency: latency}
			result.Service = guessService(p)
			if banner, err := TCPBanner(s.config.Target, p, 500*time.Millisecond); err == nil {
				result.Banner = banner
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(port)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Port < results[j].Port
	})

	return results
}

func (s *PortScanner) ScanRange(ctx context.Context, ports []int) []PortResult {
	s.config.Ports = ports
	return s.Scan(ctx)
}

func guessService(port int) string {
	services := map[int]string{
		21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp", 53: "dns",
		80: "http", 110: "pop3", 143: "imap", 443: "https", 445: "smb",
		3306: "mysql", 3389: "rdp", 5432: "postgresql", 6379: "redis",
		8080: "http-alt", 8443: "https-alt", 27017: "mongodb",
	}
	if s, ok := services[port]; ok {
		return s
	}
	return "unknown"
}
