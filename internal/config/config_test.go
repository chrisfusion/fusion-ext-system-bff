package config

import (
	"strings"
	"testing"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestLoad_OpenCannotBeMixed(t *testing.T) {
	setEnv(t, map[string]string{"AUTH_MODES": "apikey,open", "APIKEY_HASHED_KEYS": "a"})
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected 'cannot be combined' error, got: %v", err)
	}
}

func TestLoad_OpenAlone_OK(t *testing.T) {
	setEnv(t, map[string]string{"AUTH_MODES": "open"})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.AuthModes) != 1 || cfg.AuthModes[0] != "open" {
		t.Fatalf("expected [open], got %v", cfg.AuthModes)
	}
}

func TestLoad_ApikeyEnvRequiresHashedKeys(t *testing.T) {
	setEnv(t, map[string]string{"AUTH_MODES": "apikey", "APIKEY_SOURCE": "env"})
	// APIKEY_HASHED_KEYS not set
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "APIKEY_HASHED_KEYS") {
		t.Fatalf("expected APIKEY_HASHED_KEYS error, got: %v", err)
	}
}

func TestLoad_ApikeyEnvWithKeys_OK(t *testing.T) {
	hash := strings.Repeat("a", 64)
	setEnv(t, map[string]string{
		"AUTH_MODES":         "apikey",
		"APIKEY_SOURCE":      "env",
		"APIKEY_HASHED_KEYS": hash + ":sys-a:System A",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKeyHashedKeys == "" {
		t.Fatal("expected APIKeyHashedKeys to be set")
	}
}

func TestLoad_OAuth2RequiresIssuerAndClientID(t *testing.T) {
	setEnv(t, map[string]string{"AUTH_MODES": "oauth2"})
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "OIDC_ISSUER_URL") {
		t.Fatalf("expected OIDC_ISSUER_URL error, got: %v", err)
	}
}
