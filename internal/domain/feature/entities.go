package feature

import (
	"context"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type Feature struct {
	ID          common.ID
	Key         string
	Name        string
	Description string
	Enabled     bool
	Environments []string
	Tenants     []string
	RolloutPct  int
	Metadata    map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AuditEntry struct {
	ID        common.ID
	FeatureID common.ID
	Action    string
	Field     string
	OldValue  string
	NewValue  string
	ChangedBy string
	Timestamp time.Time
}

type Repository interface {
	Save(ctx context.Context, f *Feature) error
	FindByID(ctx context.Context, id string) (*Feature, error)
	FindByKey(ctx context.Context, key string) (*Feature, error)
	FindAll(ctx context.Context) ([]*Feature, error)
	Update(ctx context.Context, f *Feature) error
	Delete(ctx context.Context, id string) error
	SaveAudit(ctx context.Context, entry *AuditEntry) error
}
