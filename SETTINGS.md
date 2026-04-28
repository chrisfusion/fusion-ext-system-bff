# Settings reference

All configuration is read from environment variables at startup. Invalid or missing required values cause the process to exit immediately with a descriptive error.

---

## Server

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | TCP port the HTTP server listens on |

---

## Auth

### `AUTH_MODES`

**Required.** Comma-separated ordered list of enabled authenticators.

```
AUTH_MODES=apikey
AUTH_MODES=oauth2
AUTH_MODES=open
```

Valid values: `oauth2`, `apikey`, `open`. `open` cannot be combined with other modes — startup will reject the configuration.

The chain is tried in the order listed. The first authenticator that returns a successful identity wins. If an authenticator finds its credential type present but invalid (bad JWT signature, unknown API key), the chain stops and the request is rejected 401 — it does not fall through to the next authenticator.

---

### OAuth2 mode (`AUTH_MODES=oauth2`)

| Variable | Default | Description |
|----------|---------|-------------|
| `OIDC_ISSUER_URL` | — | **Required.** OIDC provider issuer URL (e.g. `https://keycloak.example.com/realms/fusion`) |
| `OIDC_CLIENT_ID` | — | **Required.** Expected `aud` claim in the JWT |
| `OIDC_JWKS_URL` | `{OIDC_ISSUER_URL}/protocol/openid-connect/certs` | Override JWKS endpoint URL |
| `OIDC_JWKS_CACHE_TTL` | `15m` | How long to cache the JWKS key set before re-fetching |

The `sub` claim from the validated JWT is used as the system identity. The `name` claim is used as the system name (falls back to `sub` if absent).

**Request format:**
```
Authorization: Bearer <access_token>
```

---

### API key mode (`AUTH_MODES=apikey`)

| Variable | Default | Description |
|----------|---------|-------------|
| `APIKEY_HEADER` | `X-Api-Key` | Header name to read the raw API key from |
| `APIKEY_SOURCE` | `env` | Key storage backend: `env` or `db` |

The raw header value is hashed with SHA-256 and compared against the store. The `system_id` bound to the key becomes the system identity.

**Request format:**
```
X-Api-Key: <raw-key>
```

#### `APIKEY_SOURCE=env`

| Variable | Default | Description |
|----------|---------|-------------|
| `APIKEY_HASHED_KEYS` | — | **Required.** Comma-separated entries in `sha256hex:system-id:system-name` format |

Each hash must be exactly 64 lowercase hex characters (validated at startup). Suitable for local dev and simple deploys. **Key rotation requires a redeployment.**

```bash
# Generate a hash
printf '%s' 'my-raw-key' | sha256sum | cut -d' ' -f1

# Example value
APIKEY_HASHED_KEYS=a3f1...64chars...:ci-pipeline:CI Pipeline,b9c2...64chars...:data-reader:Data Reader
```

#### `APIKEY_SOURCE=db`

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_DSN` | — | **Required.** PostgreSQL connection string (e.g. `postgres://user:pass@host:5432/db`) |

Keys are stored in the `system_api_keys` table (created automatically on first startup). **Keys can be added or revoked without restarting the service.**

```sql
-- Add a key
INSERT INTO system_api_keys (system_id, system_name, hashed_key)
VALUES ('ci-pipeline', 'CI Pipeline', '<sha256hex>');

-- Revoke a key (takes effect on the next request — no restart needed)
UPDATE system_api_keys SET active = FALSE WHERE hashed_key = '<sha256hex>';

-- List active keys
SELECT system_id, system_name, created_at FROM system_api_keys WHERE active = TRUE;
```

---

### Open mode (`AUTH_MODES=open`)

| Variable | Default | Description |
|----------|---------|-------------|
| `OPEN_SYSTEM_ID` | `anonymous` | Identity assigned to all requests |

No credential is required. Every request is authenticated as the configured system identity. **Never use in production.** Cannot be combined with `oauth2` or `apikey`.

---

## Public API

The public API is available at `/api/public/index/*` with **no authentication required**. It is intended for Streamlit apps and other consumers that need read access to published Streamlit artifacts without managing credentials.

### How it works

The BFF forces `?type=streamlit` into every upstream request, overriding any value the client supplies. It is structurally impossible to use this endpoint to read artifacts of any other type.

Identity headers (`X-System-ID`, `X-System-Name`) are stripped from inbound requests and are never forwarded upstream — public requests are anonymous to the backend.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/public/index/api/v1/artifacts` | List Streamlit artifacts |
| `GET` | `/api/public/index/api/v1/artifacts/{id}` | Get a Streamlit artifact |
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions` | List versions |
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions/{v}/files` | List files in a version |
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions/{v}/files/{name}` | Download a file |

POST, PUT, and DELETE are not registered on `/api/public/*` — they return 404.

### Example

```bash
# List all Streamlit artifacts — no credential needed
curl https://ext-bff.example.com/api/public/index/api/v1/artifacts

# ?type= is always overridden to streamlit even if supplied
curl "https://ext-bff.example.com/api/public/index/api/v1/artifacts?type=maven"
# → returns only streamlit artifacts
```

---

## Upstream proxy

| Variable | Default | Description |
|----------|---------|-------------|
| `INDEX_URL` | `http://fusion-index-backend.fusion.svc.cluster.local:8080` | Base URL for the fusion-index backend |

All `/api/index/*` and `/api/public/index/*` traffic is forwarded to this URL. The `/api/index` and `/api/public/index` path prefixes are stripped before forwarding.

---

## RBAC

| Variable | Default | Description |
|----------|---------|-------------|
| `RBAC_CONFIG_PATH` | `./rbac.yaml` | Path to the RBAC configuration file |

In Kubernetes, mount `rbac.yaml` as a ConfigMap volume and set this variable to the mount path. The file is read once at startup; restart the pod to apply changes.

### rbac.yaml structure

```yaml
# Maps system identity → roles
system_roles:
  ci-pipeline: [writer]
  data-reader:  [reader]
  admin-tool:   [admin]

# Maps role → permission list
role_permissions:
  reader:
    - index:artifacts:read
  writer:
    - index:artifacts:read
    - index:artifacts:write
  admin:
    - index:artifacts:read
    - index:artifacts:write
    - index:artifacts:delete

# Maps HTTP method + path pattern → required permission
# First match wins. * = one segment, trailing * = one or more segments.
route_permissions:
  - { method: DELETE, path: /api/index/api/v1/artifacts/*/versions/*/files/*, permission: index:artifacts:delete }
  - { method: DELETE, path: /api/index/api/v1/artifacts/*/versions/*,         permission: index:artifacts:delete }
  - { method: DELETE, path: /api/index/api/v1/artifacts/*,                    permission: index:artifacts:delete }
  - { method: POST,   path: /api/index/api/v1/artifacts/*/versions/*/files,   permission: index:artifacts:write }
  - { method: POST,   path: /api/index/api/v1/artifacts/*/versions,            permission: index:artifacts:write }
  - { method: POST,   path: /api/index/api/v1/artifacts,                       permission: index:artifacts:write }
  - { method: GET,    path: /api/index/*,                                       permission: index:artifacts:read }
```

The `deployment/rbac.yaml` inside the Helm chart must always be kept in sync with the root `rbac.yaml`.

---

## Helm chart values

The Helm chart (`deployment/`) exposes all settings as values. Key overrides:

```bash
# Production — apikey with DB rotation
helm install fusion-ext-system-bff deployment/ \
  --namespace fusion \
  --set config.authModes=apikey \
  --set config.apikeySource=db \
  --set secret.dbDsn="postgres://user:pass@host:5432/db"

# Production — oauth2
helm install fusion-ext-system-bff deployment/ \
  --namespace fusion \
  --set config.authModes=oauth2 \
  --set config.oidcIssuerUrl=https://keycloak.example.com/realms/fusion \
  --set config.oidcClientId=fusion-ext

# Local dev — open mode
helm install fusion-ext-system-bff deployment/ \
  --namespace fusion \
  --set image.repository=fusion-ext-system-bff \
  --set image.tag=local \
  --set image.pullPolicy=Never \
  --set config.authModes=open
```

See `deployment/values.yaml` for the full list of available values.
