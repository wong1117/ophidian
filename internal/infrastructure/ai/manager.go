package ai

import (
	"context"
	"fmt"
	"sort"
)

type Manager struct {
	providers []Provider
	defaultType ProviderType
	fallback   bool
}

func NewManager(cfg MultiProviderConfig) (*Manager, error) {
	manager := &Manager{
		fallback: cfg.Fallback,
	}

	activeCfgs := cfg.GetActiveProviders()
	if len(activeCfgs) == 0 {
		return nil, fmt.Errorf("no active AI providers configured")
	}

	sort.Slice(activeCfgs, func(i, j int) bool {
		return activeCfgs[i].Priority > activeCfgs[j].Priority
	})

	for _, pcfg := range activeCfgs {
		provider, err := NewProviderFromConfig(pcfg)
		if err != nil {
			continue
		}
		if !provider.IsAvailable() {
			continue
		}
		manager.providers = append(manager.providers, provider)
	}

	if len(manager.providers) == 0 {
		return nil, fmt.Errorf("no available AI providers")
	}

	manager.defaultType = manager.providers[0].Name()
	return manager, nil
}

func NewManagerWithProviders(providers []Provider) *Manager {
	return &Manager{
		providers: providers,
		fallback:  true,
	}
}

func (m *Manager) Default() Provider {
	if len(m.providers) == 0 {
		return nil
	}
	return m.providers[0]
}

func (m *Manager) All() []Provider {
	return m.providers
}

func (m *Manager) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	var lastErr error

	for i, provider := range m.providers {
		resp, err := provider.Generate(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if !m.fallback || i == len(m.providers)-1 {
			break
		}
	}

	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

func (m *Manager) GenerateWithFallback(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	for _, provider := range m.providers {
		if !provider.IsAvailable() {
			continue
		}
		resp, err := provider.Generate(ctx, req)
		if err == nil {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("all providers failed")
}
