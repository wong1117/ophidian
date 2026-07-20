package tenant

import (
	"context"
	"testing"

	domainTenant "github.com/ophidian/ophidian/internal/domain/tenant"
	"github.com/stretchr/testify/assert"
)

type testTenantRepo struct {
	tenants  map[string]*domainTenant.Tenant
	projects map[string]*domainTenant.TenantProject
	users    map[string]*domainTenant.TenantUser
}

func newTestTenantRepo() *testTenantRepo {
	return &testTenantRepo{
		tenants:  make(map[string]*domainTenant.Tenant),
		projects: make(map[string]*domainTenant.TenantProject),
		users:    make(map[string]*domainTenant.TenantUser),
	}
}

func (r *testTenantRepo) Save(ctx context.Context, t *domainTenant.Tenant) error {
	r.tenants[t.ID.String()] = t
	return nil
}

func (r *testTenantRepo) FindByID(ctx context.Context, id string) (*domainTenant.Tenant, error) {
	t, ok := r.tenants[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (r *testTenantRepo) FindBySlug(ctx context.Context, slug string) (*domainTenant.Tenant, error) {
	for _, t := range r.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, nil
}

func (r *testTenantRepo) FindAll(ctx context.Context) ([]*domainTenant.Tenant, error) {
	result := make([]*domainTenant.Tenant, 0, len(r.tenants))
	for _, t := range r.tenants {
		result = append(result, t)
	}
	return result, nil
}

func (r *testTenantRepo) Update(ctx context.Context, t *domainTenant.Tenant) error {
	r.tenants[t.ID.String()] = t
	return nil
}

func (r *testTenantRepo) SoftDelete(ctx context.Context, id string) error {
	if t, ok := r.tenants[id]; ok {
		t.Status = domainTenant.StatusInactive
	}
	return nil
}

func (r *testTenantRepo) SaveProject(ctx context.Context, tp *domainTenant.TenantProject) error {
	r.projects[tp.ID.String()] = tp
	return nil
}

func (r *testTenantRepo) FindProjectsByTenant(ctx context.Context, tenantID string) ([]*domainTenant.TenantProject, error) {
	var result []*domainTenant.TenantProject
	for _, tp := range r.projects {
		if tp.TenantID.String() == tenantID {
			result = append(result, tp)
		}
	}
	return result, nil
}

func (r *testTenantRepo) SaveUser(ctx context.Context, tu *domainTenant.TenantUser) error {
	r.users[tu.ID.String()] = tu
	return nil
}

func (r *testTenantRepo) FindUsersByTenant(ctx context.Context, tenantID string) ([]*domainTenant.TenantUser, error) {
	var result []*domainTenant.TenantUser
	for _, tu := range r.users {
		if tu.TenantID.String() == tenantID {
			result = append(result, tu)
		}
	}
	return result, nil
}

func (r *testTenantRepo) FindTenantsByUser(ctx context.Context, userID string) ([]*domainTenant.Tenant, error) {
	var result []*domainTenant.Tenant
	for _, tu := range r.users {
		if tu.UserID.String() == userID {
			if t, ok := r.tenants[tu.TenantID.String()]; ok {
				result = append(result, t)
			}
		}
	}
	return result, nil
}

func TestTenantService_Create(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	tenant, err := svc.Create(context.Background(), "Acme Corp", "acme-corp", "enterprise")

	assert.NoError(t, err)
	assert.NotNil(t, tenant)
	assert.Equal(t, "Acme Corp", tenant.Name)
	assert.Equal(t, "acme-corp", tenant.Slug)
	assert.Equal(t, "enterprise", tenant.Plan)
	assert.Equal(t, domainTenant.StatusActive, tenant.Status)
}

func TestTenantService_Create_DuplicateSlug(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	_, _ = svc.Create(context.Background(), "First", "test-slug", "basic")
	_, err := svc.Create(context.Background(), "Second", "test-slug", "basic")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestTenantService_Create_Validation(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	_, err := svc.Create(context.Background(), "", "slug", "basic")
	assert.Error(t, err)

	_, err = svc.Create(context.Background(), "Name", "", "basic")
	assert.Error(t, err)

	_, err = svc.Create(context.Background(), "Name", "bad/slug", "basic")
	assert.Error(t, err)
}

func TestTenantService_Update(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	created, _ := svc.Create(context.Background(), "Original", "orig", "basic")
	updated, err := svc.Update(context.Background(), created.ID.String(), "Updated Name", "pro")

	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "pro", updated.Plan)
}

func TestTenantService_Update_Inactive(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	created, _ := svc.Create(context.Background(), "Test", "test", "basic")
	_ = svc.Delete(context.Background(), created.ID.String())

	_, err := svc.Update(context.Background(), created.ID.String(), "New", "pro")
	assert.Error(t, err)
}

func TestTenantService_Delete(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	created, _ := svc.Create(context.Background(), "Test", "test", "basic")
	err := svc.Delete(context.Background(), created.ID.String())

	assert.NoError(t, err)

	stored, _ := repo.FindByID(context.Background(), created.ID.String())
	assert.Equal(t, domainTenant.StatusInactive, stored.Status)
}

func TestTenantService_AddProject(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	created, _ := svc.Create(context.Background(), "Test", "test", "basic")
	tp, err := svc.AddProject(context.Background(), created.ID.String(), "proj-1", "Project One")

	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.Equal(t, "proj-1", tp.ProjectID.String())

	projects, _ := svc.GetProjects(context.Background(), created.ID.String())
	assert.Len(t, projects, 1)
}

func TestTenantService_AddUser(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	created, _ := svc.Create(context.Background(), "Test", "test", "basic")
	tu, err := svc.AddUser(context.Background(), created.ID.String(), "user-1", "admin")

	assert.NoError(t, err)
	assert.NotNil(t, tu)

	tenants, _ := svc.GetTenantsForUser(context.Background(), "user-1")
	assert.Len(t, tenants, 1)
}

func TestTenantService_GetUsers(t *testing.T) {
	repo := newTestTenantRepo()
	svc := NewTenantService(repo)

	created, _ := svc.Create(context.Background(), "Test", "test", "basic")
	svc.AddUser(context.Background(), created.ID.String(), "u1", "admin")
	svc.AddUser(context.Background(), created.ID.String(), "u2", "viewer")

	users, err := svc.GetUsers(context.Background(), created.ID.String())

	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestTenantContext(t *testing.T) {
	ctx := context.Background()
	id, ok := domainTenant.TenantIDFrom(ctx)
	assert.False(t, ok)
	assert.Empty(t, id)

	ctx = domainTenant.WithTenant(ctx, "tenant-123")
	id, ok = domainTenant.TenantIDFrom(ctx)
	assert.True(t, ok)
	assert.Equal(t, "tenant-123", id)
}
