package mission

import (
	"fmt"

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

	if roe.MaxTargets > 0 {
		totalTargets := len(target.IPs) + len(target.Domains)
		if totalTargets > roe.MaxTargets {
			return fmt.Errorf("%w: total targets (%d) exceeds max allowed (%d)", common.ErrRoEViolation, totalTargets, roe.MaxTargets)
		}
	}

	if !roe.TimeWindowStart.IsZero() && !roe.TimeWindowEnd.IsZero() {
		now := common.Now()
		if now.Before(roe.TimeWindowStart) || now.After(roe.TimeWindowEnd) {
			return fmt.Errorf("%w: current time is outside allowed window", common.ErrRoEViolation)
		}
	}

	for _, excludedIP := range roe.ExcludedNets {
		for _, targetIP := range target.IPs {
			if excludedIP == targetIP {
				return fmt.Errorf("%w: target ip %s is in excluded nets", common.ErrRoEViolation, targetIP)
			}
		}
	}

	return nil
}

var allowedLifecycleTransitions = map[MissionStatus][]MissionStatus{
	MissionCreated:  {MissionPlanning},
	MissionPlanning: {MissionReady},
	MissionReady:    {MissionRunning},
	MissionRunning:  {MissionCompleted, MissionFailed},
}

func isValidLifecycleTransition(from, to MissionStatus) bool {
	allowed, ok := allowedLifecycleTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

var missionLifecycleOrder = map[MissionStatus]int{
	MissionCreated:  1,
	MissionPlanning: 2,
	MissionReady:    3,
	MissionRunning:  4,
}

func isTerminal(status MissionStatus) bool {
	return status == MissionCompleted || status == MissionFailed || status == MissionAborted
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
