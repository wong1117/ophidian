package ai

import (
	"sync"
)

type Registry struct {
	providers map[ProviderType]Provider
	mu        sync.RWMutex
}

var globalRegistry = &Registry{
	providers: make(map[ProviderType]Provider),
}

func RegisterProvider(p Provider) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.providers[ProviderType(p.Name())] = p
}

func GetProvider(t ProviderType) (Provider, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	p, ok := globalRegistry.providers[t]
	return p, ok
}

func ListProviders() []ProviderType {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	types := make([]ProviderType, 0, len(globalRegistry.providers))
	for t := range globalRegistry.providers {
		types = append(types, t)
	}
	return types
}
