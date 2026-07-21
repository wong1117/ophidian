package saga

import (
	"context"
	"sync"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type SagaStatus string

const (
	SagaPending    SagaStatus = "PENDING"
	SagaRunning    SagaStatus = "RUNNING"
	SagaCompleted  SagaStatus = "COMPLETED"
	SagaFailed     SagaStatus = "FAILED"
	SagaCompensated SagaStatus = "COMPENSATED"
)

type PlaneType string

const (
	PlaneControl   PlaneType = "CONTROL"
	PlaneAI        PlaneType = "AI"
	PlaneExecution PlaneType = "EXECUTION"
)

type SagaStep struct {
	Name       string
	Plane      PlaneType
	Action     func() error
	Compensate func() error
	DependsOn  []string
	Status     SagaStatus
}

type AttackSaga struct {
	sagaID    string
	missionID string
	steps     []SagaStep
	status    SagaStatus
	mu        sync.RWMutex
}

func NewAttackSaga(missionID string) *AttackSaga {
	return &AttackSaga{
		sagaID:    string(common.NewID()),
		missionID: missionID,
		status:    SagaPending,
	}
}

func (s *AttackSaga) AddStep(step SagaStep) {
	s.steps = append(s.steps, step)
}

func (s *AttackSaga) Execute(ctx context.Context) error {
	s.mu.Lock()
	s.status = SagaRunning
	s.mu.Unlock()

	completed := []string{}

	for _, step := range s.steps {
		depsMet := true
		for _, dep := range step.DependsOn {
			found := false
			for _, c := range completed {
				if c == dep {
					found = true
					break
				}
			}
			if !found {
				depsMet = false
				break
			}
		}
		if !depsMet {
			continue
		}

		if err := step.Action(); err != nil {
			s.compensate(completed)
			return err
		}
		completed = append(completed, step.Name)
	}

	s.mu.Lock()
	s.status = SagaCompleted
	s.mu.Unlock()
	return nil
}

func (s *AttackSaga) compensate(completed []string) {
	s.mu.Lock()
	s.status = SagaFailed
	s.mu.Unlock()

	for i := len(completed) - 1; i >= 0; i-- {
		for _, step := range s.steps {
			if step.Name == completed[i] && step.Compensate != nil {
				_ = step.Compensate()
			}
		}
	}

	s.mu.Lock()
	s.status = SagaCompensated
	s.mu.Unlock()
}
