# Native ACME / Free TLS

Leba issues and renews certificates with a native Mako ACME v2 client. The default provider is Let's Encrypt production; staging and custom ACME directory URLs are supported.

## Providers

| Mode | Directory |
|------|-----------|
| Let's Encrypt production | `https://acme-v02.api.letsencrypt.org/directory` |
| Let's Encrypt staging | `https://acme-staging-v02.api.letsencrypt.org/directory` |
| Custom ACME CA | Set `acme_server` or `LEBA_ACME_SERVER` |

The native client uses Mako HTTPS, ES256 JWS, a generated P-256 account key, HTTP-01 challenge files, CSR creation, certificate download, SNI attach, and live TLS reload. No nginx, HAProxy, certbot, or lego process is required for HTTP-01.

## Requirements

1. `acme_email` or `LEBA_ACME_EMAIL` for ACME account registration.
2. Public port 80 for HTTP-01 validation.
3. `acme_webroot` on the HTTP frontend serving `/.well-known/acme-challenge/*`.
4. `acme_storage` writable by the Leba process.

## Config

```text
defaults
  acme_email ops@example.com
  acme_webroot /var/lib/leba/acme
  acme_storage /var/lib/leba/acme-state
  acme_helper native
  # acme_staging on
  # acme_server staging

frontend web
  bind 80
  mode http
  acme_webroot /var/lib/leba/acme
  route default -> app
```

## Environment

| Variable | Meaning |
|----------|---------|
| `LEBA_ACME_EMAIL` | Registration email |
| `LEBA_ACME_WEBROOT` | HTTP-01 token directory |
| `LEBA_ACME_STORAGE` | Native account/certificate storage |
| `LEBA_ACME_HELPER` | `native` by default; external helper is legacy compatibility |
| `LEBA_ACME_STAGING=1` | Use Let's Encrypt staging |
| `LEBA_ACME_SERVER` | Full ACME directory URL, or `staging` / `letsencrypt` |

## One-Call Host + Cert

```bash
curl -u admin:secret -X POST   'http://127.0.0.1:8404/admin/proxy-host?frontend=web&domain=app.example.com&backend=app&server=s1&addr=127.0.0.1:3000&ssl=1&force_ssl=1'
```

This creates/updates the proxy host, issues or reuses the certificate, attaches it as SNI, and marks the host as Force SSL.

## API

```text
GET  /admin/certificates
POST /admin/certificates/issue?domain=&frontend=&email=&attach=1&staging=0|1&server=&challenge=http
POST /admin/certificates/renew
```

Issued PEMs are stored as:

```text
{acme_storage}/accounts/account-p256.key
{acme_storage}/accounts/account.url
{acme_storage}/certificates/{domain}.crt
{acme_storage}/certificates/{domain}.key
```

## HTTP-01

```text
GET /.well-known/acme-challenge/<token>
  -> {acme_webroot}/<token>
```

Challenge paths bypass HTTPS redirect, ACLs, rate limiting, and app Basic auth so the ACME CA can validate the domain.

## Renew

Native renew re-issues the certificates already present under `{acme_storage}/certificates` and triggers TLS reload through the admin path.

```bash
curl -u admin:secret -X POST http://127.0.0.1:8404/admin/certificates/renew
```

## DNS-01

Native DNS-01 provider adapters are intentionally not enabled yet. For wildcard certificates, either use HTTP-01 on concrete hostnames or configure a legacy external helper explicitly while native DNS adapters are added.

## Testing

Local gates validate the native ACME plumbing without contacting a live CA:

```bash
make doctor
make test-full
mako test leba_web_test.mko --backend c
```

`leba_web_test.mko` covers native account helpers, JWS construction, and CSR DER/base64url encoding. A real Let's Encrypt staging issuance additionally requires a public DNS name pointing at Leba and public port 80 reachability for HTTP-01.

## Preflight Errors

| Code | Meaning |
|------|---------|
| `missing_email` | Set `acme_email` or `LEBA_ACME_EMAIL` |
| `invalid_domain` | Domain failed safety validation |
| `invalid_webroot` / `invalid_storage` | Path empty or unsafe |
| `account_key_failed` | Could not generate/read native P-256 account key |
| `directory_failed` / `directory_invalid` | ACME directory fetch failed or lacked required endpoints |
| `nonce_failed` | ACME server did not return `Replay-Nonce` |
| `challenge_not_valid` | HTTP-01 did not validate before timeout |
| `certificate_download_failed` | ACME order completed but certificate download failed |
