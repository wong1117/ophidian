package target

import (
	"net"
)

type CIDR struct {
	Raw string
	Net *net.IPNet
}

func ParseCIDR(s string) (*CIDR, error) {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	return &CIDR{Raw: s, Net: ipnet}, nil
}

func (c *CIDR) Contains(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil || c.Net == nil {
		return false
	}
	return c.Net.Contains(parsed)
}

type PortRange struct {
	Start int
	End   int
}

func (p PortRange) IsValid() bool {
	return p.Start > 0 && p.End >= p.Start && p.End <= 65535
}

func (p PortRange) Size() int {
	return p.End - p.Start + 1
}

type Banner struct {
	Service string
	Version string
	Raw     string
	Parsed  map[string]string
}

type Protocol string

const (
	ProtocolTCP  Protocol = "TCP"
	ProtocolUDP  Protocol = "UDP"
	ProtocolHTTP Protocol = "HTTP"
	ProtocolHTTPS Protocol = "HTTPS"
	ProtocolDNS  Protocol = "DNS"
	ProtocolSSH  Protocol = "SSH"
	ProtocolSMB  Protocol = "SMB"
	ProtocolLDAP Protocol = "LDAP"
)
