package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPPort string

	// AUTH_MODES — comma-separated ordered list: oauth2, apikey, open
	AuthModes []string

	// oauth2 mode
	OIDCIssuerURL string
	OIDCClientID  string
	OIDCJWKSURL   string
	JWKSCacheTTL  time.Duration

	// apikey mode
	APIKeyHeader     string // APIKEY_HEADER, default "X-Api-Key"
	APIKeySource     string // APIKEY_SOURCE: "env" or "db", default "env"
	APIKeyHashedKeys string // APIKEY_HASHED_KEYS: comma-separated sha256hex:id:name entries

	// open mode
	OpenSystemID string // OPEN_SYSTEM_ID, default "anonymous"

	// DB (required when apikey source is "db")
	DBDSN string

	// upstream
	IndexURL string

	// RBAC
	RBACConfigPath string
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPPort:         envOrDefault("HTTP_PORT", "8080"),
		OIDCIssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		OIDCJWKSURL:      os.Getenv("OIDC_JWKS_URL"),
		APIKeyHeader:     envOrDefault("APIKEY_HEADER", "X-Api-Key"),
		APIKeySource:     envOrDefault("APIKEY_SOURCE", "env"),
		APIKeyHashedKeys: os.Getenv("APIKEY_HASHED_KEYS"),
		OpenSystemID:     envOrDefault("OPEN_SYSTEM_ID", "anonymous"),
		DBDSN:            os.Getenv("DB_DSN"),
		IndexURL:         envOrDefault("INDEX_URL", "http://fusion-index-backend.fusion.svc.cluster.local:8080"),
		RBACConfigPath:   envOrDefault("RBAC_CONFIG_PATH", "./rbac.yaml"),
	}

	raw := os.Getenv("AUTH_MODES")
	if raw == "" {
		return nil, fmt.Errorf("AUTH_MODES is required (valid values: oauth2, apikey, open)")
	}
	for _, m := range strings.Split(raw, ",") {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		switch m {
		case "oauth2", "apikey", "open":
			cfg.AuthModes = append(cfg.AuthModes, m)
		default:
			return nil, fmt.Errorf("AUTH_MODES: unknown mode %q (valid: oauth2, apikey, open)", m)
		}
	}
	if len(cfg.AuthModes) == 0 {
		return nil, fmt.Errorf("AUTH_MODES must contain at least one mode")
	}

	// open mode must not be combined with other modes — it always succeeds and
	// would silently authenticate any credential-less request that reaches it.
	if len(cfg.AuthModes) > 1 {
		for _, m := range cfg.AuthModes {
			if m == "open" {
				return nil, fmt.Errorf("AUTH_MODES: 'open' cannot be combined with other modes")
			}
		}
	}

	for _, m := range cfg.AuthModes {
		switch m {
		case "oauth2":
			if cfg.OIDCIssuerURL == "" {
				return nil, fmt.Errorf("OIDC_ISSUER_URL is required when AUTH_MODES includes oauth2")
			}
			if cfg.OIDCClientID == "" {
				return nil, fmt.Errorf("OIDC_CLIENT_ID is required when AUTH_MODES includes oauth2")
			}
			if cfg.OIDCJWKSURL == "" {
				cfg.OIDCJWKSURL = strings.TrimRight(cfg.OIDCIssuerURL, "/") + "/protocol/openid-connect/certs"
			}
		case "apikey":
			if cfg.APIKeySource != "env" && cfg.APIKeySource != "db" {
				return nil, fmt.Errorf("APIKEY_SOURCE must be \"env\" or \"db\", got %q", cfg.APIKeySource)
			}
			if cfg.APIKeySource == "db" && cfg.DBDSN == "" {
				return nil, fmt.Errorf("DB_DSN is required when APIKEY_SOURCE=db")
			}
			if cfg.APIKeySource == "env" && cfg.APIKeyHashedKeys == "" {
				return nil, fmt.Errorf("APIKEY_HASHED_KEYS is required when APIKEY_SOURCE=env")
			}
		}
	}

	var err error
	if cfg.JWKSCacheTTL, err = parseDuration("OIDC_JWKS_CACHE_TTL", 15*time.Minute); err != nil {
		return nil, err
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", key, v, err)
	}
	return d, nil
}
