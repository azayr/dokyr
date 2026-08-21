---
title: HTTP API reference
description: Integrate with Dokyr's authenticated HTTP API for projects, services, deployments, domains, users, registry, storage, mail, and operations.
---

# Dokyr API reference

This document describes the HTTP API exposed by the Dokyr control plane. It is written for a replacement frontend, mobile app, CLI, or another internal client.

## Conventions

- **Base URL:** the URL where the control plane is installed, for example `https://control.example.com` or `http://127.0.0.1:8888`.
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

GitHub login and account linking use a public, identity-only GitHub App created
for the individual Dokyr server through GitHub's App Manifest flow. When it is
not configured, an authenticated request to `GET /api/account/github/start`
creates it automatically using `DOKYR_PUBLIC_URL` for its manifest and
callback URLs, then continues into account authorization. No GitHub client
credentials or callback domain are configured manually.

This authorization requests no repository permissions and is used only to read
the authenticated user's public GitHub identity. It does not create a source
connection or grant repository access. Repository access uses the separate,
public GitHub App installation flow described under Sources and registries.

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

## Developer mail gateway

Authenticated workspace endpoints:

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/mail` | Current user's domains, credentials, recent messages, and connection readiness |
| `PUT` | `/api/mail/setup` | One-time setup with `{ "domain": "example.com" }`; configures `mail.example.com` |
| `POST` | `/api/mail/domains` | Add `{ "name": "example.com" }` and generate an ownership record |
| `POST` | `/api/mail/domains/{id}/verify` | Check public DNS and provision the owned domain in Stalwart |
| `DELETE` | `/api/mail/domains/{id}` | Remove the domain, its scoped keys, and Stalwart domain |
| `POST` | `/api/mail/api-keys` | Create `{ "name": "Production", "domainId": "mdom_…" }` |
| `DELETE` | `/api/mail/api-keys/{id}` | Revoke a sending key |

The secret and generated `smtpUsername` are returned only by the create
response. The same secret works as an HTTP bearer token and as the SMTP
password. Send through the public developer endpoint with:

```bash
curl -X POST 'https://console.example.com/v1/emails' \
  -H 'Authorization: Bearer dkr_mail_YOUR_FULL_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "from": "Dokyr Test <hello@updates.example.com>",
    "to": ["developer@example.com"],
    "subject": "Hello from Dokyr",
    "html": "<strong>Your first Dokyr email works.</strong>",
    "text": "Your first Dokyr email works."
  }'
```

The same request without a shell wrapper is:

```http
POST /v1/emails
Authorization: Bearer dkr_mail_...
Content-Type: application/json
```

```json
{
  "from": "Dokyr <deployments@example.com>",
  "to": ["developer@company.com"],
  "subject": "Deployment complete",
  "html": "<p>Your deployment is ready.</p>",
  "text": "Your deployment is ready.",
  "replyTo": "support@example.com"
}
```

The `from` address must belong to the verified domain attached to the key. A
request accepts 1–50 recipients and at least one of `html` or `text`. Dokyr
returns `{ "id": "mail_…", "status": "queued" }` after Stalwart accepts every
recipient. Queued is not confirmation of final recipient delivery.

For SMTP submission, use the public mail hostname configured during first-time
setup, implicit TLS on port 465, the returned `smtpUsername`, and the same
one-time secret as the password:

```dotenv
SMTP_HOST=mail.example.com
SMTP_PORT=465
SMTP_SECURE=true
SMTP_USERNAME=smtp-generated-id@updates.example.com
SMTP_PASSWORD=dkr_mail_YOUR_FULL_API_KEY
SMTP_FROM=hello@updates.example.com
```

An SMTP credential is scoped to its verified domain. Credentials created
before SMTP credential provisioning was introduced have no `smtpUsername` and
must be recreated for SMTP use.

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
| `GET` | `/api/domains` | Read the global domain-management workspace and Caddy state |
| `POST` | `/api/projects/{projectId}/deploy` | Deploy default/legacy application |
| `POST` | `/api/projects/{projectId}/stop` | Stop default/legacy application |
| `POST` | `/api/projects/{projectId}/restart` | Restart or start default/legacy application |
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

`GET /api/domains` returns every project that can own domain bindings, its
available reverse-proxy service targets, saved bindings, the effective
container-registry domain (including the `REGISTRY_HOSTS` fallback), Caddy
control-plane hosts, and the current generated Caddy configuration. This is the
aggregate endpoint used by the global **Domains** screen; it avoids inspecting
project containers simply to build the routing editor.

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

`POST /api/projects/{projectId}/stop` and `/restart` control the existing legacy container without pulling or rebuilding it. Both return `{ "service": Service, "message": "…" }`.

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
| `POST` | `/api/projects/{projectId}/compose/validate` | Validate and preview a Compose import |
| `POST` | `/api/projects/{projectId}/compose` | Create and deploy all validated Compose services |
| `PUT` | `/api/services/{serviceId}` | Update its source/runtime definition |
| `POST` | `/api/services/{serviceId}/deploy` | Start async deployment |
| `POST` | `/api/services/{serviceId}/stop` | Stop its current container |
| `POST` | `/api/services/{serviceId}/restart` | Restart or start its current container |
| `POST` | `/api/services/{serviceId}/exec` | Execute a shell command in its current container |
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

Git repositories must be visible to the selected GitHub, GitLab, or Gitea connection. `buildStrategy` is `dockerfile` (default) or `auto`. Dockerfile builds require `dockerfilePath` and `buildContext`, both of which must remain inside the repository; absolute paths and parent traversal are rejected. Auto uses Railpack to inspect the repository and does not require a Dockerfile.

Auto builds run `railpack prepare` to generate `railpack-plan.json`, then send the source and plan to the BuildKit service included in `compose.yaml` through `DOKYR_BUILDKIT_HOST=tcp://buildkit:1234`. The daemon is isolated on the dedicated build network and its port is not published to the host. Set `DOKYR_BUILDKIT_HOST` to another reachable `tcp://host:port` endpoint to use an externally managed BuildKit deployment instead. Set `DOKYR_BUILDKIT_CACHE_REF` to a registry cache reference such as `registry.example.com/dokyr/{service}:cache`; `{service}` is replaced with the service ID, and Dokyr imports and exports that cache for each build.

The bundled builder reads DNS servers from `buildkitd.toml`. Adjust that file when production builds must resolve private package mirrors or internal hostnames instead of the public defaults.

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

### Service lifecycle, logs, and removal

`POST /api/services/{serviceId}/stop` and `POST /api/services/{serviceId}/restart` have no body and return `{ "service": ApplicationService, "message": "…" }`. Restart uses the current container and does not pull, clone, or rebuild; call deploy when the source or image should be refreshed. Lifecycle actions return `409` while a deployment is in progress or before the service has its first container.

`POST /api/services/{serviceId}/exec` executes one command through `/bin/sh -lc` in the stable service container:

```json
{ "command": "php artisan about", "workingDir": "/app" }
```

`workingDir` is optional and must be an absolute path inside the container. The response contains `stdout`, `stderr`, `exitCode`, `durationMs`, and the resolved container name. Output is capped at 2 MB and the HTTP request times out after 30 seconds; Docker does not provide an API to terminate an individual exec process, so a long-running command can continue in the container after the request times out. Owner, admin, and developer roles can use this endpoint; viewer accounts receive `403`.

`GET /api/services/{serviceId}/logs?lines=300` has the same response shape and `lines` limits as project logs, but uses the service container.

`DELETE /api/services/{serviceId}` has no body and returns `{ "ok": true }`. It returns `409` until every domain route targeting the service is removed.

### Compose import

Both Compose endpoints accept `{ "compose": "services:\n  ..." }`. Validation returns a normalized preview with `valid`, `services`, `applications`, `databases`, `errors`, and `warnings`; it never creates runtime resources.

Import repeats validation against current project state, writes all service definitions in one database transaction, deploys managed databases, and starts application deployments. Official `postgres`, `mysql`, and `mariadb` images become managed databases. Other prebuilt images become application services. Compose environment references to service hostnames are rewritten to the corresponding Dokyr private container names.

Build-only services, `env_file`, application volume mounts, configs, secrets, unresolved variable interpolation, port ranges, host-IP-specific bindings, and services without a prebuilt image fail validation. Published application ports are ignored in favor of domain routes. Managed databases keep an explicitly published port and replace Compose volumes and health checks with Dokyr-managed equivalents.

Successful import returns `201` with `{ "services": ApplicationService[], "databases": DatabaseService[], "deployments": Deployment[], "deploymentErrors": [] }`. If a database cannot be deployed, the imported definitions, containers, and newly created volumes are rolled back.

## Database clusters

Database clusters are global infrastructure. A cluster can contain several logical databases and users, and can join several project networks through independent attachments.

### Cluster endpoints

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/databases` | List all clusters with databases, users, grants, and projects |
| `POST` | `/api/databases` | Create a global cluster and queue background provisioning |
| `GET` | `/api/databases/{clusterId}` | Read one complete cluster |
| `GET` | `/api/databases/{clusterId}/credentials` | Reveal cluster administrator credentials |
| `GET` | `/api/databases/{clusterId}/events` | Read the latest deployment events |
| `GET` | `/api/databases/{clusterId}/logs?lines=300` | Read runtime logs |
| `PUT` | `/api/databases/{clusterId}/exposure` | Change public port exposure |
| `POST` | `/api/databases/{clusterId}/deploy` | Deploy or retry a cluster |
| `POST` | `/api/databases/{clusterId}/stop` | Stop the cluster container |
| `POST` | `/api/databases/{clusterId}/restart` | Restart or start the cluster container |
| `DELETE` | `/api/databases/{clusterId}` | Remove the cluster and optionally its volume |

`GET /api/databases` returns `{ "clusters": DatabaseService[] }`. `GET /api/databases/{clusterId}` returns `{ "cluster": DatabaseService }`. Each cluster includes `databases`, `users`, `grants`, `projects`, `projectCount`, `status`, and an optional `lastError`. Runtime statuses include `created`, `deploying`, `healthy`, `degraded`, `stopped`, and `failed`.

### Create a cluster

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

`engine` is `postgres`, `mysql`, or `mariadb`. If `password` is omitted, the server generates one. Private access is the default. A public port is used only when `publicEnabled` is true and defaults to the engine port.

The endpoint persists the cluster with status `deploying`, returns `201` immediately, and pulls and creates the container in the background:

```json
{
  "cluster": {
    "id": "db_…",
    "status": "deploying",
    "databases": [{ "id": "database_…", "name": "app", "primary": true }],
    "users": [{ "id": "dbuser_…", "username": "app", "admin": true }],
    "grants": [{ "databaseId": "database_…", "userId": "dbuser_…" }]
  },
  "service": { "id": "db_…", "status": "deploying" },
  "credentials": {
    "username": "app",
    "password": "revealed password",
    "database": "app",
    "host": "selfhost-db-db_…",
    "port": 5432,
    "connectionUrl": "postgresql://…",
    "publicEnabled": false,
    "publicPort": 0
  },
  "message": "Primary database was created and is provisioning in the background"
}
```

The initial logical database, administrator user, and grant are included in the `cluster` object. Dokyr resumes persisted provisioning work after a control-plane restart. If provisioning fails, `status` becomes `failed`, `lastError` is retained, and `POST /api/databases/{clusterId}/deploy` retries it.

The legacy `POST /api/projects/{projectId}/databases` endpoint accepts the same body, creates a cluster, and immediately adds its initial database and user to that project. New clients should create global clusters and use project attachments explicitly.

### Logical databases, users, and grants

| Method | Path | Body | Description |
| --- | --- | --- | --- |
| `POST` | `/api/databases/{clusterId}/logical-databases` | `{ "name": "orders", "ownerUserId": "dbuser_…" }` | Create a logical database |
| `DELETE` | `/api/databases/{clusterId}/logical-databases/{databaseId}` | — | Delete an unused, non-primary database |
| `POST` | `/api/databases/{clusterId}/users` | `{ "username": "orders_app", "password": "optional" }` | Create a database user |
| `GET` | `/api/databases/{clusterId}/users/{userId}/credentials` | — | Reveal a user's password |
| `DELETE` | `/api/databases/{clusterId}/users/{userId}` | — | Delete an unused, non-administrator user |
| `POST` | `/api/databases/{clusterId}/grants` | `{ "databaseId": "database_…", "userId": "dbuser_…" }` | Grant access to a database |
| `DELETE` | `/api/databases/{clusterId}/grants/{databaseId}/{userId}` | — | Revoke an unused grant |

User passwords are generated when omitted; supplied passwords must contain 12 to 256 characters without control characters. Deletion returns `409` when the resource is primary, administrative, owned, granted, or still used by a project attachment.

### Project attachments

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/projects/{projectId}/database-attachments` | Attach an existing cluster to the project network |
| `DELETE` | `/api/projects/{projectId}/database-attachments/{attachmentId}` | Detach without deleting the cluster or data |

Create an attachment by selecting one logical database and a user already granted access to it:

```json
{
  "clusterId": "db_…",
  "databaseId": "database_…",
  "userId": "dbuser_…",
  "alias": "database"
}
```

`alias` is the project-local hostname. It defaults from the cluster name and may contain lowercase letters, numbers, and hyphens. Returns `201` with `{ "attachment": ProjectDatabaseAttachment, "message": "…" }`. A cluster can be attached to several projects, but only once per project.

After attachment, `GET /api/databases/{clusterId}/credentials?projectId={projectId}` returns credentials for that project's selected database, user, and alias. The cluster must be attached to the project or the endpoint returns `404`.

Detaching disconnects the cluster from only that project's Docker network. The cluster, volume, logical database, user, and connections to other projects remain unchanged.

### Logs, lifecycle, exposure, and removal

`GET /api/databases/{clusterId}/events` returns the latest 1,000 persisted events in display order: `{ "events": [{ "id": 1, "stage": "pull", "type": "log", "message": "…", "createdAt": "…" }] }`. `GET /logs` accepts `lines` from 1 to 1,000. Clients can poll these snapshot endpoints while a log view is open to provide live-follow behavior.

`POST /api/databases/{clusterId}/stop` and `/restart` control the existing cluster container without changing its volume, credentials, logical resources, or attachments. Both return `{ "service": DatabaseService, "message": "…" }`.

`PUT /api/databases/{clusterId}/exposure` accepts `{ "enabled": true, "port": 5432 }`. The cluster is recreated with the requested host port; the previous exposure is restored if Docker fails. It returns `{ "service": DatabaseService }`.

`DELETE /api/databases/{clusterId}` accepts:

```json
{ "confirmation": "exact cluster name", "removeVolume": false }
```

Deletion returns `409` while the cluster is provisioning or attached to any project. A successful request returns `{ "removed": true, "volumeRemoved": false, "retainedVolume": "selfhost-data-db_…" }`. Setting `removeVolume: true` irreversibly removes every logical database in that volume.

## Deployments

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/deployments` | All deployments, newest first |
| `GET` | `/api/deployments/{deploymentId}` | Deployment, project, and events |
| `POST` | `/api/deployments/{deploymentId}/cancel` | Stop a running deployment |

`GET /api/deployments/{deploymentId}` response:

```json
{
  "deployment": { "…": "Deployment" },
  "project": { "…": "Project" },
  "events": [{ "…": "Deployment event" }]
}
```

A frontend can poll every 1–3 seconds while `deployment.status` is `deploying` or `building`, then stop when it becomes `healthy`, `degraded`, `failed`, or `cancelled`.

`POST /api/deployments/{deploymentId}/cancel` returns `202 Accepted` after cancellation is requested. The deployment remains live while Dokyr aborts the pull, clone, build, or candidate rollout and cleans up. It then becomes `cancelled`. For application services, an existing stable release remains online; cancelling a first deployment returns the undeployed service to `created`. Once promotion starts, Dokyr lets the atomic release switch finish and the endpoint returns `409`.

## Sources and registries

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/integrations` | Provider state, connected accounts, registries |
| `GET` | `/api/integrations/github/install/start` | Start GitHub App install redirect |
| `POST` | `/api/integrations/github/installations/sync` | Recover an already-installed personal GitHub App installation |
| `GET` | `/api/integrations/oauth/{provider}/start` | Start GitLab or Gitea OAuth redirect |
| `GET` | `/api/integrations/sources/{sourceId}/repositories` | Repositories allowed for source |
| `DELETE` | `/api/integrations/sources/{sourceId}` | Unlink a Git source; running containers remain untouched |
| `POST` | `/api/integrations/registries` | Add private Docker registry |
| `DELETE` | `/api/integrations/registries/{registryId}` | Delete registry credential |

GitHub repository integration uses a public GitHub App installation flow so
users other than the App owner can install it. If the server has no repository
App yet, `GET /api/integrations/github/install/start` first starts the App
Manifest flow and then continues to repository selection. Each user still
chooses all repositories or only selected repositories, and the App requests
read-only contents access. The returned installation has a `manageUrl`; use this
to change repository selection in GitHub. GitLab and Gitea use OAuth and
require provider configuration on the server.

Source connections are server-wide resources. `GET /api/integrations` and the
repository listing endpoint return every configured source to any role with
project read access. Roles with project write or deploy access can use those
connections for the same shared projects. Only the Dokyr user who created an
installation receives its `manageUrl`. Source connections and private registry
credentials expose `canDelete` for the current caller. Admins may delete only
resources they created; the Owner may delete any integration resource.
Creating, unlinking, or removing integrations still requires
`integration:write`.

If GitHub shows the App as installed but `GET /api/integrations` has no GitHub source connection, call `POST /api/integrations/github/installations/sync`. It lists installations using the server's repository-access GitHub App credential and imports only an installation whose GitHub account ID exactly matches the account already linked to the authenticated Dokyr user. It never imports an unrelated personal or organization installation.

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
  "providers": { "github": { "configured": true, "managed": true, "loginConfigured": true, "linked": true, "login": "…" } },
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
| `GET` | `/api/infrastructure/control-plane/metrics` | Metrics for Dokyr, PostgreSQL, and Caddy |
| `GET` | `/api/infrastructure/control-plane/logs?service=dokyr&lines=300` | Logs for one Dokyr control-plane service |
| `GET` | `/api/infrastructure/cleanup` | Cleanup preview |
| `POST` | `/api/infrastructure/cleanup` | Run selected Docker cleanup |

Metrics are global to the Docker host. Per-project metrics belong at `/api/projects/{projectId}/metrics`.
Control-plane metrics are restricted to the current Dokyr Compose project and the `dokyr`, `postgres`, `caddy`, and `registry` services. Control-plane logs accept only those service names and between 1 and 1000 lines. The legacy `selfhost` query value remains accepted for upgraded installations.

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

## Built-in registry API

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/object-storage` | List reusable S3-compatible connections |
| `POST` | `/api/object-storage` | Create an encrypted object storage connection |
| `PUT` | `/api/object-storage/{id}` | Update a connection; omit the secret to keep it |
| `DELETE` | `/api/object-storage/{id}` | Remove a connection that is not selected by Registry |
| `GET` | `/api/registry/status` | Registry container and authenticated API health |
| `GET` | `/api/registry/settings` | Read the storage configuration |
| `PUT` | `/api/registry/settings` | Save storage settings and recreate the registry |
| `GET` | `/api/registry/domain` | Read the managed registry domain and effective registry hosts |
| `PUT` | `/api/registry/domain` | Attach, update, or detach the registry domain and refresh Caddy |
| `GET` | `/api/registry/access-tokens` | List the current user's registry credentials |
| `POST` | `/api/registry/access-tokens` | Generate a registry credential |
| `DELETE` | `/api/registry/access-tokens/{tokenId}` | Revoke one credential |
| `GET` | `/api/registry/repositories` | List repositories and tags |
| `DELETE` | `/api/registry/tags?name=…&tag=…` | Delete a manifest tag |
| `POST` | `/api/registry/garbage-collection` | Run or preview garbage collection |
| `GET` | `/api/registry/token` | Docker Distribution token exchange |

Create an object storage connection with:

```json
{
  "name": "Production R2",
  "provider": "r2",
  "region": "auto",
  "bucket": "dokyr-registry",
  "endpoint": "https://account-id.r2.cloudflarestorage.com",
  "accessKey": "access-key-id",
  "secretKey": "secret-access-key",
  "forcePathStyle": true,
  "secure": true
}
```

`provider` is `aws`, `r2`, `minio`, `digitalocean`, or `custom`. The secret is
encrypted before persistence and responses expose only `hasSecretKey`. Registry
settings select a connection with `{ "storage": "s3", "objectStorageId":
"obj_…" }`; the server resolves its current credentials before recreating the
registry. A connection selected by Registry cannot be deleted.

Create a personal registry credential with:

```json
{ "name": "Production CI", "permission": "read_write" }
```

`permission` is `read_only` or `read_write`. The `201` response includes the
opaque `secret` and the user's email in `username`; the secret is returned only
once and only its SHA-256 hash is stored. Use the email and secret with
`docker login`. Account passwords are not accepted. Viewer accounts can create
only read-only credentials. The public token-exchange endpoint accepts HTTP
Basic credentials from Docker and returns a short-lived, repository-scoped
bearer token; clients should not call it directly.

## Frontend implementation notes

1. Keep credentials enabled for every same-origin API call; authentication is cookie-based, not bearer-token based.
2. Send only documented request fields. The server intentionally rejects unknown properties.
3. Treat deployment endpoints as async. Navigate to deployment detail or poll it after receiving `202`.
4. Runtime logs are snapshots, not WebSockets. Poll `?lines=` as desired and preserve the user-selected line limit.
5. Never persist SMTP passwords, object-storage secret keys, database credentials, Git tokens, registry access-token secrets, TOTP secrets, or decoded environment secrets in local storage, analytics, or error reporting.
6. When rendering Caddy route editors, allow multiple domains and ordered path rules, including multiple paths pointing to one service.
7. Do not automatically expose databases publicly. `publicEnabled` must remain an explicit user action.

## Source of truth

The route registrations and handlers live in [`internal/api/api.go`](https://github.com/azayr/dokyr/blob/main/internal/api/api.go). The JSON resource definitions live in [`internal/store/store.go`](https://github.com/azayr/dokyr/blob/main/internal/store/store.go). Update this document whenever either changes.
