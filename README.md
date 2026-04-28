# fusion-ext-system-bff

Backend for Frontend for external system integrations with the fusion platform.

`fusion-ext-system-bff` sits between external systems — CI/CD pipelines, admin tools, and automated consumers — and internal fusion platform services. It authenticates machine clients, enforces RBAC on all `/api/*` routes, and forwards requests to backend services with the resolved system identity passed as trusted headers.

Unlike `fusion-bff` (which handles human PKCE/session flows), this service handles machine-to-machine traffic only. No browser sessions, no cookies.

## Features

- **Public API** — unauthenticated read access at `/api/public/index/*`; artifact type filter and optional per-version tag gate are independently configurable
- **Configurable auth** — `oauth2` (JWT/client credentials), `apikey`, or `open` (dev only)
- **Live key rotation** — API keys managed in PostgreSQL; revoke or add without redeployment
- **RBAC** — three roles: `reader`, `writer`, `admin` (hard delete)
- **Identity forwarding** — resolved system identity passed as `X-System-ID` / `X-System-Name` to upstream services

## Quick start

```bash
# Run locally (reads .env if present)
make run

# Unit tests
go test ./... -v -race

# Build binary
make build

# Build Docker image (inside minikube)
eval $(minikube docker-env) && make docker-build IMG=fusion-ext-system-bff:local
```

## API routes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` `/livez` `/readyz` | none | Health and readiness probes |
| `GET` | `/api/public/index/*` | **none** | Public artifact reads — type-filtered, optionally tag-gated |
| `ANY` | `/api/index/*` | required | Authenticated proxy to fusion-index |

See [ENDPOINT.md](ENDPOINT.md) for the full route reference including parameters, permissions, and error codes.

## Auth modes

Set `AUTH_MODES` to one of the following. `open` cannot be combined with other modes.

| Mode | Credential | Production |
|------|-----------|------------|
| `apikey` | `X-Api-Key` header | Yes (with `APIKEY_SOURCE=db`) |
| `oauth2` | `Authorization: Bearer <jwt>` | Yes |
| `open` | none | No — dev only |

## Roles

| Role | Permissions |
|------|------------|
| `reader` | `index:artifacts:read` |
| `writer` | `index:artifacts:read`, `index:artifacts:write` |
| `admin` | `index:artifacts:read`, `index:artifacts:write`, `index:artifacts:delete` |

Roles are assigned to system identities in `rbac.yaml`. See [SETTINGS.md](SETTINGS.md) for configuration and [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

## Public API

The public endpoint at `/api/public/index/*` requires no authentication and is read-only.

Two independent controls restrict what is accessible:

- **Type filter** (`PUBLIC_TYPE`, default `streamlit`) — forced as `?type=` on every listing request; clients cannot override it.
- **Tag gate** (`PUBLIC_DOWNLOAD_TAG`) — when set, any request targeting a specific artifact version is blocked unless that version carries the configured tag. If unset, no tag check is performed.

```bash
# List publicly visible artifacts (type=streamlit forced)
curl https://ext-bff.example.com/api/public/index/api/v1/artifacts

# Download a file — only works if the version has the PUBLIC_DOWNLOAD_TAG tag
curl https://ext-bff.example.com/api/public/index/api/v1/artifacts/42/versions/1.2.3/files/7/download
```

## Key management (no redeployment)

```bash
# Hash a raw key
printf '%s' 'my-raw-key' | sha256sum | cut -d' ' -f1

# Add a key
psql "$DB_DSN" -c "INSERT INTO system_api_keys (system_id, system_name, hashed_key)
                   VALUES ('ci-pipeline', 'CI Pipeline', '<hash>');"

# Revoke a key (takes effect immediately, no restart needed)
psql "$DB_DSN" -c "UPDATE system_api_keys SET active = FALSE WHERE hashed_key = '<hash>';"
```

## Deployment

Helm chart at `deployment/`. Follows the same Flux + GitOps pattern as `fusion-bff`.

```bash
helm install fusion-ext-system-bff deployment/ \
  --namespace fusion \
  --set config.authModes=apikey \
  --set config.apikeySource=db \
  --set secret.dbDsn="postgres://user:pass@host:5432/db"
```

Three environments via Flux: `dev-fusion`, `dev-staging-fusion`, `prod-fusion`.

## Project structure

```
cmd/server/main.go              Entry point
internal/
  config/config.go              Env var loading and validation
  auth/                         Authenticator chain (oauth2, apikey, open)
  apikey/                       Key stores (env, db)
  rbac/                         Engine, route matcher, YAML config
  db/                           PostgreSQL pool, migration
  proxy/upstream.go             Reverse proxy with identity injection
  api/
    router.go                   Route wiring (public + authenticated groups)
    handler/health.go           /health /livez /readyz
    middleware/sysauth.go       Auth + RBAC enforcement
    middleware/taggate.go       Pre-flight tag check for public version routes
    middleware/requestid.go     X-Request-ID passthrough/generation
rbac.yaml                       RBAC config — keep in sync with deployment/rbac.yaml
deployment/                     Helm chart
```

## License

GPL-3.0
