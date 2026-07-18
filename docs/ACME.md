# ACME / Let's Encrypt with Leba

Leba does **not** embed a pure-Mako ACME client (no JOSE account signing in Mako).
It orchestrates **lego** (or certbot/acme.sh) and live-reloads TLS.

## NPM-style path (recommended)

1. Configure `acme_webroot` (and optionally `acme_storage` / `acme_email`) or env:
   - `LEBA_ACME_EMAIL` — registration email (**required** for issue)
   - `LEBA_ACME_STORAGE` — lego `--path` (default `/var/lib/leba/lego`)
   - `LEBA_ACME_WEBROOT` — HTTP-01 challenge dir (default `/var/lib/leba/acme`)
   - `LEBA_ACME_HELPER` — binary name/path (default `lego`)
2. Ensure the data-plane frontend serves HTTP-01 (port 80 publicly).
3. Open **Admin UI → Certificates**, or call the API as an **admin** user:

```bash
# Issue + attach SNI cert + live tls_server_reload
curl -u admin:secret -X POST \
  'http://127.0.0.1:8404/admin/certificates/issue?domain=app.example.com&frontend=web&email=ops@example.com'

# Renew everything under acme_storage, then reload TLS
curl -u admin:secret -X POST \
  http://127.0.0.1:8404/admin/tls-reload   # after renew
curl -u admin:secret -X POST \
  http://127.0.0.1:8404/admin/certificates/renew
```

```text
GET  /admin/certificates
POST /admin/certificates/issue?domain=&frontend=&email=&attach=1
POST /admin/certificates/renew
```

Issued PEMs land at:

```text
{acme_storage}/certificates/{domain}.crt
{acme_storage}/certificates/{domain}.key
```

## HTTP-01 challenge serving

```text
defaults
  acme_webroot /var/lib/leba/acme
  acme_storage /var/lib/leba/lego
  acme_email ops@example.com
  acme_helper lego

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
  → file {acme_webroot}/<token>
```

This path **bypasses** HTTPS redirect, rate limits, and ACLs. Token body is
never written to access logs as content (only the path appears).

## Manual lego + deploy hook

```bash
lego --email ops@example.com --http --http.webroot /var/lib/leba/acme \
  --path /var/lib/leba/lego --domains app.example.com run

curl -u operator:secret -X POST http://127.0.0.1:8404/admin/tls-reload
# or
leba admin tls-reload 127.0.0.1:8404 operator:secret
```

Sample hook: `deploy/docker/lego-deploy-hook.sh`.

## Docker

```bash
docker compose build
LEBA_ADMIN_AUTH=admin:change-me LEBA_SESSION_SECRET=long-secret \
  LEBA_ACME_EMAIL=ops@example.com \
  docker compose up
```

The image installs **lego** so Admin UI issue works when port 80 is reachable
for HTTP-01. Optional profile `acme` runs a lego sidecar for CLI workflows.

## Attach without ACME

```bash
curl -u operator:secret -X POST \
  'http://127.0.0.1:8404/admin/vhost-cert?frontend=web&hostname=app.example.com&cert=/path/fullchain.pem&key=/path/privkey.pem'
```

## Preflight errors (0.11.1+)

`POST /admin/certificates/issue` and `renew` validate before shelling out to lego.
Failures return **HTTP 400** with:

```json
{"error":"human message","code":"missing_email"}
```

| Code | Meaning |
|------|---------|
| `missing_helper` | `lego` not on PATH; set `LEBA_ACME_HELPER` or use Docker image |
| `missing_email` | Set `acme_email` / `LEBA_ACME_EMAIL` or query `email=` |
| `invalid_domain` | Domain failed safety validation |
| `invalid_webroot` / `invalid_storage` | Path empty or unsafe |
| `no_certs` | Renew with empty storage |
| `lego_failed` | Helper ran but PEMs missing (often 500) |

`GET /admin/certificates` includes `settings.ready`, `settings.issues[]`, and per-cert `not_after` (via `openssl x509 -enddate` when available).
