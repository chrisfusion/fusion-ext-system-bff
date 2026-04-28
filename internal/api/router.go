package api

import (
	"github.com/gin-gonic/gin"

	"github.com/fusion-platform/fusion-ext-system-bff/internal/api/handler"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/api/middleware"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/auth"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/proxy"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/rbac"
)

func NewRouter(chain *auth.Chain, engine *rbac.Engine, index *proxy.UpstreamProxy, publicIndex *proxy.UpstreamProxy, tagGate gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(middleware.RequestID())

	r.GET("/health", handler.Health)
	r.GET("/livez", handler.Livez)
	r.GET("/readyz", handler.Readyz)

	// Public read-only endpoint — no authentication.
	// The proxy forces ?type=<PUBLIC_TYPE> upstream for listing; tagGate enforces
	// a required version tag before proxying version-scoped download requests.
	pub := r.Group("/api/public")
	pub.Use(tagGate)
	pub.GET("/index/*path", publicIndex.Handler())

	// Authenticated routes — auth + RBAC enforced for all /api/index/* traffic.
	authenticated := r.Group("/api")
	authenticated.Use(middleware.SystemAuth(chain, engine))
	authenticated.Any("/index/*path", index.Handler())

	return r
}
