package tenant

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainTenant "github.com/ophidian/ophidian/internal/domain/tenant"
)

type TenantService struct {
	repo domainTenant.TenantRepository
}

func NewTenantService(repo domainTenant.TenantRepository) *TenantService {
	return &TenantService{repo: repo}
}

func (s *TenantService) Create(ctx context.Context, name, slug, plan string) (*domainTenant.Tenant, error) {
	if err := validateTenantInput(name, slug); err != nil {
		return nil, err
	}

	existing, _ := s.repo.FindBySlug(ctx, slug)
	if existing != nil {
		return nil, fmt.Errorf("tenant with slug %s already exists", slug)
	}

	t := &domainTenant.Tenant{
		ID:        common.NewID(),
		Name:      strings.TrimSpace(name),
		Slug:      strings.ToLower(strings.TrimSpace(slug)),
		Status:    domainTenant.StatusActive,
		Plan:      plan,
		Settings:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Save(ctx, t); err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	return t, nil
}

func (s *TenantService) Update(ctx context.Context, id, name, plan string) (*domainTenant.Tenant, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("update tenant: %w", err)
	}
	if t.Status == domainTenant.StatusInactive {
		return nil, fmt.Errorf("cannot update inactive tenant %s", id)
	}

	if name != "" {
		t.Name = strings.TrimSpace(name)
	}
	if plan != "" {
		t.Plan = plan
	}
	t.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("update tenant: %w", err)
	}

	return t, nil
}

func (s *TenantService) Delete(ctx context.Context, id string) error {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}
	if t.Status == domainTenant.StatusInactive {
		return fmt.Errorf("tenant %s is already inactive", id)
	}

	return s.repo.SoftDelete(ctx, id)
}

func (s *TenantService) AddProject(ctx context.Context, tenantID, projectID, name string) (*domainTenant.TenantProject, error) {
	_, err := s.repo.FindByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("add project: %w", err)
	}

	tp := &domainTenant.TenantProject{
		ID:        common.NewID(),
		TenantID:  common.ID(tenantID),
		ProjectID: common.ID(projectID),
		Name:      name,
		CreatedAt: time.Now(),
	}

	if err := s.repo.SaveProject(ctx, tp); err != nil {
		return nil, fmt.Errorf("add project: %w", err)
	}

	return tp, nil
}

func (s *TenantService) AddUser(ctx context.Context, tenantID, userID, role string) (*domainTenant.TenantUser, error) {
	_, err := s.repo.FindByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("add user: %w", err)
	}

	tu := &domainTenant.TenantUser{
		ID:        common.NewID(),
		TenantID:  common.ID(tenantID),
		UserID:    common.ID(userID),
		Role:      role,
		CreatedAt: time.Now(),
	}

	if err := s.repo.SaveUser(ctx, tu); err != nil {
		return nil, fmt.Errorf("add user: %w", err)
	}

	return tu, nil
}

func (s *TenantService) GetTenantsForUser(ctx context.Context, userID string) ([]*domainTenant.Tenant, error) {
	tenants, err := s.repo.FindTenantsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get tenants for user: %w", err)
	}
	return tenants, nil
}

func (s *TenantService) GetProjects(ctx context.Context, tenantID string) ([]*domainTenant.TenantProject, error) {
	projects, err := s.repo.FindProjectsByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get projects: %w", err)
	}
	return projects, nil
}

func (s *TenantService) GetUsers(ctx context.Context, tenantID string) ([]*domainTenant.TenantUser, error) {
	users, err := s.repo.FindUsersByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}
	return users, nil
}

func validateTenantInput(name, slug string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("tenant name is required")
	}
	if strings.TrimSpace(slug) == "" {
		return fmt.Errorf("tenant slug is required")
	}
	if strings.ContainsAny(slug, " /\\*?\"<>|") {
		return fmt.Errorf("tenant slug contains invalid characters: %s", slug)
	}
	return nil
}
