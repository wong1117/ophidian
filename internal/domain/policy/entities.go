package policy

import (
	"context"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type Decision string

const (
	DecisionAllow Decision = "ALLOW"
	DecisionDeny  Decision = "DENY"
)

type Policy struct {
	ID          common.ID
	Name        string
	Description string
	Version     int
	Enabled     bool
	Priority    int
	Rules       []Rule
	DefaultDecision Decision
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Rule struct {
	Condition  Condition
	Decision   Decision
	Reason     string
}

type Condition struct {
	ResourceType string
	Actions      []string
	Attributes   []AttributeCondition
}

type AttributeCondition struct {
	Key      string
	Operator string
	Value    string
}

type EvaluationContext struct {
	ResourceType string
	Action       string
	Subject      string
	Attributes   map[string]string
	TenantID     string
}

type EvaluationResult struct {
	Allowed      bool
	Decision     Decision
	MatchedRule  int
	Reason       string
	PolicyID     string
	PolicyName   string
}

type Repository interface {
	Save(ctx context.Context, p *Policy) error
	FindByID(ctx context.Context, id string) (*Policy, error)
	FindByName(ctx context.Context, name string) (*Policy, error)
	FindAll(ctx context.Context) ([]*Policy, error)
	FindEnabled(ctx context.Context) ([]*Policy, error)
	Update(ctx context.Context, p *Policy) error
	Delete(ctx context.Context, id string) error
}

type PolicyEvaluated struct {
	PolicyID    common.ID
	ResourceType string
	Action      string
	Subject     string
	Decision    Decision
	Reason      string
	Timestamp   time.Time
}

func (e PolicyEvaluated) EventID() string            { return e.PolicyID.String() }
func (e PolicyEvaluated) EventType() string          { return "PolicyEvaluated" }
func (e PolicyEvaluated) OccurredAt() common.UTCTime { return common.UTCTime{Time: e.Timestamp} }
func (e PolicyEvaluated) AggregateID() string        { return e.PolicyID.String() }
func (e PolicyEvaluated) Version() int               { return 1 }
