package apikey

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
)

// EnvStore resolves API keys from a static in-memory map loaded at startup.
// Each entry in the raw string has the format: <sha256hex>:<system-id>:<system-name>
// Multiple entries are comma-separated.
// Keys are pre-hashed by the operator: echo -n "rawkey" | sha256sum
type EnvStore struct {
	keys map[string][2]string // sha256hex → [systemID, systemName]
}

func NewEnvStore(raw string) (*EnvStore, error) {
	store := &EnvStore{keys: make(map[string][2]string)}
	if raw == "" {
		return store, nil
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("apikey env store: invalid entry %q (expected sha256hex:id:name)", entry)
		}
		hashHex := parts[0]
		if len(hashHex) != 64 {
			return nil, fmt.Errorf("apikey env store: entry %q: hash must be 64 hex chars (got %d)", entry, len(hashHex))
		}
		if _, err := hex.DecodeString(hashHex); err != nil {
			return nil, fmt.Errorf("apikey env store: entry %q: hash is not valid hex: %w", entry, err)
		}
		store.keys[hashHex] = [2]string{parts[1], parts[2]}
	}
	return store, nil
}

func (s *EnvStore) Lookup(_ context.Context, keyHash string) (string, string, error) {
	if v, ok := s.keys[keyHash]; ok {
		return v[0], v[1], nil
	}
	return "", "", ErrNotFound
}
