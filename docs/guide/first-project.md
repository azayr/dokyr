---
title: Create your first project
description: Deploy a container image with Dokyr, configure environment variables and health checks, then expose it through a domain.
---

# Create your first project

A project groups application services, databases, domains, and deployment history. Start with an image that already exposes an HTTP port.

## 1. Choose the image source

Open **Workspace → Projects → New project** and select an image-based source. Enter a complete image reference:

```text
ghcr.io/acme/example-api:2.4.1
```

Prefer immutable version tags or digests in production. A mutable `latest` tag can point to different content over time and makes rollbacks harder to reason about.

For a private image, save the Registry V2 credential under **Infrastructure → Sources** first, then select that connection while creating the service.

## 2. Configure the service

Set the port the process listens on inside its container. Do not enter the public host port; Caddy reaches the service over Dokyr's private proxy network.

Add encrypted environment variables from the service configuration screen:

```dotenv
APP_ENV=production
DATABASE_URL=postgres://app:secret@database:5432/app
```

Dokyr never returns stored secret values through the API after they are saved.

## 3. Add a health check

An HTTP health check is the clearest promotion signal for a web service:

```text
GET /health
```

The replacement container must pass before Dokyr promotes it behind Caddy. You can also configure a command health check for workers that do not expose HTTP.

## 4. Deploy

Start the deployment and follow its event stream. An image deployment moves through pull, replace, create, start, verify, promote, and complete stages. A failed replacement leaves the prior working container available for rollback.

See [Deployments](/guide/deployments) for the lifecycle and troubleshooting signals.

## 5. Add data and traffic

- Add PostgreSQL, MySQL, or MariaDB from the project database area. Database containers remain private unless you deliberately publish a host port.
- Attach a hostname from **Infrastructure → Domains**, choose the target service and path, then enable automatic HTTPS.

Continue with [Domains and HTTPS](/infrastructure/domains).
