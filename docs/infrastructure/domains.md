---
title: Domains and automatic HTTPS
description: Route domains and URL paths to Dokyr services with Caddy and automatic TLS certificates.
---

# Domains and automatic HTTPS

Dokyr manages application ingress through a separate Caddy container. Caddy receives public traffic while workloads stay on the private `selfhost-proxy` network.

## Prepare DNS

Create an A record for IPv4 and, when applicable, an AAAA record for IPv6:

```text
app.example.com  A  203.0.113.10
```

The record must resolve to the Dokyr host before Caddy can complete an HTTP-01 certificate challenge. If you use a DNS proxy, temporarily switch it to DNS-only while diagnosing issuance.

## Add a domain

Open **Infrastructure → Domains**, create the hostname, and add at least one rule. Each rule selects a target application service and its private container port.

```text
/api/*  → api :8080
/*      → web :3000
```

More specific paths should target the appropriate service. Dokyr validates and applies the complete Caddy configuration atomically, so a bad route cannot partially replace the current working configuration.

## HTTP or automatic HTTPS

Enable HTTPS after DNS reaches the host and ports 80 and 443 are available. Caddy obtains and renews the certificate. HTTP-only mode remains useful for a private network or for a separate upstream proxy that terminates TLS.

## Control-panel hosts

`CONTROL_HOSTS` is the allowlist for control-panel domains. Unknown hostnames receive a 404 instead of being forwarded to the panel. Direct IPv4 access is allowed so a fresh installation remains reachable before DNS is configured.

Application domains belong in the Domains interface. Do not add every application hostname to `CONTROL_HOSTS`.
