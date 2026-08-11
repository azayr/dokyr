# Dokyr

A lightweight, self-hosted deployment control plane. The foundation combines a Go API, an embedded Svelte interface, PostgreSQL, Caddy, a private container registry, and a built-in Stalwart mail server.

The complete implementation guide—including container topology, request and deployment sequences, data model, security boundaries, configuration, operations, known limitations, and maintainer invariants—is in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

The prebuilt control-plane image is available as `ghcr.io/azayr/dokyr:latest`. It contains the Go service and Svelte interface; PostgreSQL, Caddy, the registry, Stalwart, the Docker socket, networks, and volumes are supplied by `compose.yaml`.

## Install on a VPS

The fastest way to install Dokyr on a fresh server is the remote installer. It downloads the Compose files, generates secure secrets, and starts the stack:

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/install.sh | sudo sh
```

The installer defaults to `/opt/dokyr`, publishes the control panel on HTTP port `80` and HTTPS port `443`, and prompts for the panel hostname while generating omitted secrets. Run it as root so the control plane can access the host Docker socket. After the stack starts, create the owner account, then configure one public mail-server hostname from **Infrastructure → Mail**.

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
docker compose up --build --watch
```

Open `http://localhost:8888`. The non-standard default avoids conflicts with local tools such as Laravel Herd. On the first visit, Dokyr asks you to create the owner account; public registration closes immediately afterward. PostgreSQL persists in the `postgres_data` volume and the control plane reads the host Docker Engine through `/var/run/docker.sock`.

Compose Watch monitors both the Svelte UI and Go API sources. Saving a change automatically rebuilds and replaces the Dokyr container, so there is no manual rebuild step. Compose Watch requires Docker Compose 2.22 or later.

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

The in-app updater replaces the Dokyr image only. When a release changes the
Compose topology—such as adding the bundled Stalwart service—an existing VPS
must also refresh `compose.yaml`. Preserve a backup and any local edits:

```sh
cd /opt/dokyr
sudo cp compose.yaml compose.yaml.before-stalwart
sudo curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/compose.yaml -o compose.yaml
sudo sed -i.bak '/^    build: \.$/d' compose.yaml
sudo docker compose pull
sudo docker compose up -d --remove-orphans
```

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

For S3-compatible storage, open **Infrastructure → Object storage**, add an
Amazon S3, Cloudflare R2, MinIO, DigitalOcean Spaces, or custom connection, then
select it under **Registry → Storage backend**. Secret keys are encrypted and
are never returned by the API after saving.

`REGISTRY_STORAGE=s3` and the `REGISTRY_S3_*` variables remain available for
first-start configuration. Dokyr imports them into a reusable object storage
connection. For MinIO, set `REGISTRY_S3_ENDPOINT=https://minio.example.com` and
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

## Documentation site

The landing page and operator documentation are a VitePress site in `docs/`:

```sh
cd docs
pnpm install
pnpm dev
```

Run `pnpm build` for a local production build. Pushes to `main` that change
`docs/**` trigger `.github/workflows/publish-pages.yml`, which builds with the
`/dokyr/` base path and deploys the result to GitHub Pages.

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

## Developer mail gateway

Dokyr can expose a small Resend-style sending workflow under
**Infrastructure → Mail**. A developer adds a domain, publishes the unique
ownership TXT record, and asks Dokyr to verify public DNS. Stalwart ships as a
first-class service in the Compose stack. Dokyr bootstraps it with RocksDB
storage, manages domains through its internal JMAP endpoint, and imports the
generated DKIM, SPF, MX, DMARC, and discovery records for copy-and-verify
setup. Stalwart configuration and messages persist in the `stalwart_config`
and `stalwart_data` volumes.

Use a dedicated sending subdomain such as `updates.example.com` for the first
release. Stalwart generates DNS for the exact domain it manages, so using the
apex domain can overlap with an existing mailbox provider's MX and SPF records.

On the first visit to **Infrastructure → Mail**, the platform owner enters the
Dokyr installation domain, for example `example.com`. Dokyr uses
`mail.example.com` as the public Stalwart hostname and enables an HTTP-01 Let's
Encrypt certificate for its mail listeners. Add a DNS-only A/AAAA record for
that hostname before setup and configure matching PTR (reverse DNS) with the
VPS provider.

After every required sending record is verified, the developer creates a
one-time, domain-scoped mail credential. Dokyr provisions a unique Stalwart
SMTP username and uses the `dkr_mail_…` secret as both its SMTP password and the
bearer token for `POST /v1/emails`. The credential may send only from its
verified domain. Credentials created by an older Dokyr release remain HTTP-only
and must be recreated to enable SMTP. **Settings → SMTP** remains separate and
controls Dokyr's own deployment notifications.

The API-key secret is shown only once. Save it when the key is created, then
send a first message by replacing the Dokyr URL, key, sender domain, and
recipient below:

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

A successful request returns `{"id":"mail_…","status":"queued"}` and appears
under **Infrastructure → Mail → Activity**. Queued means Stalwart accepted the
message; it does not prove that the recipient provider delivered it. The sender
address must use the verified domain attached to the credential.

Applications can instead submit directly over SMTP using the values shown when
the credential is created:

```dotenv
SMTP_HOST=mail.example.com
SMTP_PORT=465
SMTP_SECURE=true
SMTP_USERNAME=smtp-generated-id@updates.example.com
SMTP_PASSWORD=dkr_mail_YOUR_FULL_API_KEY
SMTP_FROM=hello@updates.example.com
```

Port 465 uses implicit TLS. Do not use the Dokyr login email as the SMTP
username; use the generated username shown beside the credential.

For production, allow inbound TCP ports 25 and 465 (plus 993, 995, and 4190
only when exposing mailbox protocols). Port 80 must be reachable for ACME
HTTP-01; Caddy forwards only the mail hostname's challenge path to Stalwart.
Some VPS providers block port 25 by default. The mail A/AAAA record must not be
HTTP-proxied by a DNS provider. Server-to-server SMTP on port 25 continues to
use opportunistic TLS.

An external Stalwart installation remains supported by overriding
`MAIL_STALWART_URL` and `MAIL_STALWART_API_KEY`, clearing the bundled Basic
authentication values, and pointing the `MAIL_STALWART_RELAY_*` values at its
submission listener. Alternatively, clear the relay values and use the generic
SMTP configuration for delivery.

This first milestone sends synchronously and records the latest delivery
attempts. It does not yet include a durable retry queue, bounce/complaint
webhooks, suppression lists, per-key rate limits, or reputation management;
those are required before offering the gateway to untrusted public tenants.

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
