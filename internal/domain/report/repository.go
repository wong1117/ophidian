package report

import "context"

type ReportRepository interface {
	Save(ctx context.Context, r *Report) error
	FindByID(ctx context.Context, id string) (*Report, error)
	FindByMission(ctx context.Context, missionID string) ([]*Report, error)
	Delete(ctx context.Context, id string) error
}
