package rbac

import "slices"

// Engine resolves global permissions for a system identity from static YAML.
type Engine struct {
	cfg *RBACConfig
}

func NewEngine(cfg *RBACConfig) *Engine {
	return &Engine{cfg: cfg}
}

// HasPermission returns true if the system identity holds the given permission
// via any of its assigned roles.
func (e *Engine) HasPermission(systemID, permission string) bool {
	for _, role := range e.cfg.SystemRoles[systemID] {
		if slices.Contains(e.cfg.RolePermissions[role], permission) {
			return true
		}
	}
	return false
}

// RoutePermissions exposes the config rules for use in middleware.
func (e *Engine) RoutePermissions() []RouteRule {
	return e.cfg.RoutePermissions
}
