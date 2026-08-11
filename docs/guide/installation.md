---
title: Install Dokyr on a VPS
description: Install the Dokyr self-hosted PaaS on a Linux VPS, configure its public hostname, and complete first-run setup.
---

# Install Dokyr on a VPS

The installer downloads the reference Compose topology, generates secure secrets, and starts the control plane and its supporting services.

## Requirements

Prepare a Linux server with:

- a recent Docker Engine and Docker Compose;
- root or `sudo` access;
- at least 2 GB of memory for the platform and a small workload;
- TCP ports 80 and 443 available for public HTTP and HTTPS;
- a DNS A or AAAA record when you want a hostname for the control panel.

::: warning Docker access
Dokyr controls the host Docker Engine through `/var/run/docker.sock`. Anyone who can administer Dokyr can indirectly administer the host. Use a dedicated server and restrict owner access.
:::

## Run the installer

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/install.sh | sudo sh
```

The installer uses `/opt/dokyr` by default, publishes the panel on ports 80 and 443, and prompts for the control-panel hostname when it cannot infer one.

After Compose reports the services as running, open the panel URL and create the first owner. Public account creation closes as soon as this owner exists.

## Automated installation

Download the script first when you need a repeatable, non-interactive installation:

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/install.sh -o /tmp/install-dokyr.sh

sudo DOKYR_INSTALL_DIR=/srv/dokyr \
  HTTP_PORT=80 \
  HTTPS_PORT=443 \
  PUBLIC_URL=https://panel.example.com \
  CONTROL_HOSTS="panel.example.com" \
  POSTGRES_PASSWORD="replace-with-a-long-random-value" \
  JWT_SECRET="replace-with-at-least-32-random-characters" \
  ENCRYPTION_KEY="replace-with-at-least-32-random-characters" \
  sh /tmp/install-dokyr.sh
```

Keep `ENCRYPTION_KEY` stable and backed up. Dokyr uses it to encrypt integration tokens, registry passwords, object-storage secrets, and SMTP credentials stored in PostgreSQL.

## Verify the installation

Check the public health endpoint:

```sh
curl --fail https://panel.example.com/api/health
```

A healthy response reports `ok: true` and includes PostgreSQL and Docker status. If the hostname is not ready, connect using the server IP and configured HTTP port first.

## Local development

From the repository root:

```sh
docker compose up --build --watch
```

Open `http://localhost:8888`. Compose Watch rebuilds the bundled Svelte interface or Go service when their source changes.

## Next

- [Create your first project](/guide/first-project)
- [Configure domains and HTTPS](/infrastructure/domains)
- [Understand platform updates](/operations/upgrades)
