package interfaces

import (
	"github.com/ophidian/ophidian/internal/domain/attackplan"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type PlanRequest struct {
	MissionID  string
	TargetData attackplan.TargetProfile
	Constraints mission.RoEConstraints
	History    []mission.PastAttempt
}

type PlanResponse struct {
	PlanID      string
	AttackGraph attackplan.AttackGraph
	RankedPaths []attackplan.RankedPath
	Confidence  float64
	Rationale   string
	ETA         int
}
