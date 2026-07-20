package report

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	domainReport "github.com/ophidian/ophidian/internal/domain/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockFindingRepo struct {
	mock.Mock
}

func (m *mockFindingRepo) Save(ctx context.Context, f *finding.Finding) error {
	args := m.Called(ctx, f)
	return args.Error(0)
}

func (m *mockFindingRepo) FindByID(ctx context.Context, id string) (*finding.Finding, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*finding.Finding), args.Error(1)
}

func (m *mockFindingRepo) FindByMission(ctx context.Context, missionID string) ([]*finding.Finding, error) {
	args := m.Called(ctx, missionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*finding.Finding), args.Error(1)
}

func (m *mockFindingRepo) FindByTarget(ctx context.Context, targetID string) ([]*finding.Finding, error) {
	args := m.Called(ctx, targetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*finding.Finding), args.Error(1)
}

func (m *mockFindingRepo) Update(ctx context.Context, f *finding.Finding) error {
	args := m.Called(ctx, f)
	return args.Error(0)
}

func (m *mockFindingRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockFindingRepo) SaveEvidence(ctx context.Context, ev *finding.Evidence) error {
	args := m.Called(ctx, ev)
	return args.Error(0)
}

func (m *mockFindingRepo) FindEvidenceByFinding(ctx context.Context, findingID string) ([]*finding.Evidence, error) {
	args := m.Called(ctx, findingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*finding.Evidence), args.Error(1)
}

type mockReportRepo struct {
	mock.Mock
}

func (m *mockReportRepo) Save(ctx context.Context, r *domainReport.Report) error {
	args := m.Called(ctx, r)
	return args.Error(0)
}

type mockEventStore struct {
	mock.Mock
}

func (m *mockEventStore) Append(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func makeFindings() []*finding.Finding {
	return []*finding.Finding{
		{
			ID:          common.NewID(),
			MissionID:   common.ID("mission-1"),
			TargetID:    common.ID("target-1"),
			Title:       "SQL Injection at login",
			Description: "Found SQLi on login page allowing authentication bypass",
			Severity:    common.SeverityCritical,
			CVSS:        9.8,
			CVE:         "CVE-2024-0001",
			CWE:         "CWE-89",
			Confidence:  finding.ConfidenceConfirmed,
			Status:      finding.FindingStatusConfirmed,
			CreatedAt:   common.Now(),
		},
		{
			ID:          common.NewID(),
			MissionID:   common.ID("mission-1"),
			TargetID:    common.ID("target-1"),
			Title:       "Outdated Apache Version",
			Description: "Apache 2.4.49 vulnerable to CVE-2024-0002",
			Severity:    common.SeverityHigh,
			CVSS:        7.5,
			CVE:         "CVE-2024-0002",
			CWE:         "CWE-1104",
			Confidence:  finding.ConfidenceHigh,
			Status:      finding.FindingStatusConfirmed,
			CreatedAt:   common.Now(),
		},
		{
			ID:          common.NewID(),
			MissionID:   common.ID("mission-1"),
			TargetID:    common.ID("target-2"),
			Title:       "Information disclosure",
			Description: "Server header reveals Apache version",
			Severity:    common.SeverityLow,
			CVSS:        2.5,
			Confidence:  finding.ConfidenceMedium,
			Status:      finding.FindingNew,
			CreatedAt:   common.Now(),
		},
	}
}

func TestGenerateReportUseCase_Execute_JSON_Success(t *testing.T) {
	ctx := context.Background()
	findings := makeFindings()

	mockFindings := new(mockFindingRepo)
	mockFindings.On("FindByMission", ctx, "mission-1").Return(findings, nil)

	mockRepo := new(mockReportRepo)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*report.Report")).Return(nil)

	mockEvt := new(mockEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("report.ReportGenerated")).Return(nil)

	uc := NewGenerateReportUseCase(mockFindings, mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, GenerateReportRequest{
		MissionID: "mission-1",
		Format:    "JSON",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Report)
	assert.Equal(t, "mission-1", resp.Report.MissionID)
	assert.Equal(t, "JSON", resp.Report.Format)
	assert.NotEmpty(t, resp.Report.Data)
	assert.Equal(t, 3, resp.Report.Summary.TotalFindings)
	assert.Equal(t, 1, resp.Report.Summary.CriticalCount)
	assert.Equal(t, 1, resp.Report.Summary.HighCount)
	assert.Equal(t, 1, resp.Report.Summary.LowCount)

	mockFindings.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockEvt.AssertExpectations(t)
}

func TestGenerateReportUseCase_Execute_Markdown_Success(t *testing.T) {
	ctx := context.Background()
	findings := makeFindings()

	mockFindings := new(mockFindingRepo)
	mockFindings.On("FindByMission", ctx, "mission-1").Return(findings, nil)

	mockRepo := new(mockReportRepo)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*report.Report")).Return(nil)

	mockEvt := new(mockEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("report.ReportGenerated")).Return(nil)

	uc := NewGenerateReportUseCase(mockFindings, mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, GenerateReportRequest{
		MissionID: "mission-1",
		Format:    "MARKDOWN",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "MARKDOWN", resp.Report.Format)
	assert.Contains(t, string(resp.Report.Data), "# Mission Report")
	assert.Contains(t, string(resp.Report.Data), "| Total Findings | 3 |")
	assert.Contains(t, string(resp.Report.Data), "CVE-2024-0001")
}

func TestGenerateReportUseCase_Execute_EmptyMissionID(t *testing.T) {
	uc := NewGenerateReportUseCase(nil, nil, nil)

	resp, err := uc.Execute(context.Background(), GenerateReportRequest{})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, common.ErrInvalidID)
}

func TestGenerateReportUseCase_Execute_DefaultToJSON(t *testing.T) {
	ctx := context.Background()
	findings := makeFindings()

	mockFindings := new(mockFindingRepo)
	mockFindings.On("FindByMission", ctx, "mission-1").Return(findings, nil)

	mockRepo := new(mockReportRepo)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*report.Report")).Return(nil)

	mockEvt := new(mockEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("report.ReportGenerated")).Return(nil)

	uc := NewGenerateReportUseCase(mockFindings, mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, GenerateReportRequest{
		MissionID: "mission-1",
		Format:    "INVALID",
	})

	assert.NoError(t, err)
	assert.Equal(t, "JSON", resp.Report.Format)
}

func TestGenerateReportUseCase_Execute_EmptyReport(t *testing.T) {
	ctx := context.Background()

	mockFindings := new(mockFindingRepo)
	mockFindings.On("FindByMission", ctx, "empty-mission").Return([]*finding.Finding{}, nil)

	mockRepo := new(mockReportRepo)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*report.Report")).Return(nil)

	mockEvt := new(mockEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("report.ReportGenerated")).Return(nil)

	uc := NewGenerateReportUseCase(mockFindings, mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, GenerateReportRequest{
		MissionID: "empty-mission",
		Format:    "JSON",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.Report.Summary.TotalFindings)
}

func TestGenerateReportUseCase_Execute_FindingsError(t *testing.T) {
	ctx := context.Background()

	mockFindings := new(mockFindingRepo)
	mockFindings.On("FindByMission", ctx, "bad-mission").Return(nil, errors.New("db error"))

	uc := NewGenerateReportUseCase(mockFindings, nil, nil)

	resp, err := uc.Execute(ctx, GenerateReportRequest{
		MissionID: "bad-mission",
		Format:    "JSON",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collect report data")
}

func TestGenerateReportUseCase_Execute_SaveError(t *testing.T) {
	ctx := context.Background()
	findings := makeFindings()

	mockFindings := new(mockFindingRepo)
	mockFindings.On("FindByMission", ctx, "mission-1").Return(findings, nil)

	mockRepo := new(mockReportRepo)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*report.Report")).Return(errors.New("save error"))

	uc := NewGenerateReportUseCase(mockFindings, mockRepo, nil)

	resp, err := uc.Execute(ctx, GenerateReportRequest{
		MissionID: "mission-1",
		Format:    "JSON",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save report")
}

func TestGenerateReportUseCase_Execute_EventError(t *testing.T) {
	ctx := context.Background()
	findings := makeFindings()

	mockFindings := new(mockFindingRepo)
	mockFindings.On("FindByMission", ctx, "mission-1").Return(findings, nil)

	mockRepo := new(mockReportRepo)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*report.Report")).Return(nil)

	mockEvt := new(mockEventStore)
	mockEvt.On("Append", ctx, mock.AnythingOfType("report.ReportGenerated")).Return(errors.New("event error"))

	uc := NewGenerateReportUseCase(mockFindings, mockRepo, mockEvt)

	resp, err := uc.Execute(ctx, GenerateReportRequest{
		MissionID: "mission-1",
		Format:    "JSON",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "append report event")
}

func TestJSONFormatter_Format(t *testing.T) {
	data := &ReportData{
		MissionID: "test",
		Targets:   []string{"10.0.0.1"},
		Findings:  []finding.Finding{},
		Timeline:  []TimelineEntry{},
		Summary:   ReportSummary{TotalFindings: 5, CriticalCount: 1},
	}

	fmt := &JSONFormatter{}
	result, err := fmt.Format(data)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	var parsed map[string]interface{}
	err = json.Unmarshal(result, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "test", parsed["mission_id"])
	assert.Equal(t, float64(5), parsed["summary"].(map[string]interface{})["TotalFindings"])
}

func TestMarkdownFormatter_Format(t *testing.T) {
	data := &ReportData{
		MissionID: "test",
		Targets:   []string{"10.0.0.1"},
		Findings: []finding.Finding{
			{
				Title:       "Test Finding",
				Description: "Test description",
				Severity:    common.SeverityHigh,
				CVSS:        7.5,
				CVE:         "CVE-2024-0001",
				Confidence:  finding.ConfidenceConfirmed,
			},
		},
		Timeline: []TimelineEntry{
			{Timestamp: 1000000, Event: "scan", Detail: "port scan completed"},
		},
		Summary: ReportSummary{TotalFindings: 1, HighCount: 1, SuccessRate: 1.0},
	}

	fmt := &MarkdownFormatter{}
	result, err := fmt.Format(data)

	assert.NoError(t, err)
	content := string(result)
	assert.Contains(t, content, "# Mission Report")
	assert.Contains(t, content, "**Generated:**")
	assert.Contains(t, content, "| Total Findings | 1 |")
	assert.Contains(t, content, "### Test Finding")
	assert.Contains(t, content, "CVE-2024-0001")
	assert.Contains(t, content, "port scan completed")
	assert.Contains(t, content, "Success Rate | 100.0%")
}

func TestBuildSummary(t *testing.T) {
	findings := []finding.Finding{
		{Severity: common.SeverityCritical},
		{Severity: common.SeverityHigh},
		{Severity: common.SeverityHigh},
		{Severity: common.SeverityMedium},
		{Severity: common.SeverityLow},
	}
	for i := range findings {
		findings[i].Status = finding.FindingStatusConfirmed
	}

	s := buildSummary(findings)

	assert.Equal(t, 5, s.TotalFindings)
	assert.Equal(t, 1, s.CriticalCount)
	assert.Equal(t, 2, s.HighCount)
	assert.Equal(t, 1, s.MediumCount)
	assert.Equal(t, 1, s.LowCount)
	assert.Equal(t, 1.0, s.SuccessRate)
}

func TestUniqueStrings(t *testing.T) {
	result := uniqueStrings([]string{"a", "b", "a", "", "c", "b"})
	assert.Equal(t, []string{"a", "b", "c"}, result)
}
