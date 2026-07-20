package report

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type IntelExporter interface {
	ExportSTIX(ctx context.Context, findings []finding.Finding) ([]byte, error)
	ExportMITRE(ctx context.Context, findings []finding.Finding) ([]byte, error)
	ExportCSV(ctx context.Context, findings []finding.Finding) ([]byte, error)
}

type ExportIntelUseCase struct {
	findingRepo finding.FindingRepository
	exporter    IntelExporter
}

func NewExportIntelUseCase(fr finding.FindingRepository, e IntelExporter) *ExportIntelUseCase {
	return &ExportIntelUseCase{findingRepo: fr, exporter: e}
}
