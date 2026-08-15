---
title: Database clusters
description: Provision shared PostgreSQL, MySQL, and MariaDB clusters, manage logical databases and users, and attach them privately to projects.
---

# Database clusters

Dokyr manages PostgreSQL, MySQL, and MariaDB as global infrastructure. A cluster is not owned by one project: you can attach the same cluster to several project networks while giving each project its own logical database, user, and private hostname.

## Resource model

| Resource | Purpose |
| --- | --- |
| Cluster | One database container, persistent volume, engine, and runtime lifecycle |
| Logical database | An isolated database inside the cluster |
| User and grant | Credentials plus permission to use one or more logical databases |
| Project attachment | Connects the cluster to one project's private network with a project-local hostname |

Project attachments do not copy the cluster or its data. They add the existing cluster container to the project's Docker network, much like connecting two resources through a private VPC.

## Create a cluster

Open **Infrastructure → Databases**, select **Create cluster**, then choose the engine and enter the initial database and administrator details. Private access is the default. Enable public access only when a client outside Dokyr's Docker networks must connect.

The cluster row appears immediately with a **Provisioning** status. Image pulls and container creation continue in the background, so a slow MariaDB or PostgreSQL pull does not hold the creation dialog open. Dokyr records progress as deployment events and updates the row as the cluster moves to **Healthy** or **Failed**.

If Dokyr restarts while a cluster is provisioning, it resumes the queued work. A failed cluster remains visible with its error and can be retried with **Deploy**.

## Manage databases and access

Open a cluster to manage all resources inside it:

- create logical databases and choose their owner;
- create users with a supplied or generated password;
- grant a user access to one or more databases;
- reveal a user's credentials when you need to configure an application;
- revoke grants and remove unused users or databases.

Dokyr protects dependencies. The primary database and cluster administrator cannot be deleted. A database, user, or grant also cannot be removed while a project attachment still uses it. Detach the affected projects first.

## Attach a cluster to a project

Open a project, select its **Databases** tab, then choose **Add database**:

1. Select an existing cluster, or use **Create new** to open the global database page.
2. Select a logical database inside that cluster.
3. Select a user that has a grant for that database.
4. Choose the internal service name that applications in this project will use.

The attachment belongs to the project network, not to one Laravel, Spring Boot, or other application service. Every container in the project can connect through the selected service name. For example, with the alias `database`:

```dotenv
# PostgreSQL
DATABASE_URL=postgresql://app:password@database:5432/app

# MySQL or MariaDB
DATABASE_URL=mysql://app:password@database:3306/app
```

One cluster can serve multiple projects. Each project can use a different database, user, and alias while the cluster keeps one container and one persistent volume.

Detaching removes only the project's private-network connection. It does not delete the logical database, the cluster, its volume, or attachments from other projects.

## Logs and lifecycle

The cluster list and cluster detail page provide two live-follow views:

- **Deployment** follows provisioning activity such as image pulls and container creation.
- **Runtime** follows the database container's latest output.

Both views refresh automatically while open and stay pinned to the newest entry unless you scroll upward. You can pause following, change the line limit, or jump back to the latest entry.

Stopping or restarting a cluster preserves its volume, databases, users, grants, and project attachments. Changing public exposure recreates the container while retaining its data. Deleting the cluster is blocked while it is provisioning or attached to any project; deleting its volume is irreversible.

## Security notes

- Keep clusters private unless public access is required.
- Use a separate logical database and least-privileged user for each application boundary.
- Treat credential endpoints and copied connection URLs as secrets.
- Back up the server and database volumes before destructive changes. See [Backups and restores](/operations/backups).

For automation and response formats, see the [database API](/API#database-clusters).
