# ACME / Let's Encrypt with Leba

Leba does **not** embed a full ACME client (Mako lacks JOSE account signing).
Use **lego**, **acme.sh**, or **certbot** to issue/renew certificates, then
tell Leba to live-reload TLS.

## HTTP-01 challenge serving

Configure a challenge directory:

```text
defaults
  acme_webroot /var/lib/leba/acme

# or per-frontend:
frontend web
  bind 80
  mode http
  acme_webroot /var/lib/leba/acme
  redirect https
  route default -> app
```

Leba serves:

```text
GET /.well-known/acme-challenge/<token>
  → file /var/lib/leba/acme/<token>
  Content-Type: text/plain
```

This path **bypasses** HTTPS redirect, rate limits, and ACLs. Token body is
never written to access logs as content (only the path appears).

Point your ACME tool's HTTP-01 webroot at the same directory.

## Live TLS reload after renew

```bash
# After PEMs are written to the paths in leba.conf:
curl -u operator:secret -X POST http://127.0.0.1:8404/admin/tls-reload

# or
leba admin tls-reload 127.0.0.1:8404 operator:secret
```

Stats/admin should be on localhost or a private network. Use
`LEBA_ADMIN_AUTH=user:pass` for deploy hooks.

## Sample lego deploy hook

See `deploy/docker/lego-deploy-hook.sh`. Minimal:

```bash
#!/bin/sh
set -eu
curl -fsS -u "$LEBA_ADMIN_AUTH" -X POST \
  "${LEBA_ADMIN_URL:-http://127.0.0.1:8404}/admin/tls-reload"
```

## Docker

```bash
docker compose build
LEBA_ADMIN_AUTH=admin:change-me LEBA_SESSION_SECRET=long-secret \
  docker compose up
# optional ACME helper profile:
# docker compose --profile acme up
```

Mount issued certs under `./certs` and set `tls_cert` / `tls_key` in config to
match. After renew, run the deploy hook.
