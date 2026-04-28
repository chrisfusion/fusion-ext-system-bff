package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/fusion-platform/fusion-ext-system-bff/internal/apikey"
)

type apikeyAuthenticator struct {
	headerName string
	store      apikey.Store
}

// NewAPIKeyAuthenticator builds an Authenticator that reads an API key from a
// configurable request header, hashes it with SHA-256, and resolves the identity.
func NewAPIKeyAuthenticator(headerName string, store apikey.Store) Authenticator {
	return &apikeyAuthenticator{headerName: headerName, store: store}
}

func (a *apikeyAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*SystemIdentity, error) {
	rawKey := r.Header.Get(a.headerName)
	if rawKey == "" {
		return nil, ErrNotApplicable
	}

	sum := sha256.Sum256([]byte(rawKey))
	hexHash := hex.EncodeToString(sum[:])

	systemID, systemName, err := a.store.Lookup(ctx, hexHash)
	if err != nil {
		if errors.Is(err, apikey.ErrNotFound) {
			// Include hash prefix so operators can correlate with audit logs
			// without exposing the full hash or the raw key.
			return nil, fmt.Errorf("apikey: unknown key (prefix: %s)", hexHash[:8])
		}
		return nil, fmt.Errorf("apikey: lookup: %w", err)
	}

	return &SystemIdentity{ID: systemID, Name: systemName}, nil
}
