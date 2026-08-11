---
title: Backups and restores
description: Create scheduled Dokyr control-plane backups in S3-compatible storage and restore them safely.
---

# Backups and restores

Dokyr can queue on-demand or scheduled server backups to a reusable S3-compatible object-storage connection.

## What a backup contains

The backup worker asks the bundled PostgreSQL service for a consistent plain-SQL dump, packages it with a versioned manifest, and uploads a `.tar.gz` archive:

```text
dokyr-server-2026-08-11T112431Z.tar.gz
```

The archive contains control-plane records, including encrypted credentials. It does not contain:

- the installation `ENCRYPTION_KEY`;
- application container images;
- managed application database volumes;
- Caddy certificate data;
- Registry or Stalwart volume contents.

Back up those resources separately according to their storage backend.

## Encryption-key requirement

Restoring to another server requires the same `ENCRYPTION_KEY`. Without it, restored integration tokens, storage credentials, registry passwords, and SMTP secrets cannot be decrypted.

Store the key outside the Dokyr server and test access to it before an incident.

## Restore behavior

Restore downloads and validates the archive, stages the SQL inside PostgreSQL, and applies it with stop-on-error semantics in one transaction. Dokyr then rebuilds managed Caddy routes from the restored database.

Run a restore during a maintenance window. Verify account access, integrations, domains, and object-storage connectivity immediately afterward.
