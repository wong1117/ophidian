package attackplan

import "github.com/ophidian/ophidian/internal/domain/common"

type Confidence float64

func (c Confidence) IsValid() bool {
	return c >= 0 && c <= 1.0
}

type RiskScore float64

func (r RiskScore) Level() common.RiskLevel {
	switch {
	case r >= 0.8:
		return common.RiskCritical
	case r >= 0.6:
		return common.RiskHigh
	case r >= 0.3:
		return common.RiskMedium
	default:
		return common.RiskLow
	}
}

type Path struct {
	Steps      []AttackStep
	TotalRisk  RiskScore
	Confidence Confidence
}

type AttackStep struct {
	TargetID   string
	Action     string
	Service    string
	CVE        string
	Confidence float64
	RiskLevel  common.RiskLevel
}

type Strategy struct {
	ID          common.ID
	Name        string
	Description string
	Steps       []AttackStep
	RiskProfile RiskScore
	Confidence  Confidence
}
