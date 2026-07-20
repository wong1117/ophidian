package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/target"
)

type TargetRepository struct {
	deps RepoDeps
}

func NewTargetRepository(pool *pgxpool.Pool) *TargetRepository {
	return &TargetRepository{deps: repoDepsFromPool(pool)}
}

func NewTargetRepositoryWithDeps(deps RepoDeps) *TargetRepository {
	return &TargetRepository{deps: deps}
}

func (r *TargetRepository) Save(ctx context.Context, t *target.Target) error {
	ipsJSON := marshalJSON(t.IPs)
	domainsJSON := marshalJSON(t.Domains)
	hostnamesJSON := marshalJSON(t.Hostnames)
	servicesJSON := marshalJSON(t.Services)
	tagsJSON := marshalJSON(t.Tags)

	_, err := r.deps.Exec(ctx,
		`INSERT INTO targets (id, ips, domains, hostnames, os, services, tags, created_at, updated_at, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)
		 ON CONFLICT (id) DO UPDATE SET
		 ips = $2, domains = $3, hostnames = $4, os = $5, services = $6, tags = $7,
		 updated_at = $9, version = version + 1`,
		t.ID, ipsJSON, domainsJSON, hostnamesJSON, t.OS, servicesJSON, tagsJSON,
		t.CreatedAt, t.UpdatedAt,
	)
	return wrapSaveError(err, "target")
}

func (r *TargetRepository) FindByID(ctx context.Context, id string) (*target.Target, error) {
	return r.scanTarget(ctx, `SELECT id, ips, domains, hostnames, os, services, tags, created_at, updated_at
		 FROM targets WHERE id = $1`, id)
}

func (r *TargetRepository) FindByIP(ctx context.Context, ip string) (*target.Target, error) {
	return r.scanTarget(ctx, `SELECT id, ips, domains, hostnames, os, services, tags, created_at, updated_at
		 FROM targets WHERE ips::text ILIKE $1 LIMIT 1`, "%"+ip+"%")
}

func (r *TargetRepository) FindByDomain(ctx context.Context, domain string) (*target.Target, error) {
	return r.scanTarget(ctx, `SELECT id, ips, domains, hostnames, os, services, tags, created_at, updated_at
		 FROM targets WHERE domains::text ILIKE $1 LIMIT 1`, "%"+domain+"%")
}

func (r *TargetRepository) scanTarget(ctx context.Context, query string, args ...interface{}) (*target.Target, error) {
	var t target.Target
	var ipsJSON, domainsJSON, hostnamesJSON, servicesJSON, tagsJSON []byte

	err := r.deps.QueryRow(ctx, query, args...).Scan(
		&t.ID, &ipsJSON, &domainsJSON, &hostnamesJSON, &t.OS,
		&servicesJSON, &tagsJSON, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: target not found", common.ErrMissionNotFound)
		}
		return nil, fmt.Errorf("find target: %w", err)
	}

	if err := unmarshalJSON(ipsJSON, &t.IPs); err != nil {
		return nil, fmt.Errorf("find target: unmarshal ips: %w", err)
	}
	if err := unmarshalJSON(domainsJSON, &t.Domains); err != nil {
		return nil, fmt.Errorf("find target: unmarshal domains: %w", err)
	}
	json.Unmarshal(hostnamesJSON, &t.Hostnames)
	json.Unmarshal(servicesJSON, &t.Services)
	json.Unmarshal(tagsJSON, &t.Tags)

	return &t, nil
}

func (r *TargetRepository) FindAll(ctx context.Context, filter target.TargetFilter) ([]*target.Target, error) {
	query := `SELECT id, ips, domains, hostnames, os, services, tags, created_at, updated_at
		 FROM targets WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if len(filter.Tags) > 0 {
		tagConditions := make([]string, len(filter.Tags))
		for i, tag := range filter.Tags {
			tagConditions[i] = fmt.Sprintf("tags::text ILIKE $%d", argIdx)
			args = append(args, "%"+tag+"%")
			argIdx++
		}
		query += " AND (" + strings.Join(tagConditions, " OR ") + ")"
	}
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
		argIdx++
	}

	return r.scanTargets(ctx, query, args...)
}

func (r *TargetRepository) scanTargets(ctx context.Context, query string, args ...interface{}) ([]*target.Target, error) {
	rows, err := r.deps.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find targets: %w", err)
	}
	defer rows.Close()

	var targets []*target.Target
	for rows.Next() {
		var t target.Target
		var ipsJSON, domainsJSON, hostnamesJSON, servicesJSON, tagsJSON []byte
		if err := rows.Scan(&t.ID, &ipsJSON, &domainsJSON, &hostnamesJSON, &t.OS,
			&servicesJSON, &tagsJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("find targets: scan: %w", err)
		}
		json.Unmarshal(ipsJSON, &t.IPs)
		json.Unmarshal(domainsJSON, &t.Domains)
		json.Unmarshal(hostnamesJSON, &t.Hostnames)
		json.Unmarshal(servicesJSON, &t.Services)
		json.Unmarshal(tagsJSON, &t.Tags)
		targets = append(targets, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find targets: rows: %w", err)
	}

	return targets, nil
}

func (r *TargetRepository) Update(ctx context.Context, t *target.Target) error {
	ipsJSON := marshalJSON(t.IPs)
	domainsJSON := marshalJSON(t.Domains)
	hostnamesJSON := marshalJSON(t.Hostnames)
	servicesJSON := marshalJSON(t.Services)
	tagsJSON := marshalJSON(t.Tags)

	tag, err := r.deps.Exec(ctx,
		`UPDATE targets SET ips = $1, domains = $2, hostnames = $3, os = $4, services = $5,
		 tags = $6, updated_at = $7, version = version + 1
		 WHERE id = $8`,
		ipsJSON, domainsJSON, hostnamesJSON, t.OS, servicesJSON, tagsJSON,
		t.UpdatedAt, t.ID,
	)
	if err != nil {
		return wrapUpdateError(err, "target")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: target %s not found for update", common.ErrMissionNotFound, t.ID)
	}
	return nil
}

func (r *TargetRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.deps.Exec(ctx, `DELETE FROM targets WHERE id = $1`, id)
	if err != nil {
		return wrapDeleteError(err, "target")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: target %s not found", common.ErrMissionNotFound, id)
	}
	return nil
}
