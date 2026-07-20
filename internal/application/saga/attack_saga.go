package saga

import "github.com/ophidian/ophidian/internal/domain/common"

func NewAttackSagaOrchestrator(missionID string) *AttackSaga {
	saga := NewAttackSaga(missionID)

	saga.AddStep(SagaStep{
		Name:   "recon_passive",
		Plane:  PlaneExecution,
		Action: func() error { return nil },
		Compensate: func() error { return nil },
	})

	saga.AddStep(SagaStep{
		Name:      "ai_plan",
		Plane:     PlaneAI,
		Action:    func() error { return nil },
		Compensate: func() error { return nil },
		DependsOn: []string{"recon_passive"},
	})

	saga.AddStep(SagaStep{
		Name:      "exploit_execute",
		Plane:     PlaneExecution,
		Action:    func() error { return nil },
		Compensate: func() error { return nil },
		DependsOn: []string{"ai_plan"},
	})

	saga.AddStep(SagaStep{
		Name:      "post_exploit",
		Plane:     PlaneExecution,
		Action:    func() error { return nil },
		Compensate: func() error { return nil },
		DependsOn: []string{"exploit_execute"},
	})

	saga.AddStep(SagaStep{
		Name:      "report_generate",
		Plane:     PlaneExecution,
		Action:    func() error { return nil },
		Compensate: func() error { return nil },
		DependsOn: []string{"post_exploit"},
	})

	return saga
}
