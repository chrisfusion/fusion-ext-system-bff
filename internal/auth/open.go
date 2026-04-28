package auth

import (
	"context"
	"net/http"
)

type openAuthenticator struct {
	identity SystemIdentity
}

// NewOpenAuthenticator builds an Authenticator that always succeeds with a
// fixed system identity. For development and testing only — never production.
func NewOpenAuthenticator(systemID string) Authenticator {
	if systemID == "" {
		systemID = "anonymous"
	}
	return &openAuthenticator{identity: SystemIdentity{ID: systemID, Name: systemID}}
}

func (a *openAuthenticator) Authenticate(_ context.Context, _ *http.Request) (*SystemIdentity, error) {
	id := a.identity
	return &id, nil
}
