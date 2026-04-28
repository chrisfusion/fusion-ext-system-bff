package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
)

type oauth2Authenticator struct {
	verifier *gooidc.IDTokenVerifier
}

// NewOAuth2Authenticator builds an Authenticator that validates Bearer JWTs
// against a JWKS endpoint. The "sub" claim is used as the system identity.
func NewOAuth2Authenticator(ctx context.Context, issuerURL, clientID, jwksURL string, cacheTTL time.Duration) (Authenticator, error) {
	if jwksURL == "" {
		return nil, fmt.Errorf("oauth2 authenticator: JWKS URL is required")
	}
	keySet := newCachingKeySet(ctx, jwksURL, cacheTTL)
	verifier := gooidc.NewVerifier(issuerURL, keySet, &gooidc.Config{ClientID: clientID})
	return &oauth2Authenticator{verifier: verifier}, nil
}

func (a *oauth2Authenticator) Authenticate(ctx context.Context, r *http.Request) (*SystemIdentity, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, ErrNotApplicable
	}
	rawToken := strings.TrimPrefix(header, "Bearer ")

	token, err := a.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("oauth2: token verification failed: %w", err)
	}

	var claims struct {
		Sub  string `json:"sub"`
		Name string `json:"name"`
	}
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oauth2: extracting claims: %w", err)
	}

	name := claims.Name
	if name == "" {
		name = claims.Sub
	}
	return &SystemIdentity{ID: claims.Sub, Name: name}, nil
}
