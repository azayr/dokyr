---
title: Private container registry
description: Configure Dokyr's built-in Docker registry, issue scoped credentials, and store images locally or in S3-compatible storage.
---

# Private container registry

Dokyr includes Docker Distribution as a private registry. Caddy exposes it through a hostname, and Dokyr issues short-lived bearer tokens after validating personal access credentials.

## Attach a registry hostname

1. Point a DNS record such as `registry.example.com` at the Dokyr host.
2. Open **Infrastructure → Registry → Registry domain**.
3. Save the hostname and enable automatic HTTPS.

The registry is not published on a direct host port. Caddy forwards its hostname to the internal `registry:5000` service.

## Create an access token

Open **Infrastructure → Registry → Access tokens** and generate a read-only or read-write credential. The secret appears once.

```sh
docker login registry.example.com
docker tag myapp:2.4.1 registry.example.com/acme/myapp:2.4.1
docker push registry.example.com/acme/myapp:2.4.1
```

Use your Dokyr email as the Docker username and the generated token as the password. Dokyr account passwords are never accepted by the registry.

The stored credential is an irreversible hash. Revoke it from the Registry page to invalidate future exchanges immediately.

## Storage backends

The default filesystem driver keeps image data in the `registry_data` Docker volume. For S3, Cloudflare R2, MinIO, DigitalOcean Spaces, or another compatible service, create an [object storage connection](/infrastructure/storage) and select it under **Registry → Storage backend**.
