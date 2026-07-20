package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domainTenant "github.com/ophidian/ophidian/internal/domain/tenant"
)

type TenantRepo struct {
	deps RepoDeps
}

func NewTenantRepo(pool *pgxpool.Pool) *TenantRepo {
	return &TenantRepo{deps: repoDepsFromPool(pool)}
}

func NewTenantRepoWithDeps(deps RepoDeps) *TenantRepo {
	return &TenantRepo{deps: deps}
}

func (r *TenantRepo) Save(ctx context.Context, t *domainTenant.Tenant) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO tenants (id, name, slug, status, plan, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		t.ID, t.Name, t.Slug, t.Status, t.Plan, marshalJSON(t.Settings),
		t.CreatedAt, t.UpdatedAt,
	)
	return wrapSaveError(err, "tenant")
}

func (r *TenantRepo) FindByID(ctx context.Context, id string) (*domainTenant.Tenant, error) {
	var t domainTenant.Tenant
	var settingsJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, name, slug, status, plan, settings, created_at, updated_at, deleted_at
		 FROM tenants WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.Plan, &settingsJSON,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tenant not found: %s", id)
		}
		return nil, fmt.Errorf("find tenant: %w", err)
	}
	if len(settingsJSON) > 0 && string(settingsJSON) != "null" {
		unmarshalJSON(settingsJSON, &t.Settings)
	}
	return &t, nil
}

func (r *TenantRepo) FindBySlug(ctx context.Context, slug string) (*domainTenant.Tenant, error) {
	var t domainTenant.Tenant
	var settingsJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, name, slug, status, plan, settings, created_at, updated_at, deleted_at
		 FROM tenants WHERE slug = $1 AND deleted_at IS NULL`, slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.Plan, &settingsJSON,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tenant not found by slug: %s", slug)
		}
		return nil, fmt.Errorf("find tenant by slug: %w", err)
	}
	if len(settingsJSON) > 0 && string(settingsJSON) != "null" {
		unmarshalJSON(settingsJSON, &t.Settings)
	}
	return &t, nil
}

func (r *TenantRepo) FindAll(ctx context.Context) ([]*domainTenant.Tenant, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, name, slug, status, plan, settings, created_at, updated_at, deleted_at
		 FROM tenants WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("find all tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*domainTenant.Tenant
	for rows.Next() {
		var t domainTenant.Tenant
		var settingsJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.Plan, &settingsJSON,
			&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
			return nil, fmt.Errorf("find all tenants scan: %w", err)
		}
		if len(settingsJSON) > 0 && string(settingsJSON) != "null" {
			unmarshalJSON(settingsJSON, &t.Settings)
		}
		tenants = append(tenants, &t)
	}
	return tenants, rows.Err()
}

func (r *TenantRepo) Update(ctx context.Context, t *domainTenant.Tenant) error {
	_, err := r.deps.Exec(ctx,
		`UPDATE tenants SET name = $1, plan = $2, settings = $3, updated_at = $4
		 WHERE id = $5 AND deleted_at IS NULL`,
		t.Name, t.Plan, marshalJSON(t.Settings), t.UpdatedAt, t.ID,
	)
	return wrapUpdateError(err, "tenant")
}

func (r *TenantRepo) SoftDelete(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.deps.Exec(ctx,
		`UPDATE tenants SET status = 'INACTIVE', deleted_at = $1, updated_at = $1
		 WHERE id = $2 AND deleted_at IS NULL`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("soft delete tenant: %w", err)
	}
	return nil
}

func (r *TenantRepo) SaveProject(ctx context.Context, tp *domainTenant.TenantProject) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO tenant_projects (id, tenant_id, project_id, name, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		tp.ID, tp.TenantID, tp.ProjectID, tp.Name, tp.CreatedAt,
	)
	return wrapSaveError(err, "tenant project")
}

func (r *TenantRepo) FindProjectsByTenant(ctx context.Context, tenantID string) ([]*domainTenant.TenantProject, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT tp.id, tp.tenant_id, tp.project_id, tp.name, tp.created_at
		 FROM tenant_projects tp WHERE tp.tenant_id = $1`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find projects by tenant: %w", err)
	}
	defer rows.Close()

	var projects []*domainTenant.TenantProject
	for rows.Next() {
		var tp domainTenant.TenantProject
		if err := rows.Scan(&tp.ID, &tp.TenantID, &tp.ProjectID, &tp.Name, &tp.CreatedAt); err != nil {
			return nil, fmt.Errorf("find projects scan: %w", err)
		}
		projects = append(projects, &tp)
	}
	return projects, rows.Err()
}

func (r *TenantRepo) SaveUser(ctx context.Context, tu *domainTenant.TenantUser) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO tenant_users (id, tenant_id, user_id, role, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		tu.ID, tu.TenantID, tu.UserID, tu.Role, tu.CreatedAt,
	)
	return wrapSaveError(err, "tenant user")
}

func (r *TenantRepo) FindUsersByTenant(ctx context.Context, tenantID string) ([]*domainTenant.TenantUser, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT tu.id, tu.tenant_id, tu.user_id, tu.role, tu.created_at
		 FROM tenant_users tu WHERE tu.tenant_id = $1`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find users by tenant: %w", err)
	}
	defer rows.Close()

	var users []*domainTenant.TenantUser
	for rows.Next() {
		var tu domainTenant.TenantUser
		if err := rows.Scan(&tu.ID, &tu.TenantID, &tu.UserID, &tu.Role, &tu.CreatedAt); err != nil {
			return nil, fmt.Errorf("find users scan: %w", err)
		}
		users = append(users, &tu)
	}
	return users, rows.Err()
}

func (r *TenantRepo) FindTenantsByUser(ctx context.Context, userID string) ([]*domainTenant.Tenant, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT t.id, t.name, t.slug, t.status, t.plan, t.settings, t.created_at, t.updated_at, t.deleted_at
		 FROM tenants t
		 JOIN tenant_users tu ON t.id = tu.tenant_id
		 WHERE tu.user_id = $1 AND t.deleted_at IS NULL
		 ORDER BY t.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("find tenants by user: %w", err)
	}
	defer rows.Close()

	var tenants []*domainTenant.Tenant
	for rows.Next() {
		var t domainTenant.Tenant
		var settingsJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.Plan, &settingsJSON,
			&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
			return nil, fmt.Errorf("find tenants by user scan: %w", err)
		}
		if len(settingsJSON) > 0 && string(settingsJSON) != "null" {
			unmarshalJSON(settingsJSON, &t.Settings)
		}
		tenants = append(tenants, &t)
	}
	return tenants, rows.Err()
}
