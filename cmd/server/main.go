package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fusion-platform/fusion-ext-system-bff/internal/api"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/api/middleware"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/apikey"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/auth"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/config"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/db"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/proxy"
	"github.com/fusion-platform/fusion-ext-system-bff/internal/rbac"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	rbacCfg, err := rbac.LoadConfig(cfg.RBACConfigPath)
	if err != nil {
		log.Fatalf("rbac config: %v", err)
	}
	rbacEngine := rbac.NewEngine(rbacCfg)

	// Build the auth chain in the order specified by AUTH_MODES.
	var authenticators []auth.Authenticator
	var pool *pgxpool.Pool

	for _, mode := range cfg.AuthModes {
		switch mode {
		case "oauth2":
			a, aerr := auth.NewOAuth2Authenticator(ctx,
				cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCJWKSURL, cfg.JWKSCacheTTL)
			if aerr != nil {
				log.Fatalf("oauth2 authenticator: %v", aerr)
			}
			authenticators = append(authenticators, a)

		case "apikey":
			var store apikey.Store
			if cfg.APIKeySource == "db" {
				pool, err = db.Open(ctx, cfg.DBDSN)
				if err != nil {
					log.Fatalf("db: %v", err)
				}
				if err = db.Migrate(ctx, pool); err != nil {
					log.Fatalf("db migrate: %v", err)
				}
				store = apikey.NewDBStore(pool)
			} else {
				store, err = apikey.NewEnvStore(cfg.APIKeyHashedKeys)
				if err != nil {
					log.Fatalf("apikey env store: %v", err)
				}
			}
			authenticators = append(authenticators, auth.NewAPIKeyAuthenticator(cfg.APIKeyHeader, store))

		case "open":
			log.Println("[WARNING] AUTH_MODES includes 'open' — no authentication enforced — NOT for production use")
			authenticators = append(authenticators, auth.NewOpenAuthenticator(cfg.OpenSystemID))
		}
	}

	if pool != nil {
		defer pool.Close()
	}

	chain := auth.NewChain(authenticators...)

	indexProxy, err := proxy.NewUpstreamProxy(cfg.IndexURL, "/api/index")
	if err != nil {
		log.Fatalf("index proxy: %v", err)
	}

	// Public proxy for /api/public/index — no auth, type forced upstream for listings.
	publicIndexProxy, err := proxy.NewUpstreamProxy(cfg.IndexURL, "/api/public/index",
		proxy.WithForcedQuery(url.Values{"type": {cfg.PublicType}}))
	if err != nil {
		log.Fatalf("public index proxy: %v", err)
	}

	tagGate := middleware.TagGate(cfg.IndexURL, cfg.PublicDownloadTag)
	router := api.NewRouter(chain, rbacEngine, indexProxy, publicIndexProxy, tagGate)

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		log.Println("shutting down")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutCancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}
