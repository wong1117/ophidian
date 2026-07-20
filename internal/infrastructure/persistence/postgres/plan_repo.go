package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type AttackPlanRepository struct {
	deps RepoDeps
}

func NewAttackPlanRepository(pool *pgxpool.Pool) *AttackPlanRepository {
	return &AttackPlanRepository{deps: repoDepsFromPool(pool)}
}

func NewAttackPlanRepositoryWithDeps(deps RepoDeps) *AttackPlanRepository {
	return &AttackPlanRepository{deps: deps}
}

func (r *AttackPlanRepository) Save(ctx context.Context, p *attackplan.AttackPlan) error {
	graphJSON := marshalJSON(p.Graph)
	pathsJSON := marshalJSON(p.RankedPaths)

	_, err := r.deps.Exec(ctx,
		`INSERT INTO attack_plans (id, mission_id, graph, ranked_paths, confidence, rationale, eta, status, created_at, updated_at, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 1)`,
		p.ID, p.MissionID, graphJSON, pathsJSON, p.Confidence, p.Rationale,
		p.ETA, p.Status, p.CreatedAt, p.UpdatedAt,
	)
	return wrapSaveError(err, "attack plan")
}

func (r *AttackPlanRepository) FindByID(ctx context.Context, id string) (*attackplan.AttackPlan, error) {
	var p attackplan.AttackPlan
	var graphJSON, pathsJSON []byte

	err := r.deps.QueryRow(ctx,
		`SELECT id, mission_id, graph, ranked_paths, confidence, rationale, eta, status, created_at, updated_at
		 FROM attack_plans WHERE id = $1`, id,
	).Scan(&p.ID, &p.MissionID, &graphJSON, &pathsJSON, &p.Confidence, &p.Rationale,
		&p.ETA, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: attack plan %s not found", common.ErrPlanNotFound, id)
		}
		return nil, fmt.Errorf("find attack plan by id: %w", err)
	}

	if err := unmarshalJSON(graphJSON, &p.Graph); err != nil {
		return nil, fmt.Errorf("find attack plan: unmarshal graph: %w", err)
	}
	if err := unmarshalJSON(pathsJSON, &p.RankedPaths); err != nil {
		return nil, fmt.Errorf("find attack plan: unmarshal paths: %w", err)
	}

	return &p, nil
}

func (r *AttackPlanRepository) FindByMission(ctx context.Context, missionID string) ([]*attackplan.AttackPlan, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, mission_id, graph, ranked_paths, confidence, rationale, eta, status, created_at, updated_at
		 FROM attack_plans WHERE mission_id = $1 ORDER BY created_at DESC`, missionID,
	)
	if err != nil {
		return nil, fmt.Errorf("find plans by mission: %w", err)
	}
	defer rows.Close()

	var plans []*attackplan.AttackPlan
	for rows.Next() {
		var p attackplan.AttackPlan
		var graphJSON, pathsJSON []byte
		if err := rows.Scan(&p.ID, &p.MissionID, &graphJSON, &pathsJSON, &p.Confidence,
			&p.Rationale, &p.ETA, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("find plans by mission: scan: %w", err)
		}
		if err := unmarshalJSON(graphJSON, &p.Graph); err != nil {
			return nil, fmt.Errorf("find plans by mission: unmarshal graph: %w", err)
		}
		if err := unmarshalJSON(pathsJSON, &p.RankedPaths); err != nil {
			return nil, fmt.Errorf("find plans by mission: unmarshal paths: %w", err)
		}
		plans = append(plans, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find plans by mission: rows: %w", err)
	}

	return plans, nil
}

func (r *AttackPlanRepository) Update(ctx context.Context, p *attackplan.AttackPlan) error {
	graphJSON := marshalJSON(p.Graph)
	pathsJSON := marshalJSON(p.RankedPaths)

	tag, err := r.deps.Exec(ctx,
		`UPDATE attack_plans SET graph = $1, ranked_paths = $2, confidence = $3, rationale = $4,
		 eta = $5, status = $6, updated_at = $7, version = version + 1
		 WHERE id = $8`,
		graphJSON, pathsJSON, p.Confidence, p.Rationale,
		p.ETA, p.Status, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return wrapUpdateError(err, "attack plan")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: attack plan %s not found for update", common.ErrPlanNotFound, p.ID)
	}
	return nil
}

func (r *AttackPlanRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.deps.Exec(ctx, `DELETE FROM attack_plans WHERE id = $1`, id)
	if err != nil {
		return wrapDeleteError(err, "attack plan")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: attack plan %s not found", common.ErrPlanNotFound, id)
	}
	return nil
}
