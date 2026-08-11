---
title: S3-compatible object storage
description: Connect Amazon S3, Cloudflare R2, MinIO, DigitalOcean Spaces, or another S3-compatible provider to Dokyr.
---

# S3-compatible object storage

Object-storage connections are reusable encrypted resources. Dokyr can use them for registry objects and server backup archives.

## Create a connection

Open **Infrastructure → Object storage** and provide:

- a memorable connection name;
- region and bucket;
- access key and secret key;
- an optional custom endpoint;
- path-style addressing when required by the provider.

For MinIO, use its complete HTTPS endpoint and enable path-style addressing:

```text
Endpoint: https://minio.example.com
Force path style: enabled
```

Use plain HTTP only on a trusted private network. Secret keys are encrypted with the installation `ENCRYPTION_KEY` and are not returned by the API after saving.

## Least privilege

Give Dokyr access only to the selected bucket. The credential needs to list the bucket and read, create, and delete objects used by its configured feature. Use separate buckets or credentials when registry retention and backup retention have different policies.

## Use the connection

- Select it from **Infrastructure → Registry** to store Docker image objects in S3-compatible storage.
- Select it from **Infrastructure → Servers → Backups** to create on-demand or scheduled control-plane backups.

See [Backups and restores](/operations/backups) before relying on an archive for disaster recovery.
