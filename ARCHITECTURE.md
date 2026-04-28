# Architecture

## Overview

`fusion-ext-system-bff` is a stateless reverse proxy with an authentication and RBAC gate. It has no business logic of its own — its sole job is to authenticate an inbound machine identity, verify it has the right permission for the requested route, and forward the request to the appropriate upstream service with the resolved identity injected as trusted headers.

```
External system
      │
      │  GET /api/public/index/...         (no credential)
      │  GET /api/index/...                X-Api-Key: <key>
      │  POST /api/index/...               X-Api-Key: <key>
      │  DELETE /api/index/...             X-Api-Key: <key>
      ▼
┌─────────────────────────────────────────────────────┐
│                fusion-ext-system-bff                │
│                                                     │
│  ┌──────────┐   ┌────────────────────────────────┐  │
│  │ /health  │   │ /api/public/index/*            │  │
│  │ /livez   │   │  no auth — type=streamlit      │  │
│  │ /readyz  │   │  forced upstream               │  │
│  └──────────┘   └───────────��───┬────────────────┘  │
│                                 │                   │
│                 ┌───────────────▼────────────────┐  │
│                 │ /api/index/*                   │  │
│                 │  1. strip inbound identity hdrs │  │
│                 │  2. authenticate (auth chain)   │  │
│                 │  3. RBAC permission check       │  │
│                 │  4. inject X-System-ID/Name     │  │
│                 └───────────────┬────────────────┘  │
└─────────────────────────────────┼───────────────────┘
                                  │
                                  ▼
                        fusion-index-backend
                        (Kubernetes ClusterIP)
```

## Request flow — authenticated route

```
1. RequestID middleware
   └─ pass through X-Request-ID or generate one

2. SystemAuth middleware
   ├─ del X-System-ID, X-System-Name from inbound request (spoofing prevention)
   ├─ auth chain: try each authenticator in AUTH_MODES order
   │    oauth2  → no Bearer header          → ErrNotApplicable → try next
   │    apikey  → header present, hash miss  → reject 401
   │    apikey  → header present, hash hit   → SystemIdentity{ID, Name}
   │    open    → always succeeds (dev only)
   ├─ no authenticator matched              → 401
   ├─ RBAC: MatchRoute(method, path)        → required permission (or "")
   ├─ engine.HasPermission(id, perm) false  → 403
   └─ SetSystemContext(r, id, name)         → identity stored in request context

3. UpstreamProxy.Rewrite
   ├─ strip /api/index prefix from path
   ├─ apply forced query params (public route only)
   ├─ del X-System-ID, X-System-Name from outbound headers
   └─ set X-System-ID, X-System-Name from context
```

## Public route

`GET /api/public/index/*path` is registered outside the `SystemAuth` middleware group. No credential is needed. The proxy rewrites the upstream query to always include `type=streamlit`, overriding any client-supplied value — it is structurally impossible to use this endpoint to read non-Streamlit artifacts.

Identity headers are still stripped in the proxy Rewrite, preventing callers from injecting `X-System-ID` / `X-System-Name` to impersonate an authenticated identity.

## Auth chain

The auth chain is ordered: each authenticator returns one of three outcomes:

| Outcome | Meaning | Chain action |
|---------|---------|--------------|
| `(identity, nil)` | Authenticated | Stop, use identity |
| `ErrNotApplicable` | Credential type not present | Try next authenticator |
| `(nil, error)` | Credential present but invalid | Stop, return 401 |

This means a caller with a `Bearer` token that fails JWT validation always gets a 401, even if `open` comes later in the list. `open` only wins for requests that carry no credential at all — which is why it is forbidden from being combined with other modes in config validation.

## RBAC

```
system identity (sub, api key system_id, or "anonymous")
        │
        ▼
  rbac.yaml: system_roles
        │
        ▼
  rbac.yaml: role_permissions
        │
        ▼
  resolved permission set
        │
        ▼
  rbac.yaml: route_permissions  ←  MatchRoute(method, path)
        │                              first-match glob, ordered
        ▼
  permission required for this route
        │
        ▼
  HasPermission(identity, required) → allow / 403
```

`MatchRoute` uses ordered first-match glob semantics: `*` matches one path segment in non-last position, trailing `*` matches one or more segments. Rules are evaluated top-to-bottom; the first match wins. DELETE rules must appear above GET/POST rules to avoid being shadowed.

`rbac.yaml` is loaded once at startup from `RBAC_CONFIG_PATH`. In Kubernetes it is mounted from a ConfigMap — update the ConfigMap and restart the pod to pick up changes. `deployment/rbac.yaml` is the Helm chart copy and must be kept in sync with the root `rbac.yaml`.

## API key storage

Two backends, selected by `APIKEY_SOURCE`:

| Backend | Storage | Rotation |
|---------|---------|----------|
| `env` | `APIKEY_HASHED_KEYS` env var | Requires redeployment |
| `db` | `system_api_keys` PostgreSQL table | Live — `UPDATE active = FALSE` |

Keys are never stored in plaintext. The store holds SHA-256 hex hashes. At lookup time the raw header value is hashed and compared. The `env` store validates at startup that every hash is exactly 64 lowercase hex characters.

### Database schema

```sql
CREATE TABLE IF NOT EXISTS system_api_keys (
    id          SERIAL PRIMARY KEY,
    system_id   TEXT NOT NULL,
    system_name TEXT NOT NULL DEFAULT '',
    hashed_key  TEXT NOT NULL UNIQUE,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Migration runs automatically at startup via `db.Migrate`.

## Identity forwarding

After successful auth and RBAC the proxy injects two headers into the upstream request:

| Header | Value | Source |
|--------|-------|--------|
| `X-System-ID` | Stable machine identity (JWT `sub`, api key `system_id`) | Auth chain |
| `X-System-Name` | Human-readable label | Auth chain |

These are stripped from the inbound request before auth (middleware) and stripped again from the outbound request before being re-set from context (proxy Rewrite). The double-strip pattern means a client can never supply these headers, even if they slip through the first layer.

## Proxy path rewriting

| Client path | Strip prefix | Upstream path |
|-------------|-------------|---------------|
| `/api/index/api/v1/artifacts` | `/api/index` | `/api/v1/artifacts` |
| `/api/public/index/api/v1/artifacts` | `/api/public/index` | `/api/v1/artifacts?type=streamlit` |

`httputil.ReverseProxy` is used with a `Rewrite` func (not `Director`). CORS headers are stripped from upstream responses — CORS policy is owned by the ingress or the consumer, not the upstream backend.

## Security properties

| Property | Mechanism |
|----------|-----------|
| Identity spoofing | Headers stripped pre-auth and pre-forward (two layers) |
| Public endpoint scope | `type=streamlit` forced in proxy Rewrite, client value overridden |
| Open mode in production | Config validation rejects `open` combined with any other mode at startup |
| Misconfigured key store | Startup validation: 64-char hex required per entry; empty env store rejected |
| Credential vs infra errors | `ErrNotFound` → 401 with hash prefix logged; infra error → 401 with wrapped error logged |
