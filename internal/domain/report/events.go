package report

import "github.com/ophidian/ophidian/internal/domain/common"

type ReportGenerated struct {
	ReportID  common.ID
	MissionID common.ID
	Format    ReportFormat
	Timestamp common.UTCTime
}

func (e ReportGenerated) EventID() string            { return e.ReportID.String() }
func (e ReportGenerated) EventType() string          { return "ReportGenerated" }
func (e ReportGenerated) OccurredAt() common.UTCTime { return e.Timestamp }
func (e ReportGenerated) AggregateID() string        { return e.MissionID.String() }
func (e ReportGenerated) Version() int               { return 1 }
