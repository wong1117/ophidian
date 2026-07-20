package tenant

import (
	"context"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
)

type Tenant struct {
	ID        common.ID
	Name      string
	Slug      string
	Status    Status
	Plan      string
	Settings  map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type TenantProject struct {
	ID        common.ID
	TenantID  common.ID
	ProjectID common.ID
	Name      string
	CreatedAt time.Time
}

type TenantUser struct {
	ID        common.ID
	TenantID  common.ID
	UserID    common.ID
	Role      string
	CreatedAt time.Time
}

type TenantRepository interface {
	Save(ctx context.Context, t *Tenant) error
	FindByID(ctx context.Context, id string) (*Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*Tenant, error)
	FindAll(ctx context.Context) ([]*Tenant, error)
	Update(ctx context.Context, t *Tenant) error
	SoftDelete(ctx context.Context, id string) error
	SaveProject(ctx context.Context, tp *TenantProject) error
	FindProjectsByTenant(ctx context.Context, tenantID string) ([]*TenantProject, error)
	SaveUser(ctx context.Context, tu *TenantUser) error
	FindUsersByTenant(ctx context.Context, tenantID string) ([]*TenantUser, error)
	FindTenantsByUser(ctx context.Context, userID string) ([]*Tenant, error)
}

type contextKey string

const TenantIDKey contextKey = "tenant_id"

func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

func TenantIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(TenantIDKey).(string)
	return id, ok
}
