package policy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainPolicy "github.com/ophidian/ophidian/internal/domain/policy"
)

type EventStore interface {
	Append(ctx context.Context, event interface{}) error
}

type PolicyService struct {
	repo       domainPolicy.Repository
	eventStore EventStore
}

func NewPolicyService(repo domainPolicy.Repository, eventStore EventStore) *PolicyService {
	return &PolicyService{repo: repo, eventStore: eventStore}
}

func (s *PolicyService) Create(ctx context.Context, name, description string, rules []domainPolicy.Rule, defaultDecision domainPolicy.Decision, priority int) (*domainPolicy.Policy, error) {
	if name == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("policy must have at least one rule")
	}
	if defaultDecision == "" {
		defaultDecision = domainPolicy.DecisionDeny
	}

	p := &domainPolicy.Policy{
		ID:              common.NewID(),
		Name:            name,
		Description:     description,
		Version:         1,
		Enabled:         true,
		Priority:        priority,
		Rules:           rules,
		DefaultDecision: defaultDecision,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}
	return p, nil
}

func (s *PolicyService) Update(ctx context.Context, id, name, description string, rules []domainPolicy.Rule, enabled bool, priority int) (*domainPolicy.Policy, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}

	p.Name = name
	p.Description = description
	p.Rules = rules
	p.Enabled = enabled
	p.Priority = priority
	p.Version++
	p.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}
	return p, nil
}

func (s *PolicyService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *PolicyService) Evaluate(ctx context.Context, evalCtx *domainPolicy.EvaluationContext) (*domainPolicy.EvaluationResult, error) {
	policies, err := s.repo.FindEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})

	for _, p := range policies {
		for ruleIdx, rule := range p.Rules {
			if s.matchRule(rule, evalCtx) {
				result := &domainPolicy.EvaluationResult{
					Allowed:     rule.Decision == domainPolicy.DecisionAllow,
					Decision:    rule.Decision,
					MatchedRule: ruleIdx,
					Reason:      rule.Reason,
					PolicyID:    p.ID.String(),
					PolicyName:  p.Name,
				}
				s.emitEvent(ctx, p.ID, evalCtx, rule.Decision, rule.Reason)
				return result, nil
			}
		}
	}

	if len(policies) == 0 {
		result := &domainPolicy.EvaluationResult{
			Allowed:    true,
			Decision:   domainPolicy.DecisionAllow,
			Reason:     "no policies configured, default allow",
		}
		return result, nil
	}

	lastPolicy := policies[len(policies)-1]
	defaultDecision := lastPolicy.DefaultDecision

	result := &domainPolicy.EvaluationResult{
		Allowed:    defaultDecision == domainPolicy.DecisionAllow,
		Decision:   defaultDecision,
		Reason:     "no matching rule",
		PolicyID:   lastPolicy.ID.String(),
		PolicyName: lastPolicy.Name,
	}
	s.emitEvent(ctx, lastPolicy.ID, evalCtx, defaultDecision, "default decision")
	return result, nil
}

func (s *PolicyService) matchRule(rule domainPolicy.Rule, evalCtx *domainPolicy.EvaluationContext) bool {
	if rule.Condition.ResourceType != "" && rule.Condition.ResourceType != evalCtx.ResourceType {
		return false
	}
	if len(rule.Condition.Actions) > 0 && !containsString(rule.Condition.Actions, evalCtx.Action) {
		return false
	}
	for _, attr := range rule.Condition.Attributes {
		actual, ok := evalCtx.Attributes[attr.Key]
		if !ok {
			return false
		}
		if !matchOperator(actual, attr.Operator, attr.Value) {
			return false
		}
	}
	return true
}

func (s *PolicyService) GetPolicies(ctx context.Context) ([]*domainPolicy.Policy, error) {
	return s.repo.FindAll(ctx)
}

func (s *PolicyService) emitEvent(ctx context.Context, policyID common.ID, evalCtx *domainPolicy.EvaluationContext, decision domainPolicy.Decision, reason string) {
	if s.eventStore == nil {
		return
	}
	s.eventStore.Append(ctx, domainPolicy.PolicyEvaluated{
		PolicyID:     policyID,
		ResourceType: evalCtx.ResourceType,
		Action:       evalCtx.Action,
		Subject:      evalCtx.Subject,
		Decision:     decision,
		Reason:       reason,
		Timestamp:    time.Now(),
	})
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func matchOperator(actual, op, expected string) bool {
	switch op {
	case "eq":
		return strings.EqualFold(actual, expected)
	case "neq":
		return !strings.EqualFold(actual, expected)
	case "contains":
		return strings.Contains(strings.ToLower(actual), strings.ToLower(expected))
	case "prefix":
		return strings.HasPrefix(strings.ToLower(actual), strings.ToLower(expected))
	case "suffix":
		return strings.HasSuffix(strings.ToLower(actual), strings.ToLower(expected))
	default:
		return false
	}
}
