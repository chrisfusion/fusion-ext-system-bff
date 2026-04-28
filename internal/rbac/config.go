package rbac

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RBACConfig is the top-level structure loaded from rbac.yaml.
type RBACConfig struct {
	SystemRoles      map[string][]string `yaml:"system_roles"`
	RolePermissions  map[string][]string `yaml:"role_permissions"`
	RoutePermissions []RouteRule         `yaml:"route_permissions"`
}

// RouteRule maps an HTTP method + path pattern to a required permission string.
type RouteRule struct {
	Method     string `yaml:"method"`
	Path       string `yaml:"path"`
	Permission string `yaml:"permission"`
}

func LoadConfig(path string) (*RBACConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rbac: read config %q: %w", path, err)
	}
	var cfg RBACConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("rbac: parse config: %w", err)
	}
	return &cfg, nil
}
