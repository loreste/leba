# Proxy hosts (NPM mental model)

Leba maps **proxy hosts** to `route host` lines (plus optional locations, ACLs, and Basic auth). Admin UI and API write `leba.vhosts.conf` / `leba.access.conf`.

## Host modes

| Mode | Config / API | Behavior |
|------|----------------|----------|
| **Proxy** (default) | `route host app.example.com -> backend` | Reverse proxy |
| **Disabled** | `route host app.example.com dead 503` or `enable=0` | Immediate 503 |
| **Dead** | `action=dead` | 404 (or custom code) |
| **Redirect** | `action=redirect&target=URL` | 301/302 to target |
| **Force SSL** | frontend `redirect https` / `force_ssl=1` | Cleartext → HTTPS |

## WebSocket

Default: **allowed** (Upgrade pass-through).
Disable: `websocket off` on the host (or location) route → **403** for `Upgrade: websocket`.

```text
route host app.example.com -> app websocket off
```

## Custom locations

Path prefixes under a domain (checked **before** the host catch-all):

```text
route path_prefix /api host app.example.com -> api
route host app.example.com -> app
```

API:

```bash
POST /admin/proxy-host-location?frontend=web&domain=app.example.com&path=/api&backend=api&server=a1&addr=127.0.0.1:3001
POST /admin/proxy-host-location-delete?frontend=web&domain=app.example.com&path=/api
```

## Host IP access list

ACL kind `host_src` with value `domain|ip-prefix`:

```text
deny host_src app.example.com|203.0.113.
allow host_src app.example.com|10.0.
```

API: `/admin/host-access-list` and `-delete`.

## Host HTTP Basic

```text
auth_user alice s3cret host app.example.com
```

API (admin, triggers full reload):

```bash
POST /admin/host-http-auth?frontend=web&domain=app.example.com&user=alice&pass=s3cret
```

## Certificates

- HTTP-01: `challenge=http` (default)
- DNS-01: `challenge=dns&dns_provider=cloudflare` (+ lego env e.g. `CF_DNS_API_TOKEN`)
  or `LEBA_ACME_DNS_PROVIDER`

Daily renew timer: `deploy/linux/leba-acme-renew.timer` + `.service`.
