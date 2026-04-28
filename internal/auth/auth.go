package auth

import (
	"context"
	"errors"
	"net/http"
)

// SystemIdentity is the resolved identity of an authenticated external system.
type SystemIdentity struct {
	ID   string // forwarded as X-System-ID
	Name string // forwarded as X-System-Name
}

// ErrNotApplicable is returned when an Authenticator cannot handle the request
// because no credential for its mode is present. The chain advances to the next
// authenticator. Any other error means credentials were found but invalid; the
// chain stops and a 401 is returned.
var ErrNotApplicable = errors.New("authenticator: not applicable")

// Authenticator attempts to identify the caller from the incoming request.
type Authenticator interface {
	Authenticate(ctx context.Context, r *http.Request) (*SystemIdentity, error)
}

// Chain tries each Authenticator in order.
// The first non-ErrNotApplicable result terminates the chain.
// If all authenticators return ErrNotApplicable, ErrNotApplicable is returned.
type Chain struct {
	chain []Authenticator
}

func NewChain(auths ...Authenticator) *Chain {
	return &Chain{chain: auths}
}

func (c *Chain) Authenticate(ctx context.Context, r *http.Request) (*SystemIdentity, error) {
	for _, a := range c.chain {
		id, err := a.Authenticate(ctx, r)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, ErrNotApplicable) {
			return nil, err
		}
	}
	return nil, ErrNotApplicable
}
