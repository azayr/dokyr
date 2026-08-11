---
title: Security model
description: Understand Dokyr's Docker socket boundary, encrypted secrets, authentication model, network isolation, and production hardening requirements.
---

# Security model

Dokyr is an infrastructure control plane. Treat owner and administrator access as privileged access to the underlying server.

## Docker socket boundary

The Dokyr container mounts `/var/run/docker.sock` so it can create, replace, inspect, and remove workloads. Access to this socket is equivalent to host administration.

Only the control-plane container receives the socket. Caddy, PostgreSQL, Registry, Stalwart, and managed workloads do not.

## Authentication and authorization

- The first setup request creates the owner and closes public registration.
- Browser sessions use signed JWTs in HTTP-only cookies.
- Two-factor authentication can protect individual accounts.
- GitHub identity login is separate from repository access.
- Roles and permissions gate project, deployment, user, registry, and infrastructure operations.

## Stored secrets

Dokyr encrypts Git provider credentials, private registry passwords, object-storage secrets, and SMTP credentials with AES-GCM before writing them to PostgreSQL. Keep `ENCRYPTION_KEY` stable, random, and backed up separately.

Registry personal credentials are stored as irreversible hashes. Their cleartext value is shown only once.

## Network boundaries

- `control` connects the platform services privately.
- `selfhost-proxy` lets Caddy reach managed application containers.
- `mail_egress` gives Stalwart Internet access without joining application workloads.
- Managed databases remain private unless an operator explicitly publishes a host port.

## Production checklist

- Replace every development placeholder in `.env`.
- Set `COOKIE_SECURE=true` behind HTTPS.
- Restrict SSH and Dokyr owner access.
- Keep the operating system and Docker Engine patched.
- Back up PostgreSQL and the installation encryption key.
- Test restore and platform rollback paths.
- Review unknown-host rejection and `CONTROL_HOSTS`.
- Expose database ports only when required and firewall them narrowly.
