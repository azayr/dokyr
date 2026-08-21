---
title: Configuration reference
description: Reference the core environment variables used to configure Dokyr ports, security, updates, registry, mail, and integrations.
---

# Configuration reference

The installer writes the production environment file. For local development, copy `.env.example` and change values before exposing the stack outside your machine.

## Control plane

| Variable | Purpose | Default |
| --- | --- | --- |
| `HTTP_PORT` | Standard Caddy HTTP ingress port (`80` in VPS installs; `8888` locally) | `8888` |
| `RECOVERY_HTTP_PORT` | Secondary HTTP port for temporary IP and recovery access in the VPS topology | `3030` |
| `HTTPS_PORT` | Public Caddy HTTPS port | `8443` |
| `PUBLIC_URL` | Complete external control-panel URL | `http://localhost:8888` |
| `CONTROL_HOSTS` | Space-separated control-panel host allowlist | `localhost` |
| `POSTGRES_PASSWORD` | Bundled control-plane database password | development placeholder |
| `JWT_SECRET` | Session signing key, at least 32 characters | development placeholder |
| `ENCRYPTION_KEY` | Stored-secret encryption key, at least 32 characters | development placeholder |
| `COOKIE_SECURE` | Send the session cookie only over HTTPS | `false` |
| `DOKYR_BUILDKIT_HOST` | BuildKit daemon used by Auto builds | `tcp://buildkit:1234` |
| `DOKYR_BUILDKIT_CACHE_REF` | Optional registry cache reference; supports `{service}` | empty |
| `DOKYR_RAILPACK_FRONTEND` | BuildKit frontend used for Railpack plans | `ghcr.io/railwayapp/railpack-frontend:latest` |

The VPS installer exposes Caddy on standard HTTP port 80 for domain traffic and ACME challenges, while retaining port 3030 for temporary IP and recovery access. It detects `SERVER_IP`, derives the temporary `PUBLIC_URL` from `RECOVERY_HTTP_PORT`, and generates `POSTGRES_PASSWORD`, `JWT_SECRET`, and `ENCRYPTION_KEY`. These variables remain available for advanced automation and recovery; they are not interactive installation questions. A permanent control-panel domain is stored from **Infrastructure → Domains** after the owner account is created.

## Platform updates

| Variable | Purpose | Default |
| --- | --- | --- |
| `DOKYR_IMAGE` | Image used by the Compose service | `ghcr.io/azayr/dokyr:latest` |
| `DOKYR_REGISTRY_IMAGE` | Registry repository checked for updates | `ghcr.io/azayr/dokyr` |
| `DOKYR_UPDATE_CHANNEL` | Mutable channel resolved to an immutable digest | `latest` |

## Registry

| Variable | Purpose | Default |
| --- | --- | --- |
| `REGISTRY_HOSTS` | First-start compatibility hostname | `registry.invalid` |
| `REGISTRY_STORAGE` | `filesystem` or `s3` | `filesystem` |
| `REGISTRY_HTTP_RELATIVEURLS` | Keep upload redirects relative behind proxies | `true` |
| `REGISTRY_S3_REGION` | S3 region | empty |
| `REGISTRY_S3_BUCKET` | S3 bucket | empty |
| `REGISTRY_S3_ENDPOINT` | Custom S3-compatible endpoint | empty |
| `REGISTRY_S3_FORCEPATHSTYLE` | Use path-style bucket addressing | `false` |

## Integrations and mail

GitHub applications are created interactively through the App Manifest flow and do not require static client credentials. GitLab OAuth uses `GITLAB_CLIENT_ID`, `GITLAB_CLIENT_SECRET`, and optionally `GITLAB_BASE_URL`. Gitea OAuth uses `GITEA_CLIENT_ID`, `GITEA_CLIENT_SECRET`, and `GITEA_BASE_URL`; the base URL may be an HTTPS production origin or an explicit `http://` local-network origin for development.

Dokyr's own notification SMTP settings can be bootstrapped once with the `SMTP_*` variables. After the complete configuration is imported, PostgreSQL becomes the source of truth and later Compose restarts do not overwrite values saved in the interface.

The bundled Stalwart service uses `STALWART_*` and `MAIL_STALWART_*` variables. The installer generates its passwords. Configure the public mail hostname from **Infrastructure → Mail** rather than editing the compatibility variables by hand.

See the repository's [`.env.example`](https://github.com/azayr/dokyr/blob/main/.env.example) for the complete list and comments.
