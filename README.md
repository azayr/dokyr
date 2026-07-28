# Dokyr

A lightweight, self-hosted deployment control plane. The foundation combines a Go API, an embedded Svelte interface, a small PostgreSQL container, Docker Engine discovery through the host socket, and a separate Caddy reverse proxy.

The complete implementation guide—including container topology, request and deployment sequences, data model, security boundaries, configuration, operations, known limitations, and maintainer invariants—is in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

The prebuilt control-plane image is available as `ghcr.io/azayr/dokyr:latest`. It contains the Go service and Svelte interface; PostgreSQL, Caddy, the Docker socket, networks, and volumes are still supplied by `compose.yaml`.

## Install on a VPS

The fastest way to install Dokyr on a fresh server is the remote installer. It downloads the Compose files, generates secure secrets, and starts the stack:

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/install.sh | sudo sh
```

The installer defaults to `/opt/dokyr`, publishes the control panel on HTTP port `80` and HTTPS port `443`, and asks only for the public URL and control-panel hostname. Run it as root so the control plane can access the host Docker socket. After the stack starts, open the printed URL and create the owner account.

For an automated install, download the script and run it with the variables you want preset:

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/install.sh -o /tmp/install-dokyr.sh
sudo DOKYR_INSTALL_DIR=/srv/dokyr \
  HTTP_PORT=80 HTTPS_PORT=443 \
  PUBLIC_URL=http://panel.example.com \
  CONTROL_HOSTS="panel.example.com" \
  POSTGRES_PASSWORD="..." JWT_SECRET="..." ENCRYPTION_KEY="..." \
  sh /tmp/install-dokyr.sh
```

## Uninstall

To remove the Dokyr control plane, PostgreSQL data, and configuration, run the uninstaller:

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/uninstall.sh | sudo sh
```

This stops the containers, removes their volumes, and deletes the install directory. It does not remove Docker images, networks, or any project containers and volumes Dokyr created for your applications.

## Run it

```sh
docker compose up --build
```

Open `http://localhost:8888`. The non-standard default avoids conflicts with local tools such as Laravel Herd. On the first visit, Dokyr asks you to create the owner account; public registration closes immediately afterward. PostgreSQL persists in the `postgres_data` volume and the control plane reads the host Docker Engine through `/var/run/docker.sock`.

For a VPS using the published image, start without a local build:

```sh
docker compose pull
docker compose up -d
```

After this initial Compose installation, Dokyr manages its own control-plane
updates from **Settings → Platform**. The sidebar shows the running version and
signals when the configured registry channel has a different immutable image
digest. Manual and automatic updates pull that digest, hand replacement to an
external helper container, verify `/api/health`, and restore the previous
container if verification fails. Managed applications, Caddy, and PostgreSQL
are not restarted.

Automatic updates are off by default. When enabled, they run only during the
configured maintenance hour. Release migrations must remain backward
compatible so a container rollback can safely run the previous binary.

Caddy rejects unknown hostnames with a 404 instead of forwarding them to the control panel. Direct IPv4 access is allowed automatically, so a fresh installation remains reachable at the VPS IP and published HTTP port. `CONTROL_HOSTS` is the allowlist for additional control-panel domain names. For example, on a VPS:

```sh
CONTROL_HOSTS="panel.example.com"
PUBLIC_URL=http://panel.example.com:8888
```

Project domains are assigned from each project's **Domains** tab. A hostname that is neither assigned to a project nor listed in `CONTROL_HOSTS` receives Caddy's 404 response.

The published ports are configurable. For a production server where ports 80 and 443 are free:

```sh
HTTP_PORT=80 HTTPS_PORT=443 docker compose up -d --build
```

## Built-in container registry

Dokyr includes the Docker Distribution registry. By default it stores images in
a Docker volume. Open **Infrastructure → Registry → Registry domain** to attach
a hostname and let Dokyr configure the Caddy route and automatic HTTPS:

```sh
docker login registry.example.com
docker tag myapp:latest registry.example.com/myapp:latest
docker push registry.example.com/myapp:latest
```

Create the hostname's A or AAAA DNS record first and point it to the Dokyr
server. `REGISTRY_HOSTS` remains available as a first-start or compatibility
fallback, but a domain saved in Dokyr is the source of truth for generated
credentials and image references.

Open **Infrastructure → Registry → Access tokens** and generate a personal
read-only or read-write credential. Use your Dokyr email as the Docker username
and the one-time token as the password. Dokyr account passwords are never
accepted by the registry.

The stored credential is an irreversible hash. During login, Dokyr exchanges it
for a short-lived bearer token scoped to the repository/actions requested by
Docker. Read-only credentials can pull but cannot push. Read-write credentials
can push for `developer`, `admin`, and `owner` users; `viewer` users remain
pull-only. Revoke a credential from the Registry page to invalidate it
immediately.

For S3-compatible storage, set `REGISTRY_STORAGE=s3` and fill the
`REGISTRY_S3_*` variables in `.env`. For MinIO, set
`REGISTRY_S3_ENDPOINT=https://minio.example.com` and
`REGISTRY_S3_FORCEPATHSTYLE=true`. Use `REGISTRY_S3_SECURE=false` only when the
endpoint is plain HTTP.

The registry is not exported with a direct host port. Caddy routes matching
registry hosts to the internal `registry:5000` service on the control network,
and Docker Distribution validates Dokyr-issued bearer tokens before accepting
push or pull requests.

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

On startup, Dokyr imports a complete environment configuration only when
the `smtp_settings` row does not exist. The SMTP password is encrypted before
the row is created. From that point onward PostgreSQL is the only source of
truth: restarting the Compose stack—or changing/removing the `SMTP_*`
variables—does not replace settings saved in the interface.

To deliberately bootstrap a different SMTP server, first remove the SMTP
configuration through an explicit administrative/database operation. Merely
restarting the containers is intentionally not enough.

## GitHub login, private repositories, and registries

GitHub login and repository access use separate applications and permissions.
No GitHub domain, client ID, or client secret is configured in `.env`. The first
authenticated user who selects **Connect GitHub** under **Settings → Security**
is sent through GitHub's App Manifest flow. Dokyr includes the current
`PUBLIC_URL` automatically, then stores the returned credentials encrypted.
This creates a public, identity-only GitHub App for that Dokyr server. It
requests no repository permissions and cannot list or clone repositories.

After setup, each user signs in with their password once and links their GitHub
identity under **Settings → Security**. Repository access is optional and
remains separate: open **Sources**, select **Select repositories**, and Dokyr
will create a second, public GitHub App so every Dokyr user can install it. Each
user then grants it access to all repositories or only selected repositories.
Connected repository sources are shared across the Dokyr server. They are
visible to roles that can read projects and usable for project changes or
deployments when that role holds the corresponding permission. The GitHub
installation management link remains visible only to the Dokyr user who
connected that installation. Admins may unlink only source connections they
added; the Owner may unlink any connection. Private registry credentials follow
the same rule and are shared for authorized project deployments.

If the repository-access GitHub App is deleted from GitHub, its encrypted local
credentials become unusable. The next repository-install attempt verifies the
App with GitHub first. A confirmed 401/404 clears the stale App credentials and
installations, then **Select repositories** starts a fresh App creation flow.
GitHub login is unaffected because it uses the separate identity App.

GitLab does not provide GitHub's App Manifest flow. To connect GitLab, copy
`.env.example` to `.env`, create a GitLab OAuth application, and use this exact
callback URL:

```text
http://localhost:8888/api/integrations/oauth/gitlab/callback
```

For production, replace `http://localhost:8888` with `PUBLIC_URL` and enable `COOKIE_SECURE=true`. GitLab self-managed instances can be selected with `GITLAB_BASE_URL`.

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
- Identity-only GitHub App plus separate GitHub App/GitLab repository discovery
- Encrypted private Docker registry credentials and image-based projects
- Docker Engine health integration using the official Go client
- Production multi-stage image and Caddy/Compose topology

## Security boundary

Access to the Docker socket is equivalent to administrative access to the host. Only the control-plane container receives it. Caddy, PostgreSQL and managed workloads never receive the socket. Before exposing this beyond a trusted network, replace the example database password, JWT secret, and encryption key; enable secure cookies behind HTTPS; and add request auditing plus strict resource validation.
