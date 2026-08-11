---
title: Deployment lifecycle
description: Understand Dokyr deployment stages, health verification, promotion, logs, and rollback behavior.
---

# Deployment lifecycle

Dokyr persists a deployment record before it changes a workload. Every meaningful action becomes an ordered event, so the interface can explain both successful and failed releases.

## Image deployment stages

| Stage | What happens |
| --- | --- |
| `prepare` | Validate the project, service, image reference, and runtime configuration |
| `pull` | Ask Docker to fetch the selected image and stream progress |
| `replace` | Preserve the current container as the rollback candidate |
| `create` | Create the replacement with networks, volumes, variables, and labels |
| `start` | Start the replacement container |
| `verify` | Wait for its configured HTTP or command health check |
| `promote` | Route traffic to the healthy replacement through Caddy |
| `rollback` | Restore the previous container when promotion cannot complete safely |
| `complete` | Persist the final status and duration |

## Follow a release

Open **Workspace → Deployments** and select a deployment. The detail view shows stage events and log messages while the release is active. Application logs are available separately from the project service view.

The API exposes the same records for automation. See the [API reference](/API) for project, deployment, and event resources.

## Troubleshooting

When a release fails, start with the first failing stage:

- **Pull:** confirm the image exists and the selected registry credential can read it.
- **Create:** check environment variables, volume paths, container command, and name conflicts.
- **Start:** inspect application logs for configuration or dependency errors.
- **Verify:** run the health endpoint inside the container and confirm the configured port.
- **Promote:** validate the Caddy configuration and ensure the service joined `selfhost-proxy`.

Avoid masking a failed health check by increasing its timeout immediately. First confirm that the endpoint represents application readiness rather than simple process liveness.
