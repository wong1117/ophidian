package rbac

import (
	"context"
	"errors"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	domainRBAC "github.com/ophidian/ophidian/internal/domain/rbac"
	"github.com/stretchr/testify/assert"
)

type testUserRepo struct {
	users map[string]*domainRBAC.User
}

func newTestUserRepo() *testUserRepo {
	return &testUserRepo{users: make(map[string]*domainRBAC.User)}
}

func (r *testUserRepo) Save(ctx context.Context, user *domainRBAC.User) error {
	r.users[user.ID.String()] = user
	return nil
}

func (r *testUserRepo) FindByID(ctx context.Context, id string) (*domainRBAC.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (r *testUserRepo) FindByUsername(ctx context.Context, username string) (*domainRBAC.User, error) {
	for _, u := range r.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *testUserRepo) Update(ctx context.Context, user *domainRBAC.User) error {
	r.users[user.ID.String()] = user
	return nil
}

func (r *testUserRepo) Delete(ctx context.Context, id string) error {
	delete(r.users, id)
	return nil
}

type testRoleRepo struct {
	roles map[string]*domainRBAC.Role
}

func newTestRoleRepo() *testRoleRepo {
	return &testRoleRepo{roles: make(map[string]*domainRBAC.Role)}
}

func (r *testRoleRepo) Save(ctx context.Context, role *domainRBAC.Role) error {
	r.roles[role.ID.String()] = role
	return nil
}

func (r *testRoleRepo) FindByID(ctx context.Context, id string) (*domainRBAC.Role, error) {
	rl, ok := r.roles[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return rl, nil
}

func (r *testRoleRepo) FindByName(ctx context.Context, name string) (*domainRBAC.Role, error) {
	for _, rl := range r.roles {
		if rl.Name == name {
			return rl, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *testRoleRepo) FindAll(ctx context.Context) ([]*domainRBAC.Role, error) {
	result := make([]*domainRBAC.Role, 0, len(r.roles))
	for _, rl := range r.roles {
		result = append(result, rl)
	}
	return result, nil
}

func (r *testRoleRepo) Update(ctx context.Context, role *domainRBAC.Role) error {
	r.roles[role.ID.String()] = role
	return nil
}

func (r *testRoleRepo) Delete(ctx context.Context, id string) error {
	delete(r.roles, id)
	return nil
}

type testPermissionRepo struct {
	perms map[string]*domainRBAC.Permission
}

func newTestPermissionRepo() *testPermissionRepo {
	return &testPermissionRepo{perms: make(map[string]*domainRBAC.Permission)}
}

func (r *testPermissionRepo) Save(ctx context.Context, perm *domainRBAC.Permission) error {
	r.perms[perm.ID.String()] = perm
	return nil
}

func (r *testPermissionRepo) FindByID(ctx context.Context, id string) (*domainRBAC.Permission, error) {
	p, ok := r.perms[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return p, nil
}

func (r *testPermissionRepo) FindAll(ctx context.Context) ([]*domainRBAC.Permission, error) {
	result := make([]*domainRBAC.Permission, 0, len(r.perms))
	for _, p := range r.perms {
		result = append(result, p)
	}
	return result, nil
}

func (r *testPermissionRepo) FindByResource(ctx context.Context, resource string) ([]*domainRBAC.Permission, error) {
	var result []*domainRBAC.Permission
	for _, p := range r.perms {
		if p.Resource == resource || p.Resource == "*" {
			result = append(result, p)
		}
	}
	return result, nil
}

type testAuditLogger struct {
	entries []*domainRBAC.AuditEntry
}

func (l *testAuditLogger) Log(ctx context.Context, entry *domainRBAC.AuditEntry) error {
	l.entries = append(l.entries, entry)
	return nil
}

func TestPolicyService_Evaluate_AdminAccess(t *testing.T) {
	usr := &domainRBAC.User{ID: common.NewID(), Username: "admin", Active: true, Roles: []string{"admin"}}
	role := &domainRBAC.Role{ID: common.NewID(), Name: "admin", Permissions: []string{"perm-all"}}
	perm := &domainRBAC.Permission{ID: common.ID("perm-all"), Name: "All Access", Resource: "*", Action: "*"}

	userRepo := newTestUserRepo()
	userRepo.users[usr.ID.String()] = usr
	roleRepo := newTestRoleRepo()
	roleRepo.roles[role.ID.String()] = role
	permRepo := newTestPermissionRepo()
	permRepo.perms[perm.ID.String()] = perm
	audit := &testAuditLogger{}

	svc := NewPolicyService(userRepo, roleRepo, permRepo, audit)
	granted, err := svc.Evaluate(context.Background(), usr, "missions", "create")

	assert.NoError(t, err)
	assert.True(t, granted)
}

func TestPolicyService_Evaluate_InactiveUser(t *testing.T) {
	usr := &domainRBAC.User{ID: common.NewID(), Username: "inactive", Active: false, Roles: []string{"admin"}}
	audit := &testAuditLogger{}
	svc := NewPolicyService(newTestUserRepo(), newTestRoleRepo(), newTestPermissionRepo(), audit)

	granted, _ := svc.Evaluate(context.Background(), usr, "missions", "create")
	assert.False(t, granted)
	assert.Len(t, audit.entries, 1)
}

func TestPolicyService_Evaluate_NoPermission(t *testing.T) {
	usr := &domainRBAC.User{ID: common.NewID(), Username: "user", Active: true, Roles: []string{"viewer"}}
	role := &domainRBAC.Role{ID: common.NewID(), Name: "viewer", Permissions: []string{"perm-read"}}
	perm := &domainRBAC.Permission{ID: common.ID("perm-read"), Name: "Read", Resource: "missions", Action: "read"}

	roleRepo := newTestRoleRepo()
	roleRepo.roles[role.ID.String()] = role
	permRepo := newTestPermissionRepo()
	permRepo.perms[perm.ID.String()] = perm

	svc := NewPolicyService(newTestUserRepo(), roleRepo, permRepo, &testAuditLogger{})
	granted, _ := svc.Evaluate(context.Background(), usr, "missions", "create")

	assert.False(t, granted)
}

func TestPolicyService_Evaluate_SpecificPermission(t *testing.T) {
	usr := &domainRBAC.User{ID: common.NewID(), Username: "op", Active: true, Roles: []string{"operator"}}
	role := &domainRBAC.Role{ID: common.NewID(), Name: "operator", Permissions: []string{"perm-exploit"}}
	perm := &domainRBAC.Permission{ID: common.ID("perm-exploit"), Name: "Exploit", Resource: "exploits", Action: "execute"}

	roleRepo := newTestRoleRepo()
	roleRepo.roles[role.ID.String()] = role
	permRepo := newTestPermissionRepo()
	permRepo.perms[perm.ID.String()] = perm
	audit := &testAuditLogger{}

	svc := NewPolicyService(newTestUserRepo(), roleRepo, permRepo, audit)
	granted, _ := svc.Evaluate(context.Background(), usr, "exploits", "execute")
	assert.True(t, granted)

	granted2, _ := svc.Evaluate(context.Background(), usr, "missions", "create")
	assert.False(t, granted2)
}

func TestPolicyService_GrantRole(t *testing.T) {
	usr := &domainRBAC.User{ID: common.NewID(), Username: "test", Active: true}
	userRepo := newTestUserRepo()
	userRepo.users[usr.ID.String()] = usr

	svc := NewPolicyService(userRepo, newTestRoleRepo(), newTestPermissionRepo(), nil)
	err := svc.GrantRole(context.Background(), usr.ID.String(), "operator")
	assert.NoError(t, err)

	updated, _ := userRepo.FindByID(context.Background(), usr.ID.String())
	assert.Contains(t, updated.Roles, "operator")
}

func TestPolicyService_RevokeRole(t *testing.T) {
	usr := &domainRBAC.User{ID: common.NewID(), Username: "test", Active: true, Roles: []string{"admin", "viewer"}}
	userRepo := newTestUserRepo()
	userRepo.users[usr.ID.String()] = usr

	svc := NewPolicyService(userRepo, newTestRoleRepo(), newTestPermissionRepo(), nil)
	err := svc.RevokeRole(context.Background(), usr.ID.String(), "viewer")
	assert.NoError(t, err)

	updated, _ := userRepo.FindByID(context.Background(), usr.ID.String())
	assert.NotContains(t, updated.Roles, "viewer")
	assert.Contains(t, updated.Roles, "admin")
}

func TestPolicyService_HasRole(t *testing.T) {
	usr := &domainRBAC.User{ID: common.NewID(), Username: "test", Active: true, Roles: []string{"admin"}}
	svc := NewPolicyService(nil, nil, nil, nil)
	assert.True(t, svc.HasRole(usr, "admin"))
	assert.False(t, svc.HasRole(usr, "operator"))
}

func TestMatchPermission(t *testing.T) {
	perm := &domainRBAC.Permission{Resource: "missions", Action: "create"}
	assert.True(t, matchPermission(perm, "missions", "create"))
	assert.False(t, matchPermission(perm, "missions", "delete"))
	assert.False(t, matchPermission(perm, "reports", "create"))

	wildcard := &domainRBAC.Permission{Resource: "*", Action: "*"}
	assert.True(t, matchPermission(wildcard, "any", "any"))
}
