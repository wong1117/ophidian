package network

import (
	"net"
)

type Tunnel struct {
	LocalAddr  string
	RemoteAddr string
	Type       string
	listener   net.Listener
}

func NewTunnel(local, remote string) *Tunnel {
	return &Tunnel{
		LocalAddr:  local,
		RemoteAddr: remote,
	}
}
