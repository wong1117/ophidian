package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type FindingRepository struct {
	pool *pgxpool.Pool
}

func NewFindingRepository(pool *pgxpool.Pool) *FindingRepository {
	return &FindingRepository{pool: pool}
}

func (r *FindingRepository) Save(ctx context.Context, f *finding.Finding) error {
	return nil
}

func (r *FindingRepository) FindByID(ctx context.Context, id string) (*finding.Finding, error) {
	return nil, nil
}

func (r *FindingRepository) FindByMission(ctx context.Context, missionID string) ([]*finding.Finding, error) {
	return nil, nil
}

func (r *FindingRepository) FindByTarget(ctx context.Context, targetID string) ([]*finding.Finding, error) {
	return nil, nil
}

func (r *FindingRepository) Update(ctx context.Context, f *finding.Finding) error {
	return nil
}

func (r *FindingRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *FindingRepository) SaveEvidence(ctx context.Context, ev *finding.Evidence) error {
	return nil
}

func (r *FindingRepository) FindEvidenceByFinding(ctx context.Context, findingID string) ([]*finding.Evidence, error) {
	return nil, nil
}
