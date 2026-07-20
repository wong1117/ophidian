package controlplane

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

type HandleAIRecommendationUseCase struct {
	missionRepo mission.MissionRepository
	eventStore   EventStore
}

func NewHandleAIRecommendationUseCase(repo mission.MissionRepository, es EventStore) *HandleAIRecommendationUseCase {
	return &HandleAIRecommendationUseCase{missionRepo: repo, eventStore: es}
}

func (uc *HandleAIRecommendationUseCase) Execute(ctx context.Context, missionID string, rec mission.StrategyRecommendation) (common.PlanDecision, error) {
	m, err := uc.missionRepo.FindByID(ctx, missionID)
	if err != nil {
		return "", err
	}

	decision := common.PlanAccepted
	reason := "plan accepted"

	if rec.RiskLevel == common.RiskCritical && !m.RoE.AllowDestructive {
		decision = common.PlanRejected
		reason = "risk level exceeds RoE constraints"
	}

	event := mission.AIPlanDecision{
		MissionID: m.ID,
		PlanID:    rec.PlanID,
		Decision:  decision,
		Reason:    reason,
		DecidedAt: common.Now(),
	}

	return decision, uc.eventStore.Append(ctx, event)
}
