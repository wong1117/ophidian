package session

import "github.com/ophidian/ophidian/internal/domain/common"

type Session struct {
	ID            common.ID
	MissionID     common.ID
	TargetID      common.ID
	Type          SessionType
	Protocol      Protocol
	Host          string
	Port          int
	User          string
	PrivilegeLevel PrivilegeLevel
	Status        SessionStatus
	Encryption    Encryption
	EstablishedAt common.UTCTime
	LastActiveAt  common.UTCTime
	ClosedAt      *common.UTCTime
	Metadata      map[string]interface{}
}

type SessionType string

const (
	SessionReverseShell SessionType = "REVERSE_SHELL"
	SessionBindShell    SessionType = "BIND_SHELL"
	SessionBeacon       SessionType = "BEACON"
	SessionWebShell     SessionType = "WEBSHELL"
)

type PrivilegeLevel string

const (
	PrivilegeUser     PrivilegeLevel = "USER"
	PrivilegeAdmin    PrivilegeLevel = "ADMIN"
	PrivilegeSystem   PrivilegeLevel = "SYSTEM"
	PrivilegeRoot     PrivilegeLevel = "ROOT"
)

type SessionStatus string

const (
	SessionActive    SessionStatus = "ACTIVE"
	SessionIdle      SessionStatus = "IDLE"
	SessionLost      SessionStatus = "LOST"
	SessionClosed    SessionStatus = "CLOSED"
)
