package rbac

import (
	"context"
	"time"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type User struct {
	ID        common.ID
	Username  string
	Email     string
	Roles     []string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Role struct {
	ID          common.ID
	Name        string
	Description string
	Permissions []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Permission struct {
	ID          common.ID
	Name        string
	Resource    string
	Action      string
	Description string
}

type UserRepository interface {
	Save(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
}

type RoleRepository interface {
	Save(ctx context.Context, role *Role) error
	FindByID(ctx context.Context, id string) (*Role, error)
	FindByName(ctx context.Context, name string) (*Role, error)
	FindAll(ctx context.Context) ([]*Role, error)
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, id string) error
}

type PermissionRepository interface {
	Save(ctx context.Context, perm *Permission) error
	FindByID(ctx context.Context, id string) (*Permission, error)
	FindAll(ctx context.Context) ([]*Permission, error)
	FindByResource(ctx context.Context, resource string) ([]*Permission, error)
}

type Policy interface {
	Evaluate(ctx context.Context, user *User, resource, action string) (bool, error)
}

type AuditEntry struct {
	ID        common.ID
	UserID    string
	Username  string
	Resource  string
	Action    string
	Granted   bool
	Reason    string
	Timestamp time.Time
}

type AuditLogger interface {
	Log(ctx context.Context, entry *AuditEntry) error
}
