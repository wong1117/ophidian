package policy

import (
	"context"
	"fmt"
	"sort"
	"testing"

	domainPolicy "github.com/ophidian/ophidian/internal/domain/policy"
	"github.com/stretchr/testify/assert"
)

type testRepo struct {
	policies map[string]*domainPolicy.Policy
}

func newTestRepo() *testRepo {
	return &testRepo{policies: make(map[string]*domainPolicy.Policy)}
}

func (r *testRepo) Save(ctx context.Context, p *domainPolicy.Policy) error {
	r.policies[p.ID.String()] = p
	return nil
}

func (r *testRepo) FindByID(ctx context.Context, id string) (*domainPolicy.Policy, error) {
	p, ok := r.policies[id]
	if !ok { return nil, fmt.Errorf("not found") }
	return p, nil
}

func (r *testRepo) FindByName(ctx context.Context, name string) (*domainPolicy.Policy, error) {
	for _, p := range r.policies {
		if p.Name == name { return p, nil }
	}
	return nil, fmt.Errorf("not found")
}

func (r *testRepo) FindAll(ctx context.Context) ([]*domainPolicy.Policy, error) {
	var result []*domainPolicy.Policy
	for _, p := range r.policies { result = append(result, p) }
	sort.Slice(result, func(i, j int) bool { return result[i].Priority > result[j].Priority })
	return result, nil
}

func (r *testRepo) FindEnabled(ctx context.Context) ([]*domainPolicy.Policy, error) {
	var result []*domainPolicy.Policy
	for _, p := range r.policies {
		if p.Enabled { result = append(result, p) }
	}
	return result, nil
}

func (r *testRepo) Update(ctx context.Context, p *domainPolicy.Policy) error {
	r.policies[p.ID.String()] = p
	return nil
}

func (r *testRepo) Delete(ctx context.Context, id string) error {
	delete(r.policies, id)
	return nil
}

type testEventStore struct {
	events []interface{}
}

func (s *testEventStore) Append(ctx context.Context, event interface{}) error {
	s.events = append(s.events, event)
	return nil
}

func TestPolicyService_Create(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	p, err := svc.Create(context.Background(), "security-policy", "Security rules",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{ResourceType: "exploit", Actions: []string{"execute"}}, Decision: domainPolicy.DecisionDeny, Reason: "No exploits allowed"},
		},
		domainPolicy.DecisionAllow, 10)

	assert.NoError(t, err)
	assert.Equal(t, "security-policy", p.Name)
	assert.True(t, p.Enabled)
	assert.Equal(t, 1, p.Version)
}

func TestPolicyService_Create_Validation(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	_, err := svc.Create(context.Background(), "", "desc", nil, "", 0)
	assert.Error(t, err)

	_, err = svc.Create(context.Background(), "name", "desc", nil, domainPolicy.DecisionDeny, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one rule")
}

func TestPolicyService_Evaluate_Allow(t *testing.T) {
	repo := newTestRepo()
	eventStore := &testEventStore{}
	svc := NewPolicyService(repo, eventStore)

	p, _ := svc.Create(context.Background(), "test", "desc",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{Actions: []string{"read"}}, Decision: domainPolicy.DecisionAllow, Reason: "allow reads"},
		},
		domainPolicy.DecisionDeny, 10)
	_ = p

	result, err := svc.Evaluate(context.Background(), &domainPolicy.EvaluationContext{
		ResourceType: "mission", Action: "read", Subject: "user1",
	})

	assert.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, domainPolicy.DecisionAllow, result.Decision)
	assert.NotEmpty(t, eventStore.events)
}

func TestPolicyService_Evaluate_Deny(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	svc.Create(context.Background(), "block-exploit", "desc",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{ResourceType: "exploit", Actions: []string{"execute"}}, Decision: domainPolicy.DecisionDeny, Reason: "blocked"},
		},
		domainPolicy.DecisionAllow, 10)

	result, err := svc.Evaluate(context.Background(), &domainPolicy.EvaluationContext{
		ResourceType: "exploit", Action: "execute", Subject: "user1",
	})

	assert.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, domainPolicy.DecisionDeny, result.Decision)
}

func TestPolicyService_Evaluate_NoMatch(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	svc.Create(context.Background(), "specific-policy", "desc",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{Actions: []string{"create"}}, Decision: domainPolicy.DecisionDeny, Reason: "no create"},
		},
		domainPolicy.DecisionAllow, 10)

	result, err := svc.Evaluate(context.Background(), &domainPolicy.EvaluationContext{
		ResourceType: "mission", Action: "read", Subject: "user1",
	})

	assert.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, domainPolicy.DecisionAllow, result.Decision)
}

func TestPolicyService_Evaluate_Attributes(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	svc.Create(context.Background(), "env-policy", "desc",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{
				Actions: []string{"deploy"},
				Attributes: []domainPolicy.AttributeCondition{
					{Key: "environment", Operator: "eq", Value: "production"},
				},
			}, Decision: domainPolicy.DecisionDeny, Reason: "no production deploy"},
		},
		domainPolicy.DecisionAllow, 10)

	result, _ := svc.Evaluate(context.Background(), &domainPolicy.EvaluationContext{
		Action: "deploy", Subject: "user1", Attributes: map[string]string{"environment": "production"},
	})
	assert.False(t, result.Allowed)

	result2, _ := svc.Evaluate(context.Background(), &domainPolicy.EvaluationContext{
		Action: "deploy", Subject: "user1", Attributes: map[string]string{"environment": "staging"},
	})
	assert.True(t, result2.Allowed)
}

func TestPolicyService_Evaluate_Priority(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	svc.Create(context.Background(), "high-priority", "desc",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{Actions: []string{"read"}}, Decision: domainPolicy.DecisionAllow, Reason: "allowed"},
		},
		domainPolicy.DecisionDeny, 100)

	svc.Create(context.Background(), "low-priority", "desc",
		[]domainPolicy.Rule{
			{Condition: domainPolicy.Condition{Actions: []string{"read"}}, Decision: domainPolicy.DecisionDeny, Reason: "denied"},
		},
		domainPolicy.DecisionDeny, 1)

	result, _ := svc.Evaluate(context.Background(), &domainPolicy.EvaluationContext{Action: "read", Subject: "user1"})
	assert.True(t, result.Allowed)
	assert.Equal(t, "high-priority", result.PolicyName)
}

func TestPolicyService_Update(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	p, _ := svc.Create(context.Background(), "test", "desc",
		[]domainPolicy.Rule{{Condition: domainPolicy.Condition{}, Decision: domainPolicy.DecisionDeny, Reason: "default"}},
		domainPolicy.DecisionAllow, 10)

	updated, err := svc.Update(context.Background(), p.ID.String(), "updated-name", "updated-desc",
		[]domainPolicy.Rule{{Condition: domainPolicy.Condition{}, Decision: domainPolicy.DecisionAllow, Reason: "updated"}},
		false, 20)

	assert.NoError(t, err)
	assert.Equal(t, "updated-name", updated.Name)
	assert.False(t, updated.Enabled)
	assert.Equal(t, 20, updated.Priority)
	assert.Equal(t, 2, updated.Version)
}

func TestPolicyService_FindEnabled(t *testing.T) {
	repo := newTestRepo()
	svc := NewPolicyService(repo, nil)

	svc.Create(context.Background(), "p1", "", []domainPolicy.Rule{{Condition: domainPolicy.Condition{}, Decision: domainPolicy.DecisionDeny, Reason: "default"}}, domainPolicy.DecisionDeny, 10)
	svc.Create(context.Background(), "p2", "", []domainPolicy.Rule{{Condition: domainPolicy.Condition{}, Decision: domainPolicy.DecisionDeny, Reason: "default"}}, domainPolicy.DecisionDeny, 5)
	p3, _ := svc.Create(context.Background(), "p3", "", []domainPolicy.Rule{{Condition: domainPolicy.Condition{}, Decision: domainPolicy.DecisionDeny, Reason: "default"}}, domainPolicy.DecisionDeny, 1)

	svc.Update(context.Background(), p3.ID.String(), "p3-disabled", "", []domainPolicy.Rule{{Condition: domainPolicy.Condition{}, Decision: domainPolicy.DecisionAllow, Reason: ""}}, false, 1)

	enabled, err := svc.GetPolicies(context.Background())
	assert.NoError(t, err)
	assert.Len(t, enabled, 3)
}

func TestMatchOperator(t *testing.T) {
	assert.True(t, matchOperator("hello", "eq", "hello"))
	assert.False(t, matchOperator("hello", "eq", "world"))
	assert.True(t, matchOperator("hello", "neq", "world"))
	assert.True(t, matchOperator("hello world", "contains", "world"))
	assert.True(t, matchOperator("prefix_abc", "prefix", "pre"))
	assert.True(t, matchOperator("abc_suffix", "suffix", "fix"))
	assert.False(t, matchOperator("abc", "unknown", "abc"))
}
