# Selfhost

A lightweight, self-hosted deployment control plane. The foundation combines a Go API, an embedded Svelte interface, a small PostgreSQL container, Docker Engine discovery through the host socket, and a separate Caddy reverse proxy.

The complete implementation guide—including container topology, request and deployment sequences, data model, security boundaries, configuration, operations, known limitations, and maintainer invariants—is in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

The prebuilt control-plane image is available as `brahoul/selfhost:latest`. It contains the Go service and Svelte interface; PostgreSQL, Caddy, the Docker socket, networks, and volumes are still supplied by `compose.yaml`.

## Run it

```sh
docker compose up --build
```

Open `http://localhost:8080`. The non-standard default avoids conflicts with local tools such as Laravel Herd. On the first visit, Selfhost asks you to create the owner account; public registration closes immediately afterward. PostgreSQL persists in the `postgres_data` volume and the control plane reads the host Docker Engine through `/var/run/docker.sock`.

Caddy rejects unknown hostnames with a 404 instead of forwarding them to the control panel. Direct IPv4 access is allowed automatically, so a fresh installation remains reachable at the VPS IP and published HTTP port. `CONTROL_HOSTS` is the allowlist for additional control-panel domain names. For example, on a VPS:

```sh
CONTROL_HOSTS="panel.example.com"
PUBLIC_URL=http://panel.example.com:8080
```

Project domains are assigned from each project's **Domains** tab. A hostname that is neither assigned to a project nor listed in `CONTROL_HOSTS` receives Caddy's 404 response.

The published ports are configurable. For a production server where ports 80 and 443 are free:

```sh
HTTP_PORT=80 HTTPS_PORT=443 docker compose up -d --build
```

For frontend development, run the API and web app separately:

```sh
make api
make web
```

The Vite development server proxies `/api` to the Go service on port 8080.

## SMTP and password recovery

SMTP can be configured interactively from **Settings → SMTP** or bootstrapped
once through `.env`/Docker Compose. To bootstrap it, set at least
`SMTP_HOST` and `SMTP_FROM_EMAIL`; the remaining options are documented in
`.env.example`.

On startup, Selfhost imports a complete environment configuration only when
the `smtp_settings` row does not exist. The SMTP password is encrypted before
the row is created. From that point onward PostgreSQL is the only source of
truth: restarting the Compose stack—or changing/removing the `SMTP_*`
variables—does not replace settings saved in the interface.

To deliberately bootstrap a different SMTP server, first remove the SMTP
configuration through an explicit administrative/database operation. Merely
restarting the containers is intentionally not enough.

## Private repositories and registries

GitHub needs no OAuth values in `.env`. Open **Settings → Security**, select
**Link GitHub**, and authorize creation of this server's private GitHub App.
Then open **Sources** and select which repositories—or all repositories—the
app may access. Repository access can be changed later from the same page.

If the private GitHub App is deleted from GitHub, its encrypted local
credentials become unusable. The next GitHub link, login, or repository-install
attempt verifies the App with GitHub first. A confirmed 401/404 clears the stale
App credentials and installations; an authenticated owner is then taken through
the App creation flow again. Users attempting GitHub login must sign in with
their password once and reconnect GitHub from **Settings → Security**.

GitLab does not provide GitHub's App Manifest flow. To connect GitLab, copy
`.env.example` to `.env`, create a GitLab OAuth application, and use this exact
callback URL:

```text
http://localhost:8080/api/integrations/oauth/gitlab/callback
```

For production, replace `http://localhost:8080` with `PUBLIC_URL` and enable `COOKIE_SECURE=true`. GitLab self-managed instances can be selected with `GITLAB_BASE_URL`.

GitHub App credentials, GitLab provider tokens, and private registry passwords
are encrypted with `ENCRYPTION_KEY` before PostgreSQL stores them. Keep that key
stable: changing it makes existing saved credentials unreadable. GitHub
installation tokens are short-lived and generated only when repository access
is needed; they are never persisted.

Private container images are supported from the project creation screen. Save an optional Registry V2 credential under **Sources**, then enter a complete image reference such as `ghcr.io/acme/customer-api:latest`.

## Current milestone

- First-run owner creation and JWT sessions in secure, HTTP-only cookies
- Operations dashboard with Docker health, projects, deployments and node metrics
- Project overview with services, traffic and deployment history
- Deployment detail with pipeline and logs
- PostgreSQL schema managed by ordered, embedded SQL migration files
- GitHub and GitLab OAuth connections with private repository discovery
- Encrypted private Docker registry credentials and image-based projects
- Docker Engine health integration using the official Go client
- Production multi-stage image and Caddy/Compose topology

## Security boundary

Access to the Docker socket is equivalent to administrative access to the host. Only the control-plane container receives it. Caddy, PostgreSQL and managed workloads never receive the socket. Before exposing this beyond a trusted network, replace the example database password, JWT secret, and encryption key; enable secure cookies behind HTTPS; and add request auditing plus strict resource validation.
