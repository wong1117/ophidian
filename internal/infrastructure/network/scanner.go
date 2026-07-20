package network

import (
	"context"
	"fmt"
	"net"
	"time"
)

type PortScanner struct {
	timeout time.Duration
}

func NewPortScanner(timeout time.Duration) *PortScanner {
	return &PortScanner{timeout: timeout}
}

func (s *PortScanner) ScanTCP(ctx context.Context, host string, port int) (bool, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, s.timeout)
	if err != nil {
		return false, nil
	}
	conn.Close()
	return true, nil
}
