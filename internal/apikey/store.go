package apikey

import (
	"context"
	"errors"
)

// ErrNotFound is returned when the key hash is not in the store.
var ErrNotFound = errors.New("apikey: not found")

// Store resolves a SHA-256 hashed API key to the system identity it represents.
// Implementations receive the hex-encoded hash of the raw key — never the raw key itself.
type Store interface {
	Lookup(ctx context.Context, keyHash string) (systemID, systemName string, err error)
}
