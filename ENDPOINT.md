# Endpoint reference

All paths are relative to the service root. The BFF proxies `/api/index/*` and `/api/public/index/*` to `fusion-index`; the prefix is stripped before forwarding.

---

## Infrastructure

No authentication. Not proxied.

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `GET` | `/health` | `200` | Basic health check |
| `GET` | `/livez` | `200` | Kubernetes liveness probe |
| `GET` | `/readyz` | `200` | Kubernetes readiness probe |

---

## Public artifact API — `/api/public/index/*`

No authentication required. Read-only (`GET` only — all other methods return `404`).

Two layers of restriction apply:

1. **Type filter** — `?type=<PUBLIC_TYPE>` is forced on every upstream request. Any `?type=` value supplied by the client is overridden. Default type is `streamlit`.
2. **Tag gate** — when `PUBLIC_DOWNLOAD_TAG` is set, any path that targets a specific version (`/artifacts/{id}/versions/{semver}/...`) is blocked with `403` unless that version carries the configured tag. The BFF makes a pre-flight `GET /api/v1/artifacts/{id}/versions/{semver}` to fusion-index before proxying.

Identity headers (`X-System-ID`, `X-System-Name`) are stripped from all inbound public requests and never forwarded upstream.

### Artifacts

| Method | Path | Tag gate | Description |
|--------|------|----------|-------------|
| `GET` | `/api/public/index/api/v1/artifacts` | No | List artifacts — `?type=<PUBLIC_TYPE>` forced; supports `?name=` prefix filter, `?page=`, `?pageSize=` |
| `GET` | `/api/public/index/api/v1/artifacts/{id}` | No | Get artifact metadata by ID |

### Versions

| Method | Path | Tag gate | Description |
|--------|------|----------|-------------|
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions` | Yes | List versions of an artifact |
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions/{semver}` | Yes | Get version metadata; response includes `tags[]` |

### Files

| Method | Path | Tag gate | Description |
|--------|------|----------|-------------|
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions/{semver}/files` | Yes | List files in a version |
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}` | Yes | Get file metadata |
| `GET` | `/api/public/index/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}/download` | Yes | Download file content |

### Tag gate error responses

| Status | Condition |
|--------|-----------|
| `403 Forbidden` | Version exists but does not carry the required tag |
| `404 Not Found` | Artifact or version does not exist |
| `503 Service Unavailable` | Pre-flight call to fusion-index failed (network error) |
| `502 Bad Gateway` | Pre-flight call returned an unexpected status or unreadable body |

---

## Authenticated artifact API — `/api/index/*`

All requests require a valid credential (see [SETTINGS.md](SETTINGS.md) for auth modes). RBAC is enforced per route based on `rbac.yaml`. On success the resolved identity is forwarded as `X-System-ID` / `X-System-Name`.

### Error responses (auth layer)

| Status | Condition |
|--------|-----------|
| `401 Unauthorized` | No credential, unknown API key, or invalid JWT |
| `403 Forbidden` | Authenticated but identity lacks the required permission |

### Artifacts

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| `GET` | `/api/index/api/v1/artifacts` | `index:artifacts:read` | List artifacts; supports `?name=`, `?type=`, `?tag=`, `?page=`, `?pageSize=` |
| `POST` | `/api/index/api/v1/artifacts` | `index:artifacts:write` | Create artifact; body: `{"fullName":"org.team.name","description":"..."}` |
| `GET` | `/api/index/api/v1/artifacts/{id}` | `index:artifacts:read` | Get artifact by ID |
| `PUT` | `/api/index/api/v1/artifacts/{id}` | `index:artifacts:write` | Update artifact description |
| `DELETE` | `/api/index/api/v1/artifacts/{id}` | `index:artifacts:delete` | Delete artifact and all versions |

### Versions

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| `GET` | `/api/index/api/v1/artifacts/{id}/versions` | `index:artifacts:read` | List versions |
| `POST` | `/api/index/api/v1/artifacts/{id}/versions` | `index:artifacts:write` | Create version; body: `{"version":"1.2.3","config":"...","tags":["stable"]}` |
| `GET` | `/api/index/api/v1/artifacts/{id}/versions/{semver}` | `index:artifacts:read` | Get version metadata including tags |
| `DELETE` | `/api/index/api/v1/artifacts/{id}/versions/{semver}` | `index:artifacts:delete` | Delete version and its files |

### Tags

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| `PUT` | `/api/index/api/v1/artifacts/{id}/tags/{tag}` | `index:artifacts:write` | Assign or move tag to a version; body: `{"version":"1.2.3"}` |
| `DELETE` | `/api/index/api/v1/artifacts/{id}/tags/{tag}` | `index:artifacts:delete` | Remove tag |

### Files

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| `GET` | `/api/index/api/v1/artifacts/{id}/versions/{semver}/files` | `index:artifacts:read` | List files in a version |
| `POST` | `/api/index/api/v1/artifacts/{id}/versions/{semver}/files` | `index:artifacts:write` | Upload file (multipart/form-data) |
| `GET` | `/api/index/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}` | `index:artifacts:read` | Get file metadata |
| `GET` | `/api/index/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}/download` | `index:artifacts:read` | Download file content |
| `DELETE` | `/api/index/api/v1/artifacts/{id}/versions/{semver}/files/{fileId}` | `index:artifacts:delete` | Delete file |

### Pagination

All list endpoints support:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `0` | Zero-based page index |
| `pageSize` | `20` | Items per page |

Response shape: `{"items":[...],"total":N,"page":0,"pageSize":20}`

---

## Headers

### Inbound (client → BFF)

| Header | Routes | Description |
|--------|--------|-------------|
| `Authorization: Bearer <jwt>` | `/api/index/*` | JWT for `oauth2` auth mode |
| `X-Api-Key: <key>` | `/api/index/*` | Raw API key for `apikey` auth mode (header name configurable via `APIKEY_HEADER`) |
| `X-Request-ID` | all | Passed through if present; generated if absent |

### Outbound (BFF → fusion-index)

| Header | Set by | Description |
|--------|--------|-------------|
| `X-System-ID` | BFF | Resolved system identity — stripped from client request first |
| `X-System-Name` | BFF | Human-readable system label — stripped from client request first |
| `X-Request-ID` | BFF | Correlation ID |
