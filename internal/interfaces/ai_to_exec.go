package interfaces

import (
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type StrategyRecommendation struct {
	PlanID         string
	SuggestedPath  []attackplan.AttackStep
	Rationale      string
	Confidence     float64
	RiskLevel      common.RiskLevel
}
