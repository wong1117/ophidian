package safety

import (
	"context"
	"net"
	"sync"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type ScopeEnforcer struct {
	mu         sync.RWMutex
	allowedCIDRs []*net.IPNet
	excludedCIDRs []*net.IPNet
	enabled      bool
	strictMode   bool
}

func NewScopeEnforcer() *ScopeEnforcer {
	return &ScopeEnforcer{
		allowedCIDRs:  make([]*net.IPNet, 0),
		excludedCIDRs: make([]*net.IPNet, 0),
		enabled:       true,
		strictMode:    true,
	}
}

func (e *ScopeEnforcer) Configure(ctx context.Context, mission *mission.Mission) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.allowedCIDRs = make([]*net.IPNet, 0)
	e.excludedCIDRs = make([]*net.IPNet, 0)

	for _, cidr := range mission.Target.CIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
		e.allowedCIDRs = append(e.allowedCIDRs, ipnet)
	}

	for _, cidr := range mission.RoE.ExcludedNets {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		e.excludedCIDRs = append(e.excludedCIDRs, ipnet)
	}

	return nil
}

func (e *ScopeEnforcer) ValidateConnection(ctx context.Context, targetIP string) error {
	if !e.enabled {
		return nil
	}

	parsedIP := net.ParseIP(targetIP)
	if parsedIP == nil {
		return common.ErrInvalidTarget
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, excluded := range e.excludedCIDRs {
		if excluded.Contains(parsedIP) {
			return common.ErrRoEViolation
		}
	}

	if e.strictMode && len(e.allowedCIDRs) > 0 {
		allowed := false
		for _, allowedNet := range e.allowedCIDRs {
			if allowedNet.Contains(parsedIP) {
				allowed = true
				break
			}
		}
		if !allowed {
			return common.ErrRoEViolation
		}
	}

	return nil
}

func (e *ScopeEnforcer) ValidateTarget(ctx context.Context, target mission.Target) error {
	if !e.enabled {
		return nil
	}

	for _, ip := range target.IPs {
		if err := e.ValidateConnection(ctx, ip); err != nil {
			return err
		}
	}

	return nil
}

func (e *ScopeEnforcer) IsInScope(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, excluded := range e.excludedCIDRs {
		if excluded.Contains(parsedIP) {
			return false
		}
	}

	if len(e.allowedCIDRs) > 0 {
		for _, allowed := range e.allowedCIDRs {
			if allowed.Contains(parsedIP) {
				return true
			}
		}
		return false
	}

	return true
}

func (e *ScopeEnforcer) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

func (e *ScopeEnforcer) SetEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = enabled
}
