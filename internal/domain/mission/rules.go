package mission

import (
	"github.com/ophidian/ophidian/internal/domain/common"
)

func isValidTransition(from, to common.Phase) bool {
	transitions := map[common.Phase][]common.Phase{
		common.PhaseRecon:       {common.PhasePlanning, common.PhaseAborted},
		common.PhasePlanning:    {common.PhaseExploit, common.PhaseAborted},
		common.PhaseExploit:     {common.PhasePostExploit, common.PhaseAborted},
		common.PhasePostExploit: {common.PhaseReport, common.PhaseAborted},
		common.PhaseReport:      {common.PhaseComplete, common.PhaseAborted},
	}
	allowed, ok := transitions[from]
	if !ok {
		return false
	}
	for _, p := range allowed {
		if p == to {
			return true
		}
	}
	return false
}

func ValidateRoE(roe RoEConstraints, target Target) error {
	if roe.MaxTargets > 0 && len(target.IPs) > roe.MaxTargets {
		return common.ErrRoEViolation
	}
	return nil
}

type StrategyRecommendation struct {
	PlanID         common.ID
	SuggestedPath []AttackStep
	Rationale      string
	Confidence     float64
	RiskLevel      common.RiskLevel
}

type AttackStep struct {
	TargetID    common.ID
	Action      string
	Service     string
	CVE         string
	Confidence  float64
	RiskLevel   common.RiskLevel
}
