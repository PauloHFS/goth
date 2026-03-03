package rbac

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
)

// Enforcer wraps Casbin enforcer with application-specific methods
type Enforcer struct {
	*casbin.Enforcer
}

// NewEnforcer creates a new RBAC enforcer
func NewEnforcer(modelPath, policyPath string) (*Enforcer, error) {
	// Load Casbin model
	casbinModel, err := model.NewModelFromFile(modelPath)
	if err != nil {
		return nil, err
	}

	// Load policy adapter
	adapter := fileadapter.NewAdapter(policyPath)

	// Create enforcer
	enforcer, err := casbin.NewEnforcer(casbinModel, adapter)
	if err != nil {
		return nil, err
	}

	// Enable auto-save
	enforcer.EnableAutoSave(true)

	return &Enforcer{
		Enforcer: enforcer,
	}, nil
}

// NewEnforcerWithModel creates enforcer with inline model
func NewEnforcerWithModel(policyPath string) (*Enforcer, error) {
	// RBAC model with resource ownership
	text := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _
g2 = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (g(r.sub, p.sub) || r.sub == p.sub) && (r.obj == p.obj || r.obj == p.sub) && r.act == p.act
`

	casbinModel, err := model.NewModelFromString(text)
	if err != nil {
		return nil, err
	}

	adapter := fileadapter.NewAdapter(policyPath)
	enforcer, err := casbin.NewEnforcer(casbinModel, adapter)
	if err != nil {
		return nil, err
	}

	enforcer.EnableAutoSave(true)

	return &Enforcer{
		Enforcer: enforcer,
	}, nil
}

// CheckPermission checks if user has permission
func (e *Enforcer) CheckPermission(userID string, resource string, action string) bool {
	allowed, err := e.Enforce(userID, resource, action)
	if err != nil {
		return false
	}
	return allowed
}

// AddRole adds a role to user
func (e *Enforcer) AddRole(userID string, role string) error {
	_, err := e.AddGroupingPolicy(userID, role)
	return err
}

// RemoveRole removes a role from user
func (e *Enforcer) RemoveRole(userID string, role string) error {
	_, err := e.RemoveGroupingPolicy(userID, role)
	return err
}

// GetRoles gets all roles for user
func (e *Enforcer) GetRoles(userID string) ([]string, error) {
	return e.GetImplicitRolesForUser(userID)
}

// AddPermission adds a permission to role
func (e *Enforcer) AddPermission(role string, resource string, action string) error {
	_, err := e.AddPolicy(role, resource, action)
	return err
}

// RemovePermission removes a permission from role
func (e *Enforcer) RemovePermission(role string, resource string, action string) error {
	_, err := e.RemovePolicy(role, resource, action)
	return err
}

// GetPermissions gets all permissions for role
func (e *Enforcer) GetPermissions(role string) ([][]string, error) {
	return e.GetFilteredPolicy(0, role)
}

// GetUserPermissions gets all permissions for user (including roles)
func (e *Enforcer) GetUserPermissions(userID string) ([][]string, error) {
	return e.GetImplicitPermissionsForUser(userID)
}

// HasRole checks if user has a role
func (e *Enforcer) HasRole(userID string, role string) bool {
	roles, _ := e.GetImplicitRolesForUser(userID)
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin checks if user is admin
func (e *Enforcer) IsAdmin(userID string) bool {
	return e.HasRole(userID, "admin")
}

// CanView checks if user can view resource
func (e *Enforcer) CanView(userID string, resource string) bool {
	return e.CheckPermission(userID, resource, "view")
}

// CanCreate checks if user can create resource
func (e *Enforcer) CanCreate(userID string, resource string) bool {
	return e.CheckPermission(userID, resource, "create")
}

// CanEdit checks if user can edit resource
func (e *Enforcer) CanEdit(userID string, resource string) bool {
	return e.CheckPermission(userID, resource, "edit")
}

// CanDelete checks if user can delete resource
func (e *Enforcer) CanDelete(userID string, resource string) bool {
	return e.CheckPermission(userID, resource, "delete")
}

// InitializeDefaultRoles creates default roles and permissions
func (e *Enforcer) InitializeDefaultRoles() error {
	// Admin role - full access
	adminPermissions := [][]string{
		{"admin", "*", "view"},
		{"admin", "*", "create"},
		{"admin", "*", "edit"},
		{"admin", "*", "delete"},
	}

	for _, perm := range adminPermissions {
		hasPolicy, _ := e.HasPolicy(perm)
		if !hasPolicy {
			_, err := e.AddPolicy(perm)
			if err != nil {
				return err
			}
		}
	}

	// User role - basic access
	userPermissions := [][]string{
		{"user", "dashboard", "view"},
		{"user", "profile", "view"},
		{"user", "profile", "edit"},
		{"user", "settings", "view"},
		{"user", "settings", "edit"},
	}

	for _, perm := range userPermissions {
		hasPolicy, _ := e.HasPolicy(perm)
		if !hasPolicy {
			_, err := e.AddPolicy(perm)
			if err != nil {
				return err
			}
		}
	}

	// Billing role - billing access
	billingPermissions := [][]string{
		{"billing", "billing", "view"},
		{"billing", "billing", "edit"},
		{"billing", "invoices", "view"},
		{"billing", "payments", "view"},
		{"billing", "payments", "create"},
	}

	for _, perm := range billingPermissions {
		hasPolicy, _ := e.HasPolicy(perm)
		if !hasPolicy {
			_, err := e.AddPolicy(perm)
			if err != nil {
				return err
			}
		}
	}

	// Moderator role - content moderation
	moderatorPermissions := [][]string{
		{"moderator", "content", "view"},
		{"moderator", "content", "edit"},
		{"moderator", "content", "delete"},
		{"moderator", "reports", "view"},
		{"moderator", "reports", "edit"},
	}

	for _, perm := range moderatorPermissions {
		hasPolicy, _ := e.HasPolicy(perm)
		if !hasPolicy {
			_, err := e.AddPolicy(perm)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// LoadPolicy reloads policy from file
func (e *Enforcer) LoadPolicy() error {
	return e.Enforcer.LoadPolicy()
}

// SavePolicy saves policy to file
func (e *Enforcer) SavePolicy() error {
	return e.Enforcer.SavePolicy()
}

// ClearPolicy clears all policies
func (e *Enforcer) ClearPolicy() error {
	e.Enforcer.ClearPolicy()
	return nil
}
