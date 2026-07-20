package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/rbac"
)

type UserRepo struct {
	deps RepoDeps
}

func NewRbacUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{deps: repoDepsFromPool(pool)}
}

func NewRbacUserRepoWithDeps(deps RepoDeps) *UserRepo {
	return &UserRepo{deps: deps}
}

func (r *UserRepo) Save(ctx context.Context, user *rbac.User) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO rbac_users (id, username, email, roles, active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		user.ID, user.Username, user.Email, marshalJSON(user.Roles), user.Active,
		user.CreatedAt, user.UpdatedAt,
	)
	return wrapSaveError(err, "rbac user")
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*rbac.User, error) {
	var u rbac.User
	var rolesJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, username, email, roles, active, created_at, updated_at
		 FROM rbac_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &rolesJSON, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: user %s not found", err, id)
		}
		return nil, fmt.Errorf("find rbac user: %w", err)
	}
	unmarshalJSON(rolesJSON, &u.Roles)
	return &u, nil
}

func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*rbac.User, error) {
	var u rbac.User
	var rolesJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, username, email, roles, active, created_at, updated_at
		 FROM rbac_users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &rolesJSON, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", username)
		}
		return nil, fmt.Errorf("find rbac user by username: %w", err)
	}
	unmarshalJSON(rolesJSON, &u.Roles)
	return &u, nil
}

func (r *UserRepo) Update(ctx context.Context, user *rbac.User) error {
	_, err := r.deps.Exec(ctx,
		`UPDATE rbac_users SET email = $1, roles = $2, active = $3, updated_at = $4 WHERE id = $5`,
		user.Email, marshalJSON(user.Roles), user.Active, user.UpdatedAt, user.ID,
	)
	return wrapUpdateError(err, "rbac user")
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	_, err := r.deps.Exec(ctx, `DELETE FROM rbac_users WHERE id = $1`, id)
	return wrapDeleteError(err, "rbac user")
}

type RoleRepo struct {
	deps RepoDeps
}

func NewRbacRoleRepo(pool *pgxpool.Pool) *RoleRepo {
	return &RoleRepo{deps: repoDepsFromPool(pool)}
}

func NewRbacRoleRepoWithDeps(deps RepoDeps) *RoleRepo {
	return &RoleRepo{deps: deps}
}

func (r *RoleRepo) Save(ctx context.Context, role *rbac.Role) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO rbac_roles (id, name, description, permissions, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		role.ID, role.Name, role.Description, marshalJSON(role.Permissions),
		role.CreatedAt, role.UpdatedAt,
	)
	return wrapSaveError(err, "rbac role")
}

func (r *RoleRepo) FindByID(ctx context.Context, id string) (*rbac.Role, error) {
	var role rbac.Role
	var permsJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, name, description, permissions, created_at, updated_at
		 FROM rbac_roles WHERE id = $1`, id,
	).Scan(&role.ID, &role.Name, &role.Description, &permsJSON, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("role not found: %s", id)
		}
		return nil, fmt.Errorf("find rbac role: %w", err)
	}
	unmarshalJSON(permsJSON, &role.Permissions)
	return &role, nil
}

func (r *RoleRepo) FindByName(ctx context.Context, name string) (*rbac.Role, error) {
	var role rbac.Role
	var permsJSON []byte
	err := r.deps.QueryRow(ctx,
		`SELECT id, name, description, permissions, created_at, updated_at
		 FROM rbac_roles WHERE name = $1`, name,
	).Scan(&role.ID, &role.Name, &role.Description, &permsJSON, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("role not found: %s", name)
		}
		return nil, fmt.Errorf("find rbac role by name: %w", err)
	}
	unmarshalJSON(permsJSON, &role.Permissions)
	return &role, nil
}

func (r *RoleRepo) FindAll(ctx context.Context) ([]*rbac.Role, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, name, description, permissions, created_at, updated_at
		 FROM rbac_roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("find all rbac roles: %w", err)
	}
	defer rows.Close()

	var roles []*rbac.Role
	for rows.Next() {
		var role rbac.Role
		var permsJSON []byte
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &permsJSON,
			&role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, fmt.Errorf("find all rbac roles scan: %w", err)
		}
		unmarshalJSON(permsJSON, &role.Permissions)
		roles = append(roles, &role)
	}
	return roles, rows.Err()
}

func (r *RoleRepo) Update(ctx context.Context, role *rbac.Role) error {
	_, err := r.deps.Exec(ctx,
		`UPDATE rbac_roles SET name = $1, description = $2, permissions = $3, updated_at = $4
		 WHERE id = $5`,
		role.Name, role.Description, marshalJSON(role.Permissions), role.UpdatedAt, role.ID,
	)
	return wrapUpdateError(err, "rbac role")
}

func (r *RoleRepo) Delete(ctx context.Context, id string) error {
	_, err := r.deps.Exec(ctx, `DELETE FROM rbac_roles WHERE id = $1`, id)
	return wrapDeleteError(err, "rbac role")
}

type PermissionRepo struct {
	deps RepoDeps
}

func NewRbacPermissionRepo(pool *pgxpool.Pool) *PermissionRepo {
	return &PermissionRepo{deps: repoDepsFromPool(pool)}
}

func NewRbacPermissionRepoWithDeps(deps RepoDeps) *PermissionRepo {
	return &PermissionRepo{deps: deps}
}

func (r *PermissionRepo) Save(ctx context.Context, perm *rbac.Permission) error {
	_, err := r.deps.Exec(ctx,
		`INSERT INTO rbac_permissions (id, name, resource, action, description)
		 VALUES ($1, $2, $3, $4, $5)`,
		perm.ID, perm.Name, perm.Resource, perm.Action, perm.Description,
	)
	return wrapSaveError(err, "rbac permission")
}

func (r *PermissionRepo) FindByID(ctx context.Context, id string) (*rbac.Permission, error) {
	var p rbac.Permission
	err := r.deps.QueryRow(ctx,
		`SELECT id, name, resource, action, description
		 FROM rbac_permissions WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.Description)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("permission not found: %s", id)
		}
		return nil, fmt.Errorf("find rbac permission: %w", err)
	}
	return &p, nil
}

func (r *PermissionRepo) FindAll(ctx context.Context) ([]*rbac.Permission, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, name, resource, action, description
		 FROM rbac_permissions ORDER BY resource, action`)
	if err != nil {
		return nil, fmt.Errorf("find all rbac permissions: %w", err)
	}
	defer rows.Close()

	var perms []*rbac.Permission
	for rows.Next() {
		var p rbac.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, fmt.Errorf("find all rbac permissions scan: %w", err)
		}
		perms = append(perms, &p)
	}
	return perms, rows.Err()
}

func (r *PermissionRepo) FindByResource(ctx context.Context, resource string) ([]*rbac.Permission, error) {
	rows, err := r.deps.Query(ctx,
		`SELECT id, name, resource, action, description
		 FROM rbac_permissions WHERE resource = $1 OR resource = '*'`, resource)
	if err != nil {
		return nil, fmt.Errorf("find rbac permissions by resource: %w", err)
	}
	defer rows.Close()

	var perms []*rbac.Permission
	for rows.Next() {
		var p rbac.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, fmt.Errorf("find rbac permissions scan: %w", err)
		}
		perms = append(perms, &p)
	}
	return perms, rows.Err()
}
