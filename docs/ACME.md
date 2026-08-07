# Let's Encrypt (ACME) with Leba

Leba integrates **Let's Encrypt** through the [lego](https://go-acme.github.io/lego/)
ACME client. Issue and renew hit the official ACME v2 directories, attach PEMs as
SNI certificates, and live-reload TLS — no process restart.

| Mode | Directory |
|------|-----------|
| **Production** (default) | `https://acme-v02.api.letsencrypt.org/directory` |
| **Staging** (rate-limit safe) | `https://acme-staging-v02.api.letsencrypt.org/directory` |

Staging certs are **not trusted by browsers** — use them to validate HTTP-01 /
DNS-01 before production.

## Requirements

1. **Email** for LE registration: `acme_email` or `LEBA_ACME_EMAIL`
2. **lego** on PATH (Docker image includes it) or `LEBA_ACME_HELPER=/path/to/lego`
3. **HTTP-01**: public port 80 + `acme_webroot` served by Leba  
   **or DNS-01**: `dns_provider` + provider env (e.g. `CF_DNS_API_TOKEN`)

## Config

```text
defaults
  acme_email ops@example.com
  acme_webroot /var/lib/leba/acme
  acme_storage /var/lib/leba/lego
  acme_helper lego
  # Optional:
  # acme_staging on          # Let's Encrypt staging
  # acme_server staging      # alias: production | staging | full https URL

frontend web
  bind 80
  mode http
  acme_webroot /var/lib/leba/acme
  # optional: redirect https  (or per-host force_ssl)
  route default -> app
```

### Environment

| Variable | Meaning |
|----------|---------|
| `LEBA_ACME_EMAIL` | Registration email (required to issue) |
| `LEBA_ACME_WEBROOT` | HTTP-01 token directory |
| `LEBA_ACME_STORAGE` | lego account + cert storage (`--path`) |
| `LEBA_ACME_HELPER` | lego binary (default `lego`) |
| `LEBA_ACME_STAGING=1` | Use LE staging directory |
| `LEBA_ACME_SERVER` | Full ACME directory URL, or `staging` / `letsencrypt` |
| `LEBA_ACME_DNS_PROVIDER` | Default DNS-01 provider name |

## NPM-style: host + cert in one call

```bash
# Production Let's Encrypt + Force SSL + SNI attach + live reload
curl -u admin:secret -X POST \
  'http://127.0.0.1:8404/admin/proxy-host?frontend=web&domain=app.example.com&backend=app&server=s1&addr=127.0.0.1:3000&ssl=1&force_ssl=1'

# Staging first (no rate limits)
curl -u admin:secret -X POST \
  'http://127.0.0.1:8404/admin/certificates/issue?domain=app.example.com&frontend=web&staging=1&attach=1'
```

Admin UI:

- **Proxy Hosts → + Add** with **Request SSL**
- **Certificates** (Let's Encrypt) tab: inventory, staging checkbox, issue, renew

## API

```text
GET  /admin/certificates
POST /admin/certificates/issue?domain=&frontend=&email=&attach=1&staging=0|1&server=&challenge=http|dns&dns_provider=
POST /admin/certificates/renew
```

`GET /admin/certificates` includes:

```json
"settings": {
  "provider": "letsencrypt",
  "ca": "letsencrypt",
  "server": "https://acme-v02.api.letsencrypt.org/directory",
  "staging": false,
  "ready": true,
  "issues": []
}
```

Issued PEMs:

```text
{acme_storage}/certificates/{domain}.crt
{acme_storage}/certificates/{domain}.key
```

## HTTP-01 challenge serving

Leba serves:

```text
GET /.well-known/acme-challenge/<token>
  → file {acme_webroot}/<token>
```

This path **bypasses** HTTPS redirect (including per-host `force_ssl`), rate limits,
and ACLs so Let's Encrypt can complete validation on port 80.

## Renew

Daily systemd timer (Linux package):

```bash
systemctl enable --now leba-acme-renew.timer
# uses LEBA_ADMIN_AUTH + LEBA_ADMIN_URL from /etc/leba/leba.env
```

Manual:

```bash
curl -u admin:secret -X POST http://127.0.0.1:8404/admin/certificates/renew
# then TLS reload is triggered when the API returns tls_reload:true
```

## DNS-01

```bash
export CF_DNS_API_TOKEN=…
curl -u admin:secret -X POST \
  'http://127.0.0.1:8404/admin/certificates/issue?domain=*.example.com&frontend=web&challenge=dns&dns_provider=cloudflare&attach=1'
```

## Preflight errors

| Code | Meaning |
|------|---------|
| `missing_helper` | Install lego or set `LEBA_ACME_HELPER` |
| `missing_email` | Set `acme_email` / `LEBA_ACME_EMAIL` |
| `invalid_domain` | Domain failed safety validation |
| `invalid_webroot` / `invalid_storage` | Path empty or unsafe |
| `missing_dns_provider` | DNS-01 without provider |
| `no_certs` | Renew with empty storage |
| `lego_failed` | Helper ran but PEMs missing |

## Manual lego (same directories)

```bash
# Production
lego --accept-tos --email ops@example.com \
  --server https://acme-v02.api.letsencrypt.org/directory \
  --http --http.webroot /var/lib/leba/acme \
  --path /var/lib/leba/lego --domains app.example.com run

curl -u operator:secret -X POST http://127.0.0.1:8404/admin/tls-reload
```

Sample hook: `deploy/docker/lego-deploy-hook.sh`.

## Docker

```bash
LEBA_ADMIN_AUTH=admin:change-me LEBA_SESSION_SECRET=long-secret \
  LEBA_ACME_EMAIL=ops@example.com \
  docker compose up
```

The image installs **lego** so Admin UI issue works when port 80 is reachable
for HTTP-01.
