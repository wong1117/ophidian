package target

import "github.com/ophidian/ophidian/internal/domain/common"

type Target struct {
	ID        common.ID
	IPs       []IP
	Domains   []Domain
	Hostnames []string
	OS        string
	Services  []Service
	Tags      []string
	CreatedAt common.UTCTime
	UpdatedAt common.UTCTime
}

type Service struct {
	Port     int
	Protocol string
	Name     string
	Banner   string
	Version  string
	State    string
}

type IP struct {
	Address string
	Type    IPType
}

type IPType string

const (
	IPv4 IPType = "IPv4"
	IPv6 IPType = "IPv6"
)

type Domain struct {
	Name    string
	TLD     string
	IsSubdomain bool
}

type Host struct {
	IP        IP
	Hostnames []string
	OS        string
	Services  []Service
	Tags      []string
}
