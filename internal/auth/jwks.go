package auth

import (
	"context"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
)

// cachingKeySet wraps a RemoteKeySet with a TTL-forced refresh.
// This ensures key rotation is picked up on a predictable schedule regardless
// of HTTP cache headers from the JWKS endpoint.
type cachingKeySet struct {
	mu        sync.RWMutex
	inner     *gooidc.RemoteKeySet
	expiresAt time.Time
	ttl       time.Duration
	jwksURL   string
}

func newCachingKeySet(ctx context.Context, jwksURL string, ttl time.Duration) *cachingKeySet {
	return &cachingKeySet{
		inner:   gooidc.NewRemoteKeySet(ctx, jwksURL),
		ttl:     ttl,
		jwksURL: jwksURL,
	}
}

func (c *cachingKeySet) VerifySignature(ctx context.Context, jwt string) ([]byte, error) {
	c.mu.RLock()
	expired := time.Now().After(c.expiresAt)
	c.mu.RUnlock()

	if expired {
		c.mu.Lock()
		if time.Now().After(c.expiresAt) {
			// Use context.Background so a cancelled request context does not
			// abort an in-progress key rotation.
			c.inner = gooidc.NewRemoteKeySet(context.Background(), c.jwksURL)
			c.expiresAt = time.Now().Add(c.ttl)
		}
		c.mu.Unlock()
	}

	c.mu.RLock()
	inner := c.inner
	c.mu.RUnlock()

	return inner.VerifySignature(ctx, jwt)
}
