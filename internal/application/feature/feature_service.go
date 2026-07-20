package feature

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainFeature "github.com/ophidian/ophidian/internal/domain/feature"
)

type FeatureService struct {
	repo  domainFeature.Repository
	mu    sync.RWMutex
	cache map[string]*domainFeature.Feature
}

func NewFeatureService(repo domainFeature.Repository) *FeatureService {
	return &FeatureService{
		repo:  repo,
		cache: make(map[string]*domainFeature.Feature),
	}
}

func (s *FeatureService) Create(ctx context.Context, key, name, description string, enabled bool) (*domainFeature.Feature, error) {
	if key == "" {
		return nil, fmt.Errorf("feature key is required")
	}

	existing, err := s.repo.FindByKey(ctx, key)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("feature with key %s already exists", key)
	}

	f := &domainFeature.Feature{
		ID:          common.NewID(),
		Key:         strings.ToLower(strings.TrimSpace(key)),
		Name:        name,
		Description: description,
		Enabled:     enabled,
		RolloutPct:  100,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Save(ctx, f); err != nil {
		return nil, fmt.Errorf("create feature: %w", err)
	}

	s.mu.Lock()
	s.cache[f.Key] = f
	s.mu.Unlock()

	return f, nil
}

func (s *FeatureService) Update(ctx context.Context, key string, updates map[string]interface{}, changedBy string) (*domainFeature.Feature, error) {
	f, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("update feature: %w", err)
	}

	for field, newVal := range updates {
		oldVal := ""
		switch field {
		case "name":
			if v, ok := newVal.(string); ok {
				oldVal = f.Name
				f.Name = v
			}
		case "description":
			if v, ok := newVal.(string); ok {
				f.Description = v
			}
		case "enabled":
			if v, ok := newVal.(bool); ok {
				oldVal = fmt.Sprintf("%t", f.Enabled)
				f.Enabled = v
			}
		case "rollout_pct":
			if v, ok := newVal.(int); ok {
				oldVal = fmt.Sprintf("%d", f.RolloutPct)
				if v < 0 || v > 100 {
					return nil, fmt.Errorf("rollout percentage must be 0-100")
				}
				f.RolloutPct = v
			}
		case "environments":
			if v, ok := newVal.([]string); ok {
				oldVal = strings.Join(f.Environments, ",")
				f.Environments = v
			}
		case "tenants":
			if v, ok := newVal.([]string); ok {
				oldVal = strings.Join(f.Tenants, ",")
				f.Tenants = v
			}
		}

		_ = s.repo.SaveAudit(ctx, &domainFeature.AuditEntry{
			ID:        common.NewID(),
			FeatureID: f.ID,
			Action:    "update",
			Field:     field,
			OldValue:  oldVal,
			NewValue:  fmt.Sprintf("%v", newVal),
			ChangedBy: changedBy,
			Timestamp: time.Now(),
		})
	}

	f.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, f); err != nil {
		return nil, fmt.Errorf("update feature: %w", err)
	}

	s.mu.Lock()
	s.cache[f.Key] = f
	s.mu.Unlock()

	return f, nil
}

func (s *FeatureService) Delete(ctx context.Context, key, changedBy string) error {
	f, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		return fmt.Errorf("delete feature: %w", err)
	}

	_ = s.repo.SaveAudit(ctx, &domainFeature.AuditEntry{
		ID:        common.NewID(),
		FeatureID: f.ID,
		Action:    "delete",
		ChangedBy: changedBy,
		Timestamp: time.Now(),
	})

	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()

	return s.repo.Delete(ctx, f.ID.String())
}

func (s *FeatureService) IsEnabled(ctx context.Context, key string, opts ...EvalOption) bool {
	f, err := s.cachedFeature(ctx, key)
	if err != nil || f == nil {
		return false
	}
	return s.evaluate(f, opts...)
}

func (s *FeatureService) evaluate(f *domainFeature.Feature, opts ...EvalOption) bool {
	if !f.Enabled {
		return false
	}

	ctx := &evalContext{}
	for _, opt := range opts {
		opt(ctx)
	}

	if len(f.Environments) > 0 && !containsFold(f.Environments, ctx.environment) {
		return false
	}

	if ctx.tenantID != "" && len(f.Tenants) > 0 && !containsFold(f.Tenants, ctx.tenantID) {
		return false
	}

	if f.RolloutPct < 100 && ctx.targetID != "" {
		return isInRollout(ctx.targetID, f.RolloutPct)
	}

	return true
}

type EvalOption func(*evalContext)

type evalContext struct {
	environment string
	tenantID    string
	targetID    string
}

func WithEnvironment(env string) EvalOption {
	return func(c *evalContext) { c.environment = env }
}

func WithTenant(tenantID string) EvalOption {
	return func(c *evalContext) { c.tenantID = tenantID }
}

func WithTarget(targetID string) EvalOption {
	return func(c *evalContext) { c.targetID = targetID }
}

func (s *FeatureService) cachedFeature(ctx context.Context, key string) (*domainFeature.Feature, error) {
	key = strings.ToLower(key)

	s.mu.RLock()
	if f, ok := s.cache[key]; ok {
		s.mu.RUnlock()
		return f, nil
	}
	s.mu.RUnlock()

	f, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[key] = f
	s.mu.Unlock()

	return f, nil
}

func (s *FeatureService) RefreshCache(ctx context.Context) error {
	all, err := s.repo.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("refresh cache: %w", err)
	}

	s.mu.Lock()
	s.cache = make(map[string]*domainFeature.Feature, len(all))
	for _, f := range all {
		s.cache[f.Key] = f
	}
	s.mu.Unlock()

	return nil
}

func containsFold(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func isInRollout(targetID string, pct int) bool {
	h := fnv.New32a()
	h.Write([]byte(targetID))
	hash := h.Sum32() % 100
	return int(hash) < pct
}
