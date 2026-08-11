---
title: What is Dokyr?
description: Learn what Dokyr does, who it is for, and where its deliberately single-server architecture fits.
---

# What is Dokyr?

Dokyr is an open-source deployment control plane for a single Docker host. It gives one VPS the operational conveniences people expect from a platform as a service: projects, releases, domains, automatic HTTPS, databases, a private registry, object storage connections, backups, and developer mail.

The platform is intentionally small. A Go service owns the control plane, an embedded Svelte application provides the interface, PostgreSQL stores platform state, Caddy routes traffic, and Dokyr talks to the host Docker Engine through its Unix socket.

## When Dokyr fits

Dokyr works well for:

- independent products running on a VPS;
- agencies hosting isolated client applications;
- internal tools that do not justify a cluster;
- small teams that want a visible, inspectable deployment pipeline;
- developers who want platform ergonomics without giving up infrastructure ownership.

## What it manages

| Area | Dokyr provides |
| --- | --- |
| Applications | Image-based services, Compose imports, encrypted environment variables, health checks, logs, and release history |
| Traffic | Caddy routes, hostnames, path rules, HTTP-only or automatic HTTPS ingress |
| Data | Private PostgreSQL, MySQL, and MariaDB services with persistent volumes |
| Images | A private Docker Distribution registry with scoped access tokens |
| Storage | Reusable S3-compatible connections for registry data and server backups |
| Mail | Verified sending domains and domain-scoped SMTP/HTTP credentials through Stalwart |
| Operations | Host metrics, release updates, scheduled backups, restore jobs, users, and role-based permissions |

## Current boundaries

Dokyr is a **single-node control plane**. It does not currently provide a worker fleet, clustering, high availability, or a general-purpose scheduler. Repository discovery is available, but repository cloning and builds are not yet part of the supported deployment path; deploy container images instead.

Access to the Docker socket is equivalent to administrative access to the host. Dokyr mounts it only into the control-plane container. Read the [security model](/operations/security) before exposing a new installation publicly.

## Next steps

1. [Install Dokyr on a VPS](/guide/installation).
2. [Create your first project](/guide/first-project).
3. [Attach a domain and enable HTTPS](/infrastructure/domains).
4. Read the [complete architecture](/ARCHITECTURE) when you need implementation details.
