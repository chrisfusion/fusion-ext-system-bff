# fusion-ext-system-bff

Backend for Frontend for external system integrations with the fusion platform.

`fusion-ext-system-bff` sits between external systems (CI/CD pipelines, automation tools, third-party integrations) and internal fusion platform services. It authenticates machine clients using a configurable auth mode, enforces RBAC on `/api/*` routes, and forwards requests to backend services using its own Kubernetes service account token with the authenticated client identity passed as a trusted header.

Unlike `fusion-bff` (which handles human PKCE/session flows), this service handles machine-to-machine traffic only — no browser sessions, no cookies.

## Purpose

- **Public endpoint** — unauthenticated `/api/public/index/*`; `?type=<PUBLIC_TYPE>` forced for listings; `TagGate` middleware does a pre-flight version check against fusion-index when `PUBLIC_DOWNLOAD_TAG` is set
- **Configurable auth** — one of three modes selected by `AUTH_MODES`: `oauth2` (client credentials), `apikey`, or `open` (no auth, dev/internal only)
- **RBAC enforcement** — resolve client identity → roles → permissions; enforce per-route permission checks on all `/api/*` traffic
- **Identity forwarding** — pass resolved system identity (`X-System-ID`, `X-System-Name`) to upstream services as trusted headers
- **Proxy / routing** — forward API calls to fusion-index; additional upstreams added per roadmap
- **Pod-to-pod traffic is not routed through this BFF** — Kubernetes SA TokenReview auth on upstream services handles that path directly

## Stack

- **Go 1.25**, **Gin** (REST API + reverse proxy); module `github.com/fusion-platform/fusion-ext-system-bff`
- **OAuth2 token introspection / JWKS validation** — `github.com/coreos/go-oidc/v3` (reused from fusion-bff pattern)
- **License**: GPL-3.0

## Platform context

| Service | Internal URL | Purpose |
|---|---|---|
| fusion-index | `http://fusion-index-backend.{namespace}.svc.cluster.local:8080` | Artifact registry |

Namespace pattern: `dev-fusion` / `dev-staging-fusion` / `prod-fusion`

## Auth design

Auth mode is selected at startup via `AUTH_MODES`. The three modes share a common `Authenticator` interface — swapping mode requires only config change, no code change.

```
AUTH_MODES=oauth2
  Bearer <access_token> (client credentials grant)
  → introspect or validate JWT via JWKS (OIDC_ISSUER_URL)
  → extract client_id / sub as system identity
  → resolve system identity → roles → permissions (rbac.Engine)
  → set X-System-ID / X-System-Name on upstream request

AUTH_MODES=apikey
  X-Api-Key: <key> header
  → look up key in APIKEY_SOURCE (env list or DB)
  → extract system name bound to key
  → resolve system name → roles → permissions (rbac.Engine)
  → set X-System-ID / X-System-Name on upstream request

AUTH_MODES=open
  No auth validation — identity set to "anonymous"
  NEVER use in production
```

### /api/* middleware (SystemAuth)
1. Run authenticator for current `AUTH_MODES` → extract system identity or reject 401
2. Resolve identity → roles → permissions via `rbac.Engine`
3. Check `MatchRoute(rules, method, path)` → enforce required permission or 403
4. Set `X-System-ID` / `X-System-Name` on the proxied upstream request

## RBAC design

```
System identity (client_id, api key name, or "anonymous")
  └─ StaticSystemRoleStore (rbac.yaml) or DBSystemRoleStore
        │
        ▼
   system_roles map  (rbac.yaml)
        │
        ▼
   role_permissions map  (rbac.yaml)
        │
        ▼
   Resolved { Roles[], Permissions[] }
        │
        └── enforced in SystemAuth middleware per route
```

### rbac.yaml
Loaded from `RBAC_CONFIG_PATH` (default `./rbac.yaml`). In K8s, mount as a ConfigMap volume.
Top-level keys: `system_roles`, `role_permissions`, `route_permissions`.

`route_permissions` follows the same ordered first-match glob semantics as fusion-bff.

**`deployment/rbac.yaml` sync**: Helm chart reads from `deployment/rbac.yaml`. Always update BOTH when changing `rbac.yaml`.

## API key source

`APIKEY_SOURCE` selects key storage:
- `env` — keys loaded from `APIKEY_HASHED_KEYS` env var (`sha256hex:system-id:system-name,...`); for local dev and simple deploys
- `db` — keys stored in postgres `system_api_keys` table; requires `DB_DSN`

Keys are stored hashed (SHA-256). The `env` source requires pre-hashed keys (64 lowercase hex chars, validated at startup).

## Key environment variables

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | Listen port |
| `AUTH_MODES` | — | **Required.** Comma-separated auth modes: `oauth2`, `apikey`, `open` |
| `OIDC_ISSUER_URL` | — | OIDC provider issuer URL (required for `oauth2` mode) |
| `OIDC_CLIENT_ID` | — | Expected `aud` claim (required for `oauth2` mode) |
| `OIDC_JWKS_URL` | `{OIDC_ISSUER_URL}/protocol/openid-connect/certs` | Override JWKS endpoint |
| `OIDC_JWKS_CACHE_TTL` | `15m` | JWKS key set refresh interval |
| `APIKEY_SOURCE` | `env` | API key backend: `env` or `db` (used when `AUTH_MODES=apikey`) |
| `APIKEY_HASHED_KEYS` | — | Comma-separated `sha256hex:system-id:system-name` entries (used when `APIKEY_SOURCE=env`) |
| `INDEX_URL` | `http://fusion-index-backend.fusion.svc.cluster.local:8080` | fusion-index base URL |
| `RBAC_CONFIG_PATH` | `./rbac.yaml` | Path to RBAC config file |
| `DB_DSN` | — | PostgreSQL DSN; required when `APIKEY_SOURCE=db` |
| `PUBLIC_TYPE` | `streamlit` | Artifact type forced as `?type=` on all `/api/public/*` listing requests |
| `PUBLIC_DOWNLOAD_TAG` | — | Tag that must be present on a version for it to be downloadable via `/api/public/*`; if unset, no tag check is performed |

## Deployment

Same Flux + Helm pattern as fusion-bff:
- Helm chart at `deployment/`
- Flux config at `flux/` with three environments: `dev-fusion`, `dev-staging-fusion`, `prod-fusion`
- Self-contained chart (no external subchart dependencies)
- Always use semver image tags — never `latest`

## Commands

```bash
# Dev build
go build ./...

# Unit tests
go test ./... -v -race

# e2e tests (no external services needed — uses httptest mock servers)
go test ./test/e2e/... -tags e2e -v -timeout=120s

# Build Docker image (inside minikube)
make docker-build IMG=fusion-ext-system-bff:local

# Run locally (reads .env if present)
make run

# Port-forward
kubectl port-forward -n fusion service/fusion-ext-system-bff 18082:8080 --address 127.0.0.1
```

## Project structure

See [ENDPOINT.md](ENDPOINT.md) for the full route reference.

```
cmd/
  server/main.go              # Entry point — loads config, selects Authenticator, wires routes
internal/
  config/config.go            # Env var loading
  auth/
    auth.go                   # Authenticator interface + Chain
    oauth2.go                 # OAuth2Authenticator — JWKS validation, client_id extraction
    apikey.go                 # APIKeyAuthenticator — header lookup, key→system resolution
    open.go                   # OpenAuthenticator — no-op, returns "anonymous"
  apikey/
    store.go                  # APIKeyStore interface
    env_store.go              # EnvKeyStore — loaded from APIKEY_HASHED_KEYS env var
    db_store.go               # DBKeyStore — postgres-backed, hashed keys
  rbac/
    config.go                 # RBACConfig, LoadConfig
    engine.go                 # Engine.HasPermission() — system identity → roles → permissions
    route.go                  # MatchRoute() — first-match glob
  db/
    db.go                     # Open(), Migrate()
  proxy/
    upstream.go               # UpstreamProxy for fusion-index
  api/
    handler/health.go         # /health /livez /readyz
    middleware/sysauth.go     # AUTH_MODES dispatch + RBAC enforcement for /api/*
    middleware/taggate.go     # Pre-flight version tag check for /api/public/* (PUBLIC_DOWNLOAD_TAG)
    middleware/requestid.go
    router.go                 # Gin routes
rbac.yaml                     # Root config — keep in sync with deployment/rbac.yaml
deployment/rbac.yaml          # Helm chart copy
ENDPOINT.md                   # Full route reference
test/e2e/                     # e2e tests (build tag: e2e)
deployment/                   # Helm chart
flux/                         # Flux GitOps (3 environments)
Dockerfile
Makefile
```
