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
- TCP port 3030 available for initial HTTP access and port 443 available for HTTPS;
- a DNS A or AAAA record when you want a hostname for the control panel.

::: warning Docker access
Dokyr controls the host Docker Engine through `/var/run/docker.sock`. Anyone who can administer Dokyr can indirectly administer the host. Use a dedicated server and restrict owner access.
:::

## Run the installer

```sh
curl -fsSL https://sh.dokyr.com | sudo sh
```

The installer uses `/opt/dokyr` by default, detects the server's public IP, publishes the temporary panel URL on port 3030, and generates the database password and application secrets automatically. It does not prompt for infrastructure configuration.

After Compose reports the services as running, open the displayed panel URL and enter only the owner's name, email, password, and password confirmation. Public account creation closes as soon as this owner exists. The dashboard keeps a warning visible until a permanent control-panel domain is connected from **Infrastructure → Domains**.

## Automated installation

Download the script first when you need a repeatable, non-interactive installation:

```sh
curl -fsSL https://sh.dokyr.com/install.sh -o /tmp/install-dokyr.sh

sudo DOKYR_INSTALL_DIR=/srv/dokyr \
  HTTP_PORT=3030 \
  HTTPS_PORT=443 \
  sh /tmp/install-dokyr.sh
```

Use `SERVER_IP` only when automatic public-IP detection is not appropriate. Existing automation can still override `PUBLIC_URL`, `CONTROL_HOSTS`, and generated secrets, but normal installations do not need them. Back up `/opt/dokyr/.env`; it contains the generated encryption key used for stored credentials.

## Verify the installation

Check the public health endpoint:

```sh
curl --fail http://SERVER_IP:3030/api/health
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
