package recommendation

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
)

type Category string

const (
	CategoryCritical Category = "CRITICAL"
	CategorySecurity Category = "SECURITY"
	CategoryCompliance Category = "COMPLIANCE"
	CategoryPerformance Category = "PERFORMANCE"
	CategoryBestPractice Category = "BEST_PRACTICE"
)

type Severity int

const (
	SeverityCritical Severity = 5
	SeverityHigh     Severity = 4
	SeverityMedium   Severity = 3
	SeverityLow      Severity = 2
	SeverityInfo     Severity = 1
)

type Recommendation struct {
	ID           common.ID
	Title        string
	Description  string
	Category     Category
	Confidence   float64
	Priority     int
	Severity     Severity
	SourceFindings []common.ID
	Remediation  string
	Mitigation   string
	References   []string
	Metadata     map[string]interface{}
	CreatedAt    time.Time
}

type AssessmentInput struct {
	Findings      []finding.Finding
	Environment   string
	AssetCriticality int
	ComplianceReqs  []string
}

type ScoringRule struct {
	Category      Category
	SeverityWeight    float64
	CVSSWeight        float64
	ConfidenceWeight  float64
	AssetWeight       float64
	ComplianceBonus   float64
}

type Repository interface {
	Save(ctx context.Context, rec *Recommendation) error
	FindByID(ctx context.Context, id string) (*Recommendation, error)
	FindByCategory(ctx context.Context, category Category) ([]*Recommendation, error)
	FindAll(ctx context.Context) ([]*Recommendation, error)
	Delete(ctx context.Context, id string) error
}

func DefaultScoringRule(cat Category) ScoringRule {
	switch cat {
	case CategoryCritical:
		return ScoringRule{cat, 3.0, 2.0, 1.5, 1.0, 10.0}
	case CategorySecurity:
		return ScoringRule{cat, 2.5, 2.0, 1.5, 0.8, 5.0}
	case CategoryCompliance:
		return ScoringRule{cat, 2.0, 1.5, 1.0, 0.5, 15.0}
	case CategoryPerformance:
		return ScoringRule{cat, 1.5, 1.0, 0.5, 0.5, 2.0}
	default:
		return ScoringRule{cat, 1.5, 1.0, 1.0, 0.5, 0}
	}
}

type RecommendationService struct {
	repo   Repository
	rules  map[Category]ScoringRule
}

func NewRecommendationService(repo Repository) *RecommendationService {
	return &RecommendationService{
		repo:  repo,
		rules: make(map[Category]ScoringRule),
	}
}

func (s *RecommendationService) SetRules(rules map[Category]ScoringRule) {
	s.rules = rules
}

func (s *RecommendationService) Generate(ctx context.Context, input *AssessmentInput) ([]*Recommendation, error) {
	var recommendations []*Recommendation

	criticalRecs := s.generateForCategory(ctx, input, CategoryCritical)
	recommendations = append(recommendations, criticalRecs...)

	securityRecs := s.generateForCategory(ctx, input, CategorySecurity)
	recommendations = append(recommendations, securityRecs...)

	complianceRecs := s.generateForCategory(ctx, input, CategoryCompliance)
	recommendations = append(recommendations, complianceRecs...)

	performanceRecs := s.generateForCategory(ctx, input, CategoryPerformance)
	recommendations = append(recommendations, performanceRecs...)

	practiceRecs := s.generateForCategory(ctx, input, CategoryBestPractice)
	recommendations = append(recommendations, practiceRecs...)

	scored := s.rank(recommendations, input)

	if s.repo != nil {
		for _, rec := range scored {
			if err := s.repo.Save(ctx, rec); err != nil {
				return scored, fmt.Errorf("generate recommendations: %w", err)
			}
		}
	}

	return scored, nil
}

func (s *RecommendationService) generateForCategory(ctx context.Context, input *AssessmentInput, cat Category) []*Recommendation {
	var recommendations []*Recommendation
	rule := s.getRule(cat)

	for _, f := range input.Findings {
		score := s.calculateScore(f, input, rule)
		confidence := s.calculateConfidence(f, rule)
		sev := mapSeverity(f.Severity)

		if score < 10 && cat == CategoryCritical {
			continue
		}

		rec := &Recommendation{
			ID:           common.NewID(),
			Title:        fmt.Sprintf("[%s] %s", cat, f.Title),
			Description:  f.Description,
			Category:     cat,
			Confidence:   confidence,
			Priority:     int(score),
			Severity:     sev,
			SourceFindings: []common.ID{f.ID},
			Remediation:  generateRemediation(sev, f.CVE),
			Mitigation:   generateMitigation(sev),
			References:   s.generateReferences(f),
			CreatedAt:    time.Now(),
		}

		recommendations = append(recommendations, rec)
	}

	return recommendations
}

func (s *RecommendationService) calculateScore(f finding.Finding, input *AssessmentInput, rule ScoringRule) float64 {
	score := 0.0
	score += float64(sevToInt(f.Severity)) * rule.SeverityWeight
	score += f.CVSS * rule.CVSSWeight
	score += confidenceToFloat(f.Confidence) * rule.ConfidenceWeight
	score += float64(input.AssetCriticality) * rule.AssetWeight * 0.5

	for _, req := range input.ComplianceReqs {
		_ = req
		score += rule.ComplianceBonus * 0.1
	}

	return score
}

func (s *RecommendationService) calculateConfidence(f finding.Finding, rule ScoringRule) float64 {
	base := 0.5
	base += f.CVSS / 10.0 * 0.3
	base += confidenceToFloat(f.Confidence) * 0.2
	return clamp(base, 0, 1)
}

func (s *RecommendationService) getRule(cat Category) ScoringRule {
	if rule, ok := s.rules[cat]; ok {
		return rule
	}
	return DefaultScoringRule(cat)
}

func (s *RecommendationService) rank(recs []*Recommendation, input *AssessmentInput) []*Recommendation {
	sort.Slice(recs, func(i, j int) bool {
		si := float64(recs[i].Priority) * recs[i].Confidence
		sj := float64(recs[j].Priority) * recs[j].Confidence
		if si != sj {
			return si > sj
		}
		return recs[i].Severity > recs[j].Severity
	})
	return recs
}

func (s *RecommendationService) generateReferences(f finding.Finding) []string {
	var refs []string
	if f.CVE != "" {
		refs = append(refs, "https://nvd.nist.gov/vuln/detail/"+f.CVE)
	}
	if f.CWE != "" {
		refs = append(refs, "https://cwe.mitre.org/data/definitions/"+f.CWE+".html")
	}
	return refs
}

func (s *RecommendationService) GetHistory(ctx context.Context, category Category) ([]*Recommendation, error) {
	if category == "" {
		return s.repo.FindAll(ctx)
	}
	return s.repo.FindByCategory(ctx, category)
}

func sevToInt(severity common.Severity) int {
	switch severity {
	case common.SeverityCritical: return int(SeverityCritical)
	case common.SeverityHigh: return int(SeverityHigh)
	case common.SeverityMedium: return int(SeverityMedium)
	case common.SeverityLow: return int(SeverityLow)
	default: return int(SeverityInfo)
	}
}

func mapSeverity(severity common.Severity) Severity {
	return Severity(sevToInt(severity))
}

func confidenceToFloat(c finding.ConfidenceLevel) float64 {
	switch c {
	case finding.ConfidenceConfirmed: return 1.0
	case finding.ConfidenceHigh: return 0.8
	case finding.ConfidenceMedium: return 0.5
	default: return 0.3
	}
}

func generateRemediation(sev Severity, cve string) string {
	if cve != "" {
		return fmt.Sprintf("Apply vendor patch for %s. Upgrade affected software to latest version.", cve)
	}
	switch sev {
	case SeverityCritical: return "Immediate remediation required. Escalate to security team."
	case SeverityHigh: return "Remediate within 7 days. Apply security patches."
	case SeverityMedium: return "Remediate within 30 days. Review configuration."
	default: return "Review and remediate as part of standard maintenance."
	}
}

func generateMitigation(sev Severity) string {
	switch sev {
	case SeverityCritical: return "Isolate affected systems. Apply WAF rules. Enable intrusion prevention."
	case SeverityHigh: return "Restrict network access. Enable monitoring and alerting."
	case SeverityMedium: return "Apply defense-in-depth controls. Review access controls."
	default: return "Monitor for changes. Document in risk register."
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}
