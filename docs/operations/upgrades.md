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

The in-app updater replaces the Dokyr image only. When release notes mention changes to the platform Compose topology, run the topology updater:

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/update.sh | sudo sh
```

The script downloads and validates `compose.production.yaml`, pulls required
images, backs up the current Compose file and runtime configuration, and runs
`docker compose up -d --remove-orphans`. If reconciliation fails, it restores
the previous files and attempts to bring the previous topology back up. It never
runs `compose down` or removes volumes, so separately managed project
containers and databases remain running.

To validate an update without changing the installed stack:

```sh
curl -fsSL https://raw.githubusercontent.com/azayr/dokyr/main/scripts/update.sh -o /tmp/update-dokyr.sh
sudo DOKYR_UPDATE_DRY_RUN=true sh /tmp/update-dokyr.sh
```

Release database migrations are designed to remain backward compatible so the previous image can run after a failed update.
