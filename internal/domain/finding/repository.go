package finding

import "context"

type FindingRepository interface {
	Save(ctx context.Context, finding *Finding) error
	FindByID(ctx context.Context, id string) (*Finding, error)
	FindByMission(ctx context.Context, missionID string) ([]*Finding, error)
	FindByTarget(ctx context.Context, targetID string) ([]*Finding, error)
	Update(ctx context.Context, finding *Finding) error
	Delete(ctx context.Context, id string) error
	SaveEvidence(ctx context.Context, evidence *Evidence) error
	FindEvidenceByFinding(ctx context.Context, findingID string) ([]*Evidence, error)
}
