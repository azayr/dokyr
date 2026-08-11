---
title: Developer mail gateway
description: Configure a verified sending domain and issue SMTP or HTTP API credentials through Dokyr's built-in Stalwart mail service.
---

# Developer mail gateway

Dokyr bundles Stalwart and exposes a small developer mail workflow. Each credential is scoped to a verified sending domain and works as both an SMTP password and a bearer token for the HTTP send endpoint.

## Configure the mail host

Open **Infrastructure → Mail** as the owner and enter the installation domain. Dokyr derives a public mail hostname such as `mail.example.com`.

Before setup:

- create a DNS-only A or AAAA record for the mail hostname;
- configure matching PTR/reverse DNS with the VPS provider;
- allow inbound TCP 25 and 465;
- keep port 80 reachable for the mail hostname's HTTP-01 certificate challenge.

Some VPS providers block port 25 by default. Request removal before treating the server as production-ready for Internet mail.

## Verify a sending domain

Use a dedicated sending subdomain such as `updates.example.com`. Add it in the Mail interface, publish the generated DKIM, SPF, MX, DMARC, and ownership records, then run verification.

After every required record passes, create the one-time credential and save it immediately.

## Send through HTTP

```bash
curl -X POST 'https://panel.example.com/v1/emails' \
  -H 'Authorization: Bearer dkr_mail_YOUR_FULL_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "from": "Dokyr <hello@updates.example.com>",
    "to": ["developer@example.net"],
    "subject": "Hello from Dokyr",
    "text": "Your first Dokyr email works."
  }'
```

A successful response means Stalwart accepted the message; it does not prove recipient delivery.

## Send through SMTP

```dotenv
SMTP_HOST=mail.example.com
SMTP_PORT=465
SMTP_SECURE=true
SMTP_USERNAME=smtp-generated-id@updates.example.com
SMTP_PASSWORD=dkr_mail_YOUR_FULL_API_KEY
SMTP_FROM=hello@updates.example.com
```

Port 465 uses implicit TLS. Use the generated SMTP username, not the Dokyr account email.
