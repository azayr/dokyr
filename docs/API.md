# Dokyr API reference

This document describes the HTTP API exposed by the Dokyr control plane. It is written for a replacement frontend, mobile app, CLI, or another internal client.

## Conventions

- **Base URL:** the URL where the control plane is installed, for example `https://control.example.com` or `http://127.0.0.1:8080`.
- All API routes are rooted at `/api`.
- JSON request and response bodies use `application/json`.
- The server rejects unknown request-body properties and bodies larger than 1 MiB.
- IDs are opaque strings such as `prj_…`, `svc_…`, `db_…`, and `dep_…`.
- Timestamps are ISO-8601/RFC 3339 values.
- The API has no version prefix yet. Treat field additions as backwards-compatible; do not rely on an exact field order.

### Authentication

Browser clients authenticate with an HTTP-only session cookie set by setup, sign-in, 2FA completion, or GitHub sign-in. Send same-origin requests with credentials enabled:

```js
fetch('/api/projects', { credentials: 'include' });
```

All routes below are authenticated unless marked **public**. An unauthenticated protected request returns `401`:

```json
{ "error": "authentication required" }
```

OAuth start/callback routes redirect the browser and therefore are not normal JSON calls.

### Errors and status codes

All errors have this shape:

```json
{ "error": "Human-readable explanation" }
```

| Status | Meaning |
| --- | --- |
| `200` | Successful read/update/action |
| `201` | Resource created |
| `202` | Asynchronous deployment accepted |
| `400` | Invalid JSON or validation failure |
| `401` | Missing/invalid session or 2FA challenge |
| `404` | Resource/container does not exist |
| `409` | State conflict, duplicate value, or confirmation required |
| `422` | Caddy/TOTP validation rejected the request |
| `502` | Docker, Caddy, Git provider, or SMTP upstream failed |
| `503` | Required platform feature is not configured |

## Common resources

### User

```json
{
  "id": "usr_…",
  "name": "Brahim Oulhaj",
  "email": "you@example.com",
  "role": "owner",
  "twoFactorEnabled": false,
  "githubLogin": "brahimoulhaj",
  "createdAt": "2026-07-19T12:00:00Z"
}
```

### Project

`sourceType` is `empty`, `image`, or `repository`. An empty project has no default application; add independently managed application services instead.

```json
{
  "id": "prj_…",
  "name": "marketing",
  "sourceType": "empty",
  "repository": "",
  "branch": "main",
  "imageUrl": "",
  "registryId": "",
  "connectionId": "",
  "containerPort": 80,
  "domain": "",
  "httpsEnabled": false,
  "status": "created",
  "updatedAt": "2026-07-19T12:00:00Z"
}
```

### Application service

An application service is either an image pull or a Git clone + Docker build. `command`, when supplied, becomes the container command arguments passed to the image entrypoint.

```json
{
  "id": "svc_…",
  "projectId": "prj_…",
  "name": "api",
  "sourceType": "repository",
  "connectionId": "src_…",
  "repository": "acme/api",
  "branch": "main",
  "dockerfilePath": "Dockerfile",
  "buildContext": ".",
  "buildStrategy": "dockerfile",
  "autoDeploy": true,
  "imageUrl": "",
  "containerPort": 8080,
  "command": "serve --port 8080",
  "healthCheckType": "http",
  "healthCheckPath": "/health",
  "healthCheckCommand": "",
  "healthCheckTimeoutSeconds": 60,
  "status": "healthy",
  "container": "selfhost-svc-svc_…",
  "createdAt": "2026-07-19T12:00:00Z",
  "updatedAt": "2026-07-19T12:00:00Z"
}
```

### Domain binding

Each domain has one or more path rules. `serviceId` omitted/empty means the legacy project default application; for an empty project every rule must target an application service.

```json
{
  "id": "dom_…",
  "domain": "example.com",
  "httpsEnabled": true,
  "rules": [
    { "id": 1, "path": "/api/*", "port": 8080, "serviceId": "svc_…" },
    { "id": 2, "path": "/*", "port": 80, "serviceId": "svc_…" }
  ]
}
```

Paths must start with `/`; accepted examples include `/*`, `/api/*`, `/api/**` (normalized to `/api/*`), and `/health`.

### Deployment and event

```json
{
  "id": "dep_…",
  "projectId": "prj_…",
  "serviceId": "svc_…",
  "serviceName": "api",
  "commit": "acme/api@main",
  "message": "Deploy api",
  "status": "deploying",
  "duration": 0,
  "createdAt": "2026-07-19T12:00:00Z"
}
```

```json
{
  "id": 12,
  "deploymentId": "dep_…",
  "stage": "build",
  "type": "log",
  "message": "Step 3/8 …",
  "createdAt": "2026-07-19T12:00:01Z"
}
```

Typical stages are `prepare`, `clone`, `build`, `pull`, `replace`, `create`, `start`, `verify`, `promote`, `rollback`, and `complete`. Event type is generally `start`, `log`, `complete`, or `error`. Poll the deployment detail endpoint while status is `deploying`.

## Public/bootstrap API

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/health` | Service, PostgreSQL, and Docker health |
| `GET` | `/api/setup/status` | Whether the first owner exists |
| `POST` | `/api/setup` | Create the first owner and session |
| `POST` | `/api/auth/login` | Password sign-in |
| `POST` | `/api/auth/2fa` | Complete an outstanding 2FA challenge |
| `POST` | `/api/auth/logout` | Clear session/challenge cookies |
| `GET` | `/api/auth/me` | Current user (protected) |
| `GET` | `/api/auth/providers` | Available account login providers |
| `GET` | `/api/auth/password-reset/status` | Whether password recovery email is available |
| `POST` | `/api/auth/password-reset/request` | Request a reset email |
| `POST` | `/api/auth/password-reset/confirm` | Consume reset token and change password |
| `GET` | `/api/auth/github/start` | Start GitHub account sign-in redirect |
| `GET` | `/api/auth/github/callback` | GitHub sign-in callback |

### `GET /api/health` — public

```json
{
  "ok": true,
  "database": true,
  "docker": {
    "connected": true,
    "version": "28.0.0",
    "containers": 8,
    "running": 7,
    "checkedAt": "2026-07-19T12:00:00Z"
  }
}
```

### `POST /api/setup` — public

```json
{ "name": "Owner", "email": "owner@example.com", "password": "at-least-ten-characters" }
```

Returns `201` with `{ "user": User }` and sets the session cookie. It returns `409` after initial setup has completed.

### `POST /api/auth/login` — public

```json
{ "email": "owner@example.com", "password": "password" }
```

If 2FA is disabled, returns `{ "user": User }` and sets a session cookie. If enabled, returns `{ "requiresTwoFactor": true }` and sets a short-lived 2FA challenge cookie.

### `POST /api/auth/2fa` — public

```json
{ "code": "123456" }
```

Requires the preceding 2FA challenge cookie; returns `{ "user": User }` and establishes the full session.

### Password reset — public

```http
POST /api/auth/password-reset/request
{ "email": "owner@example.com" }
```

Returns `202` with `{ "accepted": true, "message": "…" }` even if the email does not exist. SMTP must be configured and enabled. The link expires after 30 minutes.

```http
POST /api/auth/password-reset/confirm
{ "token": "token from URL", "newPassword": "12-or-more-characters" }
```

Returns `{ "updated": true, "message": "…" }` and clears existing login cookies.

## Account and settings API

| Method | Path | Request body |
| --- | --- | --- |
| `GET` | `/api/account/security` | — |
| `PUT` | `/api/account/password` | password fields |
| `POST` | `/api/account/2fa/setup` | — |
| `POST` | `/api/account/2fa/confirm` | TOTP code |
| `DELETE` | `/api/account/2fa` | password + TOTP code |
| `GET` | `/api/account/github/start` | — (redirect) |
| `DELETE` | `/api/account/github` | — |
| `GET` | `/api/settings/smtp` | — |
| `PUT` | `/api/settings/smtp` | SMTP configuration |
| `POST` | `/api/settings/smtp/test` | optional recipient |

### Security

`GET /api/account/security` returns:

```json
{
  "twoFactorEnabled": true,
  "github": { "linked": true, "login": "brahimoulhaj" },
  "providers": { "github": { "configured": true } }
}
```

`PUT /api/account/password`:

```json
{
  "currentPassword": "old password",
  "newPassword": "at least 12 characters",
  "code": "123456"
}
```

`code` is required only when 2FA is enabled. Success: `{ "updated": true, "message": "Password updated." }`.

`POST /api/account/2fa/setup` returns an unmasked secret and an `otpauth://` URI:

```json
{ "secret": "BASE32", "uri": "otpauth://totp/…" }
```

Do not log or persist either value in frontend analytics. Confirm with `POST /api/account/2fa/confirm`:

```json
{ "code": "123456" }
```

Disable with `DELETE /api/account/2fa`:

```json
{ "password": "current password", "code": "123456" }
```

GitHub account linking uses the redirect returned by `GET /api/account/github/start`; unlink with `DELETE /api/account/github`.

For a managed private GitHub App, the login/link endpoint performs a signed
`GET /app` preflight before returning GitHub's OAuth URL. If GitHub returns 401
or 404 because the App was deleted, the server removes the stale provider
credentials and GitHub installation connections. An authenticated link request
immediately starts a fresh App Manifest flow. An unauthenticated login request
returns to `/login` with instructions to use password login and reconnect the
App in Settings. Other GitHub or network failures do not erase configuration.

### SMTP

`GET /api/settings/smtp` returns settings without the password:

```json
{
  "enabled": true,
  "configured": true,
  "host": "smtp.example.com",
  "port": 587,
  "encryption": "starttls",
  "username": "smtp-user",
  "hasPassword": true,
  "fromName": "Dokyr",
  "fromEmail": "ops@example.com",
  "notifyDeploymentFailures": true,
  "notifyDeploymentSuccesses": false,
  "updatedAt": "2026-07-19T12:00:00Z"
}
```

`PUT /api/settings/smtp` accepts the same fields except `configured`, `hasPassword`, and `updatedAt`:

```json
{
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "encryption": "starttls",
  "username": "smtp-user",
  "password": "new-password-or-empty-to-keep-existing",
  "fromName": "Dokyr",
  "fromEmail": "ops@example.com",
  "notifyDeploymentFailures": true,
  "notifyDeploymentSuccesses": false
}
```

`encryption` is `starttls`, `tls`, or `none`. An empty password preserves the existing saved password. `POST /api/settings/smtp/test` accepts `{ "recipient": "optional@example.com" }`; omit it to send to the current owner.

## Dashboard, projects, and domains

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/dashboard` | Projects, recent deployments, Docker health |
| `GET` | `/api/projects` | All projects |
| `POST` | `/api/projects` | Create project |
| `GET` | `/api/projects/{projectId}` | Full project detail |
| `PUT` | `/api/projects/{projectId}` | Update default/legacy project workload |
| `DELETE` | `/api/projects/{projectId}` | Delete project with confirmation |
| `PUT` | `/api/projects/{projectId}/domain` | Replace all domain bindings |
| `POST` | `/api/projects/{projectId}/deploy` | Deploy default/legacy application |
| `GET` | `/api/projects/{projectId}/logs?lines=300` | Default/legacy runtime logs |
| `GET` | `/api/projects/{projectId}/metrics` | Per-project container metrics |

### Create project

```http
POST /api/projects
```

```json
{
  "name": "My project",
  "sourceType": "empty",
  "repository": "",
  "branch": "main",
  "connectionId": "",
  "imageUrl": "",
  "registryId": "",
  "containerPort": 80,
  "domain": "",
  "httpsEnabled": false
}
```

Use `sourceType: "empty"` to create a service-oriented project. It cannot have an initial domain. For `image`, provide `imageUrl` and optionally `registryId`; for the legacy `repository` project type, provide `repository`, optional `connectionId`, and a branch. A repository default project can be saved but should use application services for the current Git build workflow.

Returns `201` with `Project`.

### Project detail

`GET /api/projects/{projectId}` returns:

```json
{
  "project": { "…": "Project" },
  "deployments": ["Deployment"],
  "services": [{ "name": "…", "image": "…", "status": "healthy", "container": "…" }],
  "applicationServices": ["Application service"],
  "databaseServices": ["Database service"],
  "ingressRules": [{ "id": 1, "path": "/api/*", "port": 8080 }],
  "ingressDefaultPath": "/*",
  "domainBindings": ["Domain binding"]
}
```

### Update default/legacy project workload

`PUT /api/projects/{projectId}` accepts the same workload fields as project creation except `domain` and `httpsEnabled`:

```json
{
  "name": "My project",
  "sourceType": "image",
  "imageUrl": "nginx:alpine",
  "registryId": "",
  "repository": "",
  "branch": "main",
  "connectionId": "",
  "containerPort": 80
}
```

Returns `{ "project": Project, "message": "…" }`. This endpoint is primarily for backwards-compatible default applications; independent application services are the recommended model.

### Delete project

```http
DELETE /api/projects/{projectId}
{ "confirmation": "exact project name", "removeVolumes": false }
```

`confirmation` must match `project.name`. `removeVolumes: true` removes managed database data volumes and is irreversible. Successful response includes `{ "removed": true, "volumesRemoved": false }`.

### Domain bindings and Caddy routing

`PUT /api/projects/{projectId}/domain` atomically replaces all bindings, updates Caddy, and restores the previous routing if Caddy rejects the configuration.

```json
{
  "domains": [
    {
      "domain": "example.com",
      "httpsEnabled": true,
      "rules": [
        { "path": "/api/**", "port": 8080, "serviceId": "svc_api" },
        { "path": "/*", "port": 80, "serviceId": "svc_front" }
      ]
    },
    {
      "domain": "www.example.com",
      "httpsEnabled": true,
      "rules": [{ "path": "/*", "port": 80, "serviceId": "svc_front" }]
    }
  ]
}
```

Up to 25 domains and 50 path rules per domain are supported. Rules must target an application service belonging to the project when `serviceId` is supplied. The response contains `{ "project": Project, "active": true, "domainBindings": [...] }`.

For backwards compatibility, a single-domain shape (`domain`, `httpsEnabled`, `defaultPort`, `defaultPath`, `rules`) is also accepted.

### Deploy default project

`POST /api/projects/{projectId}/deploy` starts an image deployment and returns `202` with `{ "project": Project, "deployment": Deployment }`. It is unavailable for the legacy repository project type. Use `POST /api/services/{serviceId}/deploy` for Git services.

### Logs and metrics

`GET /api/projects/{projectId}/logs?lines=300` returns:

```json
{ "lines": ["…"], "count": 17, "limit": 300, "container": "selfhost-prj_…" }
```

`lines` is optional, default `300`, range `1–1000`. The metrics endpoint returns Docker measurements for the project’s containers; values are host/Docker dependent. Poll it rather than assuming a fixed update interval.

## Environment variables

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/projects/{projectId}/environment` | Default project environment |
| `PUT` | `/api/projects/{projectId}/environment` | Save and restart default project container |
| `GET` | `/api/services/{serviceId}/environment` | Service environment |
| `PUT` | `/api/services/{serviceId}/environment` | Save and restart application service |

Request body:

```json
{
  "variables": [
    { "key": "APP_ENV", "value": "production", "secret": false },
    { "key": "DATABASE_URL", "value": "postgresql://…", "secret": true }
  ]
}
```

Keys use shell-style identifier rules: letters/underscore first, then letters/numbers/underscore; they must be unique. Values may be up to 16 KiB. The API stores them encrypted; `secret` affects metadata/masking but the current authenticated caller receives the value from the API.

Successful application-service update returns `{ "variables": [...], "service": Service, "restarted": true, "message": "…" }`. A service that has never been deployed saves variables with `restarted: false`; the first deployment applies them. The default project endpoint returns `{ "variables": [...], "service": Service, "message": "…" }` and restarts without rebuilding.

## Application services

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/projects/{projectId}/services` | Create an application service |
| `PUT` | `/api/services/{serviceId}` | Update its source/runtime definition |
| `POST` | `/api/services/{serviceId}/deploy` | Start async deployment |
| `GET` | `/api/services/{serviceId}/deployment-triggers` | Read Git/registry deployment automation |
| `PUT` | `/api/services/{serviceId}/deployment-triggers` | Enable or update deployment automation |
| `GET` | `/api/services/{serviceId}/logs?lines=300` | Runtime logs |
| `DELETE` | `/api/services/{serviceId}` | Remove container and service record |

### Create or update

Image service:

```json
{
  "name": "adminer",
  "sourceType": "image",
  "imageUrl": "adminer:latest",
  "registryId": "optional-registry-id",
  "containerPort": 8080,
  "command": "",
  "healthCheckType": "http",
  "healthCheckPath": "/",
  "healthCheckTimeoutSeconds": 60,
  "environment": "ADMINER_DEFAULT_SERVER=selfhost-db-db_…"
}
```

Git service:

```json
{
  "name": "api",
  "sourceType": "repository",
  "connectionId": "src_…",
  "repository": "owner/repository",
  "branch": "main",
  "dockerfilePath": "Dockerfile",
  "buildContext": ".",
  "buildStrategy": "dockerfile",
  "containerPort": 8080,
  "command": "serve --port 8080",
  "healthCheckType": "command",
  "healthCheckCommand": "wget -qO- http://127.0.0.1:8080/health",
  "healthCheckTimeoutSeconds": 60,
  "environment": "APP_ENV=production\nLOG_LEVEL=info"
}
```

`environment` is a newline-separated `KEY=value` string on service creation. Later use the structured environment endpoint. `healthCheckType` is `none`, `http`, or `command`. HTTP checks require an absolute `healthCheckPath`; command checks require `healthCheckCommand`. `healthCheckTimeoutSeconds` accepts 5–600 seconds and defaults to 60. With `none`, promotion verifies that the candidate container remains running.

Git repositories must be visible to the selected GitHub/GitLab connection. `buildStrategy` is `dockerfile` (default), `railpack`, or `nixpacks`. Dockerfile builds require `dockerfilePath` and `buildContext`, both of which must remain inside the repository; absolute paths and parent traversal are rejected. Railpack and Nixpacks inspect the repository and do not require a Dockerfile.

Nixpacks uses the control plane's existing Docker socket. Railpack requires a BuildKit daemon; set `SELFHOST_BUILDKIT_HOST` to its reachable `tcp://host:port` endpoint before selecting Railpack. This is intentionally opt-in so the control-plane compose stack does not introduce a privileged builder daemon automatically.

`POST` returns `201` with `{ "service": ApplicationService }`. `PUT` returns `{ "service": ApplicationService, "message": "Service configuration saved; deploy the service to apply it" }`.

### Deploy a service

`POST /api/services/{serviceId}/deploy` returns `202` immediately:

```json
{ "service": { "…": "Application service" }, "deployment": { "…": "Deployment" } }
```

For an image it pulls the registry image. For a repository it obtains a short-lived provider token, clones the selected branch, and builds with the configured strategy. The runtime then creates a uniquely named candidate container while the stable container remains online. After the candidate passes its HTTP, command, or running-state check, it receives the stable private hostname and the previous container is retired. If the candidate fails, it is removed and the stable release remains active. Poll `GET /api/deployments/{deploymentId}` for live event output.

### Automatic deployment triggers

Repository services can automatically deploy when GitHub sends a signed push event for the configured repository and branch. Image services get a private, per-service webhook URL that can be called by Docker Hub, GHCR, GitLab Container Registry, or another compatible registry after an image push.

Read the current configuration:

```http
GET /api/services/{serviceId}/deployment-triggers
```

Repository response:

```json
{
  "serviceId": "svc_…",
  "sourceType": "repository",
  "autoDeploy": true,
  "branch": "main",
  "registryWebhookEnabled": false,
  "registryWebhookTag": "",
  "webhookConfigured": true,
  "webhookUrl": "https://control.example.com/api/webhooks/github"
}
```

Enable or disable repository auto-deploy:

```json
{ "autoDeploy": true, "registryWebhookEnabled": false, "registryWebhookTag": "" }
```

GitHub verifies webhook calls with the secret generated by the GitHub App manifest. Existing GitHub Apps created before this feature return `webhookConfigured: false` and must be reconnected once so GitHub registers the push webhook. The API rejects enabling `autoDeploy` until this is done. A push only selects services whose repository name and branch match the signed payload. Duplicate delivery IDs are ignored.

Enable an image webhook and optionally restrict it to one tag:

```json
{ "autoDeploy": false, "registryWebhookEnabled": true, "registryWebhookTag": "stable" }
```

The response includes a URL shaped like:

```text
https://control.example.com/api/webhooks/registry/svc_…/private-token
```

Configure the registry to send an HTTP `POST` to that URL. The endpoint is public because registries cannot use the control-plane session cookie; possession of the unguessable URL token authorizes the request. Do not log, publish, or put this URL in frontend analytics. Disabling and re-enabling a registry webhook creates a new token and invalidates the previous URL.

If `registryWebhookTag` is not empty, the JSON payload must contain that value in a `tag`, `tag_name`, or `ref` field. Empty tag accepts any image-push payload. A successful webhook returns `202`, for example:

```json
{ "triggered": true, "deployment": "dep_…" }
```

Automatic deployments use the same zero-downtime execution path as the manual deploy button: Git services clone and rebuild; image services pull the configured image; both create, verify, and promote a candidate container. If the service is already deploying, the event is accepted but ignored.

### Service logs and removal

`GET /api/services/{serviceId}/logs?lines=300` has the same response shape and `lines` limits as project logs, but uses the service container.

`DELETE /api/services/{serviceId}` has no body and returns `{ "ok": true }`. It returns `409` until every domain route targeting the service is removed.

## Database services

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/projects/{projectId}/databases` | Create and deploy database |
| `GET` | `/api/databases/{databaseId}/credentials` | Reveal connection credentials |
| `GET` | `/api/databases/{databaseId}/events` | Database deployment events |
| `GET` | `/api/databases/{databaseId}/logs?lines=300` | Runtime logs |
| `PUT` | `/api/databases/{databaseId}/exposure` | Change public port exposure |
| `DELETE` | `/api/databases/{databaseId}` | Remove database |

### Create a database

```json
{
  "name": "Primary database",
  "engine": "postgres",
  "databaseName": "app",
  "username": "app",
  "password": "optional-12-or-more-character-password",
  "publicEnabled": false,
  "publicPort": 5432
}
```

`engine` is `postgres`, `mysql`, or `mariadb`. If password is omitted, the server generates one. Private access is the default. A public port is only used when `publicEnabled` is true; it defaults to the engine port.

Returns `201`:

```json
{
  "service": { "…": "Database service" },
  "credentials": {
    "username": "app",
    "password": "revealed password",
    "database": "app",
    "host": "selfhost-db-db_…",
    "port": 5432,
    "connectionUrl": "postgresql://…",
    "publicEnabled": false,
    "publicPort": 0
  }
}
```

### Database operations

`GET /api/databases/{databaseId}/credentials` returns the same `credentials` object. Treat this endpoint as sensitive: never cache its response in shared client storage.

`GET /api/databases/{databaseId}/events` returns `{ "events": [{ "id": 1, "stage": "pull", "type": "log", "message": "…", "createdAt": "…" }] }`.

`PUT /api/databases/{databaseId}/exposure`:

```json
{ "enabled": true, "port": 5432 }
```

The database is recreated with the requested host port; the previous exposure is restored if Docker fails. Returns `{ "service": DatabaseService }`.

`DELETE /api/databases/{databaseId}`:

```json
{ "confirmation": "exact database service name", "removeVolume": false }
```

Returns `{ "removed": true, "volumeRemoved": false, "retainedVolume": "selfhost-data-db_…" }`. Setting `removeVolume: true` irreversibly removes the database files.

## Deployments

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/deployments` | All deployments, newest first |
| `GET` | `/api/deployments/{deploymentId}` | Deployment, project, and events |

`GET /api/deployments/{deploymentId}` response:

```json
{
  "deployment": { "…": "Deployment" },
  "project": { "…": "Project" },
  "events": [{ "…": "Deployment event" }]
}
```

A frontend can poll every 1–3 seconds while `deployment.status === "deploying"`, then stop when it becomes `healthy`, `degraded`, or `failed`.

## Sources and registries

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/integrations` | Provider state, connected accounts, registries |
| `GET` | `/api/integrations/github/install/start` | Start GitHub App install redirect |
| `POST` | `/api/integrations/github/installations/sync` | Recover an already-installed personal GitHub App installation |
| `GET` | `/api/integrations/oauth/{provider}/start` | Start GitLab OAuth redirect |
| `GET` | `/api/integrations/sources/{sourceId}/repositories` | Repositories allowed for source |
| `DELETE` | `/api/integrations/sources/{sourceId}` | Unlink a Git source; running containers remain untouched |
| `POST` | `/api/integrations/registries` | Add private Docker registry |
| `DELETE` | `/api/integrations/registries/{registryId}` | Delete registry credential |

GitHub integration uses the GitHub App installation flow. The returned installation has a `manageUrl`; use this to change repository selection in GitHub. GitLab currently uses OAuth and requires provider configuration on the server.

If GitHub shows the App as installed but `GET /api/integrations` has no GitHub source connection, call `POST /api/integrations/github/installations/sync`. It lists installations using the server's private GitHub App credential and imports only an installation whose GitHub account ID exactly matches the account already linked to the authenticated Dokyr user. It never imports an unrelated personal or organization installation.

```json
{
  "synced": 1,
  "connections": [{ "id": "src_…", "provider": "github", "accountName": "octocat", "installationId": 123, "contentsPermission": "read" }],
  "warning": "",
  "message": "GitHub repository access synchronized."
}
```

An installation created with an older App manifest may return a warning and `contentsPermission: ""`. Repository discovery can still succeed because Metadata access exposes repository names, but private clone/deploy requires the App owner to enable read-only Contents permission and approve the GitHub permission update. Dokyr validates the permissions returned with the installation token before cloning and reports an actionable error instead of GitHub's misleading `Repository not found` response.

`GET /api/integrations` returns:

```json
{
  "providers": { "github": { "configured": true, "linked": true, "login": "…" } },
  "connections": [{ "id": "src_…", "provider": "github", "accountName": "…", "manageUrl": "https://github.com/settings/installations/…" }],
  "registries": [{ "id": "reg_…", "name": "GHCR", "registryUrl": "ghcr.io", "username": "…" }]
}
```

`GET /api/integrations/sources/{sourceId}/repositories` returns:

```json
{
  "connection": { "…": "Source connection" },
  "repositories": [
    { "id": "1", "name": "api", "fullName": "acme/api", "cloneUrl": "https://…", "defaultBranch": "main", "private": true, "updatedAt": "…" }
  ]
}
```

Add a registry:

```json
{
  "name": "GitHub Container Registry",
  "registryUrl": "ghcr.io",
  "username": "octocat",
  "password": "token"
}
```

Returns `201` with `{ "registry": RegistryCredential }`. `registryUrl` must be a host, not a URL scheme. Deletion returns `{ "ok": true }` and is rejected when a project/service still references it.

## Caddy proxy API

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/caddy/config` | Managed routes plus rendered Caddy configuration |
| `PUT` | `/api/caddy/config` | Apply raw Caddy configuration via Caddy Admin API |
| `POST` | `/api/caddy/reset` | Restore generated managed routes |

`GET /api/caddy/config`:

```json
{
  "connected": true,
  "connectionError": "",
  "mode": "managed",
  "routes": [{ "domain": "example.com", "https": true, "paths": [{ "path": "/*", "upstream": "selfhost-svc-svc_…:80" }] }],
  "configuration": "{\n…"
}
```

`PUT /api/caddy/config` accepts `{ "configuration": "full Caddyfile content" }`; maximum size is 256 KiB. Raw configuration can override managed behavior, so expose this only to trusted administrators. `POST /api/caddy/reset` reapplies routes generated from saved domain bindings.

## Infrastructure API

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/infrastructure/metrics` | Global Docker/host metrics |
| `GET` | `/api/infrastructure/cleanup` | Cleanup preview |
| `POST` | `/api/infrastructure/cleanup` | Run selected Docker cleanup |

Metrics are global to the Docker host. Per-project metrics belong at `/api/projects/{projectId}/metrics`.

Cleanup preview returns Docker’s reclaimable-resource information. To perform cleanup:

```json
{
  "containers": true,
  "images": true,
  "buildCache": true,
  "networks": false,
  "volumes": false,
  "confirmation": "CLEAN DOCKER"
}
```

Select at least one category and use the exact confirmation text. Volumes can contain persistent database data; a frontend should make this option visually distinct and require an additional confirmation. Successful responses return Docker’s deleted resources and reclaimed-space result.

## Frontend implementation notes

1. Keep credentials enabled for every same-origin API call; authentication is cookie-based, not bearer-token based.
2. Send only documented request fields. The server intentionally rejects unknown properties.
3. Treat deployment endpoints as async. Navigate to deployment detail or poll it after receiving `202`.
4. Runtime logs are snapshots, not WebSockets. Poll `?lines=` as desired and preserve the user-selected line limit.
5. Never persist SMTP passwords, database credentials, Git tokens, TOTP secrets, or decoded environment secrets in local storage, analytics, or error reporting.
6. When rendering Caddy route editors, allow multiple domains and ordered path rules, including multiple paths pointing to one service.
7. Do not automatically expose databases publicly. `publicEnabled` must remain an explicit user action.

## Source of truth

The route registrations and handlers live in [internal/api/api.go](../internal/api/api.go). The JSON resource definitions live in [internal/store/store.go](../internal/store/store.go). Update this document whenever either changes.
