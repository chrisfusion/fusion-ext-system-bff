package middleware

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fusion-platform/fusion-ext-system-bff/internal/auth"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/proxy"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/rbac"
)

// SystemAuth authenticates external system callers and enforces RBAC.
// The chain is tried in config order; first success wins.
// After successful auth, the required route permission is checked globally.
func SystemAuth(chain *auth.Chain, engine *rbac.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Strip inbound identity headers before auth to prevent spoofing.
		c.Request.Header.Del("X-System-ID")
		c.Request.Header.Del("X-System-Name")

		identity, err := chain.Authenticate(c.Request.Context(), c.Request)
		if err != nil {
			if !errors.Is(err, auth.ErrNotApplicable) {
				log.Printf("sysauth: %v", err)
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		perm := rbac.MatchRoute(engine.RoutePermissions(), c.Request.Method, c.Request.URL.Path)
		if perm != "" && !engine.HasPermission(identity.ID, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Request = proxy.SetSystemContext(c.Request, identity.ID, identity.Name)
		c.Next()
	}
}
