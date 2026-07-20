package common

import "errors"

var (
	ErrInvalidID          = errors.New("invalid id")
	ErrInvalidTarget      = errors.New("invalid target")
	ErrInvalidScope       = errors.New("invalid scope")
	ErrRoEViolation       = errors.New("rules of engagement violation")
	ErrInvalidState       = errors.New("invalid state transition")
	ErrMissionNotFound    = errors.New("mission not found")
	ErrTaskNotFound       = errors.New("task not found")
	ErrPlanNotFound       = errors.New("plan not found")
	ErrSessionNotFound    = errors.New("session not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrTimeout            = errors.New("operation timed out")
	ErrCircuitOpen        = errors.New("circuit breaker is open")
	ErrInvalidTransition  = errors.New("invalid state transition")
	ErrDuplicateEvent     = errors.New("duplicate event")
	ErrConcurrencyConflict = errors.New("concurrency conflict")
)
