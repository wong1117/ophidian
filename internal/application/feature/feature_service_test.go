package feature

import (
	"context"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainFeature "github.com/ophidian/ophidian/internal/domain/feature"
	"github.com/stretchr/testify/assert"
)

type testRepo struct {
	features map[string]*domainFeature.Feature
	audit    []*domainFeature.AuditEntry
}

func newTestRepo() *testRepo {
	return &testRepo{
		features: make(map[string]*domainFeature.Feature),
	}
}

func (r *testRepo) Save(ctx context.Context, f *domainFeature.Feature) error {
	r.features[f.Key] = f
	return nil
}

func (r *testRepo) FindByID(ctx context.Context, id string) (*domainFeature.Feature, error) {
	for _, f := range r.features {
		if f.ID.String() == id {
			return f, nil
		}
	}
	return nil, nil
}

func (r *testRepo) FindByKey(ctx context.Context, key string) (*domainFeature.Feature, error) {
	f, ok := r.features[key]
	if !ok {
		return nil, nil
	}
	return f, nil
}

func (r *testRepo) FindAll(ctx context.Context) ([]*domainFeature.Feature, error) {
	result := make([]*domainFeature.Feature, 0, len(r.features))
	for _, f := range r.features {
		result = append(result, f)
	}
	return result, nil
}

func (r *testRepo) Update(ctx context.Context, f *domainFeature.Feature) error {
	r.features[f.Key] = f
	return nil
}

func (r *testRepo) Delete(ctx context.Context, id string) error {
	for k, f := range r.features {
		if f.ID.String() == id {
			delete(r.features, k)
			return nil
		}
	}
	return nil
}

func (r *testRepo) SaveAudit(ctx context.Context, entry *domainFeature.AuditEntry) error {
	r.audit = append(r.audit, entry)
	return nil
}

func TestFeatureService_Create(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)

	f, err := svc.Create(context.Background(), "dark-mode", "Dark Mode", "Enable dark mode UI", true)

	assert.NoError(t, err)
	assert.Equal(t, "dark-mode", f.Key)
	assert.Equal(t, "Dark Mode", f.Name)
	assert.True(t, f.Enabled)
	assert.Equal(t, 100, f.RolloutPct)
}

func TestFeatureService_Create_Duplicate(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	svc.Create(context.Background(), "test", "Test", "", true)

	_, err := svc.Create(context.Background(), "test", "Test2", "", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestFeatureService_Update(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	svc.Create(context.Background(), "test", "Test", "", true)

	f, err := svc.Update(context.Background(), "test", map[string]interface{}{
		"enabled":    false,
		"rollout_pct": 50,
	}, "admin")

	assert.NoError(t, err)
	assert.False(t, f.Enabled)
	assert.Equal(t, 50, f.RolloutPct)
	assert.NotEmpty(t, repo.audit)
}

func TestFeatureService_Update_InvalidRollout(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	svc.Create(context.Background(), "test", "Test", "", true)

	_, err := svc.Update(context.Background(), "test", map[string]interface{}{
		"rollout_pct": 150,
	}, "admin")

	assert.Error(t, err)
}

func TestFeatureService_Delete(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	svc.Create(context.Background(), "test", "Test", "", true)

	err := svc.Delete(context.Background(), "test", "admin")
	assert.NoError(t, err)
	assert.NotEmpty(t, repo.audit)
}

func TestFeatureService_IsEnabled_Basic(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	svc.Create(context.Background(), "enabled-flag", "Test", "", true)

	assert.True(t, svc.IsEnabled(context.Background(), "enabled-flag"))
}

func TestFeatureService_IsEnabled_Disabled(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	svc.Create(context.Background(), "disabled-flag", "Test", "", false)

	assert.False(t, svc.IsEnabled(context.Background(), "disabled-flag"))
}

func TestFeatureService_IsEnabled_Missing(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)

	assert.False(t, svc.IsEnabled(context.Background(), "nonexistent"))
}

func TestFeatureService_IsEnabled_Environment(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	f, _ := svc.Create(context.Background(), "env-flag", "Test", "", true)
	f.Environments = []string{"production"}
	repo.Update(context.Background(), f)

	assert.True(t, svc.IsEnabled(context.Background(), "env-flag", WithEnvironment("production")))
	assert.False(t, svc.IsEnabled(context.Background(), "env-flag", WithEnvironment("staging")))
}

func TestFeatureService_IsEnabled_Tenant(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	f, _ := svc.Create(context.Background(), "tenant-flag", "Test", "", true)
	f.Tenants = []string{"tenant-a", "tenant-b"}
	repo.Update(context.Background(), f)

	assert.True(t, svc.IsEnabled(context.Background(), "tenant-flag", WithTenant("tenant-a")))
	assert.False(t, svc.IsEnabled(context.Background(), "tenant-flag", WithTenant("tenant-c")))
}

func TestFeatureService_IsEnabled_Rollout(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	f, _ := svc.Create(context.Background(), "rollout-flag", "Test", "", true)
	f.RolloutPct = 50
	repo.Update(context.Background(), f)

	enabled := 0
	disabled := 0
	for i := 0; i < 1000; i++ {
		if svc.IsEnabled(context.Background(), "rollout-flag", WithTarget(common.NewID().String())) {
			enabled++
		} else {
			disabled++
		}
	}
	assert.Greater(t, enabled, 300)
	assert.Greater(t, disabled, 300)
}

func TestFeatureService_RefreshCache(t *testing.T) {
	repo := newTestRepo()
	svc := NewFeatureService(repo)
	svc.Create(context.Background(), "a", "A", "", true)
	svc.Create(context.Background(), "b", "B", "", false)

	err := svc.RefreshCache(context.Background())
	assert.NoError(t, err)
}

func TestContainsFold(t *testing.T) {
	assert.True(t, containsFold([]string{"A", "B"}, "a"))
	assert.False(t, containsFold([]string{"A", "B"}, "c"))
	assert.False(t, containsFold(nil, "a"))
}

func TestIsInRollout(t *testing.T) {
	seen := make(map[bool]bool)
	for i := 0; i < 100; i++ {
		seen[isInRollout(common.NewID().String(), 50)] = true
	}
	assert.True(t, seen[true])
	assert.True(t, seen[false])
}
