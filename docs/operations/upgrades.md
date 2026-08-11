---
title: Update the Dokyr control plane
description: Safely update Dokyr to a newer container image with health verification and automatic rollback.
---

# Update the Dokyr control plane

Release images publish to `ghcr.io/azayr/dokyr`. Dokyr compares the running image digest with the configured registry channel rather than trusting a mutable tag name.

## Update from the interface

Open **Settings → Platform**. The page shows the running version and reports when the selected channel resolves to a different immutable digest.

When you start an update, Dokyr:

1. pulls the exact target digest;
2. starts a one-shot helper from the trusted current image;
3. preserves the current container configuration;
4. replaces only the Dokyr control-plane container;
5. waits for `/api/health` on the replacement;
6. removes the previous container on success or restores it on failure.

Managed applications, Caddy, PostgreSQL, Registry, and Stalwart do not restart during this process.

## Automatic updates

Automatic updates are disabled by default. When enabled, they run during the configured maintenance hour and use the same verified replacement path.

## Compose topology changes

The in-app updater replaces the Dokyr image only. When release notes mention changes to `compose.yaml`, refresh the file separately and preserve a backup of local edits:

```sh
cd /opt/dokyr
sudo cp compose.yaml compose.yaml.before-update
sudo curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/compose.yaml -o compose.yaml
sudo sed -i.bak '/^    build: \.$/d' compose.yaml
sudo docker compose pull
sudo docker compose up -d --remove-orphans
```

Release database migrations are designed to remain backward compatible so the previous image can run after a failed update.
