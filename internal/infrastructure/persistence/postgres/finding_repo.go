package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type FindingRepository struct {
	deps RepoDeps
}

func NewFindingRepository(pool *pgxpool.Pool) *FindingRepository {
	return &FindingRepository{deps: repoDepsFromPool(pool)}
}

func NewFindingRepositoryWithDeps(deps RepoDeps) *FindingRepository {
	return &FindingRepository{deps: deps}
}

func (r *FindingRepository) Save(ctx context.Context, f *finding.Finding) error {
	evidenceJSON := marshalJSON(f.Evidence)

	_, err := r.deps.Exec(ctx,
		`INSERT INTO findings (id, mission_id, target_id, title, description, severity, cvss,
		 cwe, cve, confidence, status, evidence, created_at, updated_at, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, $14, 1)`,
		f.ID, f.MissionID, f.TargetID, f.Title, f.Description, f.Severity,
		f.CVSS, f.CWE, f.CVE, f.Confidence, f.Status, evidenceJSON,
		f.CreatedAt, f.UpdatedAt,
	)
	return wrapSaveError(err, "finding")
}

func (r *FindingRepository) FindByID(ctx context.Context, id string) (*finding.Finding, error) {
	var f finding.Finding
	var evidenceJSON []byte

	err := r.deps.QueryRow(ctx,
		`SELECT id, mission_id, target_id, title, description, severity, cvss,
		 cwe, cve, confidence, status, evidence, created_at, updated_at
		 FROM findings WHERE id = $1`, id,
	).Scan(&f.ID, &f.MissionID, &f.TargetID, &f.Title, &f.Description, &f.Severity,
		&f.CVSS, &f.CWE, &f.CVE, &f.Confidence, &f.Status, &evidenceJSON,
		&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: finding %s not found", common.ErrMissionNotFound, id)
		}
		return nil, fmt.Errorf("find finding by id: %w", err)
	}

	if len(evidenceJSON) > 0 && string(evidenceJSON) != "null" {
		if err := unmarshalJSON(evidenceJSON, &f.Evidence); err != nil {
			return nil, fmt.Errorf("find finding: unmarshal evidence: %w", err)
		}
	}

	return &f, nil
}

func (r *FindingRepository) FindByMission(ctx context.Context, missionID string) ([]*finding.Finding, error) {
	return r.findFindings(ctx,
		`SELECT id, mission_id, target_id, title, description, severity, cvss,
		 cwe, cve, confidence, status, evidence, created_at, updated_at
		 FROM findings WHERE mission_id = $1 ORDER BY cvss DESC, created_at DESC`,
		missionID)
}

func (r *FindingRepository) FindByTarget(ctx context.Context, targetID string) ([]*finding.Finding, error) {
	return r.findFindings(ctx,
		`SELECT id, mission_id, target_id, title, description, severity, cvss,
		 cwe, cve, confidence, status, evidence, created_at, updated_at
		 FROM findings WHERE target_id = $1 ORDER BY cvss DESC, created_at DESC`,
		targetID)
}

func (r *FindingRepository) findFindings(ctx context.Context, query string, args ...interface{}) ([]*finding.Finding, error) {
	rows, err := r.deps.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find findings: %w", err)
	}
	defer rows.Close()

	var findings []*finding.Finding
	for rows.Next() {
		var f finding.Finding
		var evidenceJSON []byte
		if err := rows.Scan(&f.ID, &f.MissionID, &f.TargetID, &f.Title, &f.Description,
			&f.Severity, &f.CVSS, &f.CWE, &f.CVE, &f.Confidence, &f.Status,
			&evidenceJSON, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("find findings: scan: %w", err)
		}
		if len(evidenceJSON) > 0 && string(evidenceJSON) != "null" {
			json.Unmarshal(evidenceJSON, &f.Evidence)
		}
		findings = append(findings, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find findings: rows: %w", err)
	}

	return findings, nil
}

func (r *FindingRepository) Update(ctx context.Context, f *finding.Finding) error {
	evidenceJSON := marshalJSON(f.Evidence)

	tag, err := r.deps.Exec(ctx,
		`UPDATE findings SET title = $1, description = $2, severity = $3, cvss = $4,
		 cwe = $5, cve = $6, confidence = $7, status = $8, evidence = $9::jsonb,
		 updated_at = $10, version = version + 1
		 WHERE id = $11`,
		f.Title, f.Description, f.Severity, f.CVSS, f.CWE, f.CVE,
		f.Confidence, f.Status, evidenceJSON, f.UpdatedAt, f.ID,
	)
	if err != nil {
		return wrapUpdateError(err, "finding")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: finding %s not found for update", common.ErrMissionNotFound, f.ID)
	}
	return nil
}

func (r *FindingRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.deps.Exec(ctx, `DELETE FROM findings WHERE id = $1`, id)
	if err != nil {
		return wrapDeleteError(err, "finding")
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: finding %s not found", common.ErrMissionNotFound, id)
	}
	return nil
}

func (r *FindingRepository) SaveEvidence(ctx context.Context, ev *finding.Evidence) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO finding_evidence (id, finding_id, type, data, source, timestamp, hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		ev.ID, ev.FindingID, ev.Type, ev.Data, ev.Source, ev.Timestamp, ev.Hash,
	)
	return wrapSaveError(err, "evidence")
}

func (r *FindingRepository) FindEvidenceByFinding(ctx context.Context, findingID string) ([]*finding.Evidence, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, finding_id, type, data, source, timestamp, hash
		 FROM finding_evidence WHERE finding_id = $1 ORDER BY timestamp ASC`, findingID,
	)
	if err != nil {
		return nil, fmt.Errorf("find evidence by finding: %w", err)
	}
	defer rows.Close()

	var evidence []*finding.Evidence
	for rows.Next() {
		var ev finding.Evidence
		if err := rows.Scan(&ev.ID, &ev.FindingID, &ev.Type, &ev.Data, &ev.Source,
			&ev.Timestamp, &ev.Hash); err != nil {
			return nil, fmt.Errorf("find evidence by finding: scan: %w", err)
		}
		evidence = append(evidence, &ev)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find evidence by finding: rows: %w", err)
	}

	return evidence, nil
}
