package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domainFeature "github.com/ophidian/ophidian/internal/domain/feature"
)

type FeatureRepo struct {
	deps RepoDeps
}

func NewFeatureRepo(pool *pgxpool.Pool) *FeatureRepo {
	return &FeatureRepo{deps: repoDepsFromPool(pool)}
}

func (r *FeatureRepo) Save(ctx context.Context, f *domainFeature.Feature) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO features (id, key, name, description, enabled, environments, tenants, rollout_pct, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		f.ID, f.Key, f.Name, f.Description, f.Enabled,
		marshalJSON(f.Environments), marshalJSON(f.Tenants),
		f.RolloutPct, marshalJSON(f.Metadata),
		f.CreatedAt, f.UpdatedAt,
	)
	return wrapSaveError(err, "feature")
}

func (r *FeatureRepo) FindByID(ctx context.Context, id string) (*domainFeature.Feature, error) {
	return r.scanFeature(ctx, `SELECT id, key, name, description, enabled, environments, tenants, rollout_pct, metadata, created_at, updated_at
		 FROM features WHERE id = $1`, id)
}

func (r *FeatureRepo) FindByKey(ctx context.Context, key string) (*domainFeature.Feature, error) {
	return r.scanFeature(ctx, `SELECT id, key, name, description, enabled, environments, tenants, rollout_pct, metadata, created_at, updated_at
		 FROM features WHERE key = $1`, key)
}

func (r *FeatureRepo) FindAll(ctx context.Context) ([]*domainFeature.Feature, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, key, name, description, enabled, environments, tenants, rollout_pct, metadata, created_at, updated_at
		 FROM features ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("find all features: %w", err)
	}
	defer rows.Close()

	var features []*domainFeature.Feature
	for rows.Next() {
		f, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		features = append(features, f)
	}
	return features, rows.Err()
}

func (r *FeatureRepo) scanFeature(ctx context.Context, query string, args ...interface{}) (*domainFeature.Feature, error) {
	row := r.deps.QueryRow(ctx, query, args...)
	var f domainFeature.Feature
	var envJSON, tenantJSON, metaJSON []byte
	err := row.Scan(&f.ID, &f.Key, &f.Name, &f.Description, &f.Enabled,
		&envJSON, &tenantJSON, &f.RolloutPct, &metaJSON,
		&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("feature not found")
		}
		return nil, fmt.Errorf("scan feature: %w", err)
	}
	unmarshalJSON(envJSON, &f.Environments)
	unmarshalJSON(tenantJSON, &f.Tenants)
	if len(metaJSON) > 0 && string(metaJSON) != "null" {
		unmarshalJSON(metaJSON, &f.Metadata)
	}
	return &f, nil
}

func (r *FeatureRepo) scanRow(rows pgx.Rows) (*domainFeature.Feature, error) {
	var f domainFeature.Feature
	var envJSON, tenantJSON, metaJSON []byte
	err := rows.Scan(&f.ID, &f.Key, &f.Name, &f.Description, &f.Enabled,
		&envJSON, &tenantJSON, &f.RolloutPct, &metaJSON,
		&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan feature row: %w", err)
	}
	unmarshalJSON(envJSON, &f.Environments)
	unmarshalJSON(tenantJSON, &f.Tenants)
	if len(metaJSON) > 0 && string(metaJSON) != "null" {
		unmarshalJSON(metaJSON, &f.Metadata)
	}
	return &f, nil
}

func (r *FeatureRepo) Update(ctx context.Context, f *domainFeature.Feature) error {
	_, err := r.deps.Exec(ctx,
		`UPDATE features SET name = $1, description = $2, enabled = $3, environments = $4,
		 tenants = $5, rollout_pct = $6, metadata = $7, updated_at = $8
		 WHERE id = $9`,
		f.Name, f.Description, f.Enabled,
		marshalJSON(f.Environments), marshalJSON(f.Tenants),
		f.RolloutPct, marshalJSON(f.Metadata), f.UpdatedAt, f.ID,
	)
	return wrapUpdateError(err, "feature")
}

func (r *FeatureRepo) Delete(ctx context.Context, id string) error {
	_, err := r.deps.Exec(ctx, `DELETE FROM features WHERE id = $1`, id)
	return wrapDeleteError(err, "feature")
}

func (r *FeatureRepo) SaveAudit(ctx context.Context, entry *domainFeature.AuditEntry) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO feature_audit (id, feature_id, action, field, old_value, new_value, changed_by, timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.ID, entry.FeatureID, entry.Action, entry.Field,
		entry.OldValue, entry.NewValue, entry.ChangedBy, entry.Timestamp,
	)
	return wrapSaveError(err, "feature audit")
}
