package rbac

import (
	"context"
	"fmt"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainRBAC "github.com/ophidian/ophidian/internal/domain/rbac"
)

type PolicyService struct {
	userRepo       domainRBAC.UserRepository
	roleRepo       domainRBAC.RoleRepository
	permissionRepo domainRBAC.PermissionRepository
	auditLogger    domainRBAC.AuditLogger
}

func NewPolicyService(
	userRepo domainRBAC.UserRepository,
	roleRepo domainRBAC.RoleRepository,
	permissionRepo domainRBAC.PermissionRepository,
	auditLogger domainRBAC.AuditLogger,
) *PolicyService {
	return &PolicyService{
		userRepo:       userRepo,
		roleRepo:       roleRepo,
		permissionRepo: permissionRepo,
		auditLogger:    auditLogger,
	}
}

func (s *PolicyService) Evaluate(ctx context.Context, user *domainRBAC.User, resource, action string) (bool, error) {
	if user == nil {
		return false, nil
	}
	if !user.Active {
		s.audit(ctx, user, resource, action, false, "user is inactive")
		return false, nil
	}
	for _, roleName := range user.Roles {
		role, err := s.roleRepo.FindByName(ctx, roleName)
		if err != nil {
			continue
		}
		for _, permID := range role.Permissions {
			perm, err := s.permissionRepo.FindByID(ctx, permID)
			if err != nil {
				continue
			}
			if matchPermission(perm, resource, action) {
				s.audit(ctx, user, resource, action, true,
					fmt.Sprintf("granted via role %s, permission %s", roleName, perm.Name))
				return true, nil
			}
		}
	}
	s.audit(ctx, user, resource, action, false, "no matching permission found")
	return false, nil
}

func (s *PolicyService) HasRole(user *domainRBAC.User, roleName string) bool {
	for _, r := range user.Roles {
		if r == roleName {
			return true
		}
	}
	return false
}

func (s *PolicyService) GrantRole(ctx context.Context, userID, roleName string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	for _, r := range user.Roles {
		if r == roleName {
			return nil
		}
	}
	user.Roles = append(user.Roles, roleName)
	return s.userRepo.Update(ctx, user)
}

func (s *PolicyService) RevokeRole(ctx context.Context, userID, roleName string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		if r != roleName {
			filtered = append(filtered, r)
		}
	}
	user.Roles = filtered
	return s.userRepo.Update(ctx, user)
}

func (s *PolicyService) HasPermission(ctx context.Context, user *domainRBAC.User, resource, action string) (bool, error) {
	return s.Evaluate(ctx, user, resource, action)
}

func (s *PolicyService) audit(ctx context.Context, user *domainRBAC.User, resource, action string, granted bool, reason string) {
	if s.auditLogger == nil {
		return
	}
	_ = s.auditLogger.Log(ctx, &domainRBAC.AuditEntry{
		ID:        common.NewID(),
		UserID:    user.ID.String(),
		Username:  user.Username,
		Resource:  resource,
		Action:    action,
		Granted:   granted,
		Reason:    reason,
		Timestamp: common.Now().Time,
	})
}

func matchPermission(perm *domainRBAC.Permission, resource, action string) bool {
	if perm.Resource != "*" && perm.Resource != resource {
		return false
	}
	if perm.Action != "*" && perm.Action != action {
		return false
	}
	return true
}
