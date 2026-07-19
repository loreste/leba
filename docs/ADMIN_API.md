# Admin API And CLI Reference

Leba exposes runtime operations on the `mode stats` frontend. The same endpoint
serves the built-in HTML UI, JSON stats, metrics text, probes, and server state
changes.

Example stats frontend:

```text
frontend stats
  bind 18404
  mode stats
  admin_users_file /etc/leba/admin-users.conf
```

`admin-users.conf` format:

```text
admin $argon2id$v=19$m=19456,t=2,p=1$SALT$HASH admin
viewer leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX viewer
operator leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX operator
```

`leba admin hash-password PASSWORD` emits Argon2id PHC hashes when the linked
Mako crypto backend supports them. Otherwise it emits the portable
`leba-kdf-v1` format. `leba admin hash-password-legacy PASSWORD` always emits
the portable format.

If credentials are configured, the dashboard, `/stats`, `/metrics`, and
`/admin/*` require HTTP Basic auth **or** a valid `leba_session` cookie
(password form or OIDC SSO). Probe endpoints remain unauthenticated.

**OIDC:** when `oidc { enabled on … }` is configured, operators can use
`GET /oidc/login` → IdP → `GET /oidc/callback` to obtain a session cookie.
See `docs/OIDC.md`.

Roles:

| Role | Access |
|------|--------|
| `viewer` | Dashboard, `/stats`, `/metrics`, `/admin/servers`, stick-tables GET, doctor/explain |
| `operator` | Viewer access plus server state, stick clear, WAF mode, virtual-host changes, `servers_file` reload |
| `admin` | Full admin access (ACME issue/renew, reload, app auth writes) |

## TLS reload (0.14+)

### `POST /admin/tls-reload`

| Field | Meaning |
|-------|---------|
| `tcp_tls_reloaded` | TCP TLS contexts reloaded in place |
| `h3_strategy` | Always `recreate` for HTTP/3 |
| `h3_recreate` | Accept thread rebinds H3 when cert/key paths change |
| `h3_restart_required` | `false` when in-process recreate is available |
| `h3_frontends` | Count of HTTP frontends advertising H3 in `protocols` |

## Stick tables (0.13+)

| Method | Path | Role |
|--------|------|------|
| GET | `/admin/stick-tables` | viewer |
| GET | `/admin/stick-tables/entries?backend=&limit=` | viewer |
| DELETE | `/admin/stick-tables?backend=` | operator |
| DELETE | `/admin/stick-tables/entry?key=` | operator |

## Doctor / preview (0.13+)

| Method | Path | Notes |
|--------|------|-------|
| GET | `/admin/doctor` | Config doctor JSON |
| GET | `/admin/explain?method=&path=&host=&frontend=` | Request dry-run |
| GET/POST | `/admin/preview-reload` | Doctor without apply |
| POST | `/admin/server?backend=&name=&addr=` | Upsert managed server |
| POST | `/admin/server-delete?backend=&name=` | Remove managed server |
| POST | `/admin/waf-mode?frontend=&mode=` | `off` / `detect` / `block` |
| GET | `/admin/waf-rules` | Sample local signatures |

## Probes

### `GET /livez`

Reports that the process is responsive.

Response:

```json
{"live":true}
```

### `GET /readyz`

Reports whether at least one server is alive and not drained.

Ready response:

```json
{"ready":true}
```

Not ready response:

```json
{"ready":false}
```

The not-ready response uses HTTP `503`.

## Server Listing

### `GET /admin/servers`

Returns backend server state.

Example response:

```json
{
  "servers": [
    {
      "backend": "api",
      "name": "api1",
      "addr": "127.0.0.1:19001",
      "alive": 1,
      "drain": 0,
      "conns": 0,
      "weight": 100,
      "total_req": 42
    }
  ]
}
```

Field notes:

| Field | Meaning |
|-------|---------|
| `alive` | `1` means eligible from a health/state perspective. |
| `drain` | `1` means no new traffic should be sent to that server. |
| `conns` | Current active connections tracked by Leba. |
| `total_req` | Requests or TCP sessions assigned to this server. |

## State Changes

State-changing endpoints use:

```text
/admin/{action}/{backend}/{server}
```

Supported actions:

| Action | Effect |
|--------|--------|
| `drain` | Set `drain=1`; new traffic skips this server. |
| `ready` | Set `drain=0` and `alive=1`. |
| `disable` | Set `alive=0`. |
| `enable` | Set `alive=1` and `drain=0`. |

State changes require `operator` or `admin`.

Examples:

```bash
curl -u admin:change-this -X POST \
  http://127.0.0.1:18404/admin/drain/api/api1

curl -u admin:change-this -X POST \
  http://127.0.0.1:18404/admin/ready/api/api1
```

Success response:

```json
{
  "ok": true,
  "action": "drain",
  "backend": "api",
  "server": "api1"
}
```

Error cases:

| Status | Cause |
|--------|-------|
| `400` | Missing backend or server path segment. |
| `404` | Unknown server or unknown action. |
| `405` | State-changing request did not use `POST`. |

## Servers File Reload

### `POST /admin/reload-servers`

Reloads every backend configured with `servers_file`. Existing runtime state is
preserved for servers that remain. Removed servers with active connections are
kept in drain state until their sessions finish.

Requires `operator` or `admin`.

### `POST /admin/reload`

Full **config reload** without process restart: re-reads the main config file,
runs doctor, preserves server runtime state by `(backend, name)`, re-inits pools
for address changes, swaps routes / ACLs / backends / frontends / header rules /
app auth users, OIDC, peers, and **rebinds HTTP/TCP/UDP/H3/stats/peers listen sockets** when host:port
sets change (reuse FD when name+bind unchanged; H3 also reuses when cert/key
paths are unchanged).

TLS PEMs on disk: `/admin/tls-reload` (or cert path change on reused TLS
listeners triggers `tls_server_reload` / SNI add-update-remove). H3 cert path
changes recreate the QUIC listener. **Stats listen host/port/TLS mode** rebinds
on full reload (SIGHUP / `POST /admin/reload`) via `rebind_stats_listener`.

`GET /stats` includes non-secret **`oidc`** and **`peers`** status objects
(`enabled`, `ready`, issuer/cluster/bind, remote_count). Secrets are never
serialized.

Also triggered by **SIGHUP**. File-watch on `servers_file` remains servers-only.

Requires `operator` or `admin`.

### `POST /admin/tls-reload`

Hot-reloads TLS certificates for HTTP frontends (and the stats frontend when TLS
is enabled) from the paths already configured on each frontend (`tls_cert` /
`tls_key` / `tls_sni`). Default PEMs use Mako `tls_server_reload`; SNI set
changes apply in place via `tls_server_sni_add` / `sni_update` / `sni_remove`
(Mako 0.2.2+). Free+recreate only when mTLS or `tls_min` changes. No process
restart for TCP TLS.

HTTP/3 (QUIC) may still require a process restart depending on the linked
quiche build. Response notes this.

Requires `operator` or `admin`. Typical ACME deploy-hook:

```bash
curl -u admin:pass -X POST http://127.0.0.1:8404/admin/tls-reload
```

## Virtual Hosts

Virtual hosts are host routes on an existing HTTP frontend. The admin API can
create or update the route, create the backend if needed, and create or update a
backend server.

### `GET /admin/vhosts`

Returns host routes with their backend members and the HTTP frontends that can
receive vhost mappings.

### `GET /admin/proxy-hosts`

Alias of `GET /admin/vhosts` (NPM-style name).

### `POST /admin/proxy-host`

Alias of `POST /admin/vhost-create` (NPM-style name). Creates or updates a host
route, backend, and server member; persists to `leba.vhosts.conf`.

Optional query parameters:

| Parameter | Notes |
|-----------|-------|
| `cert`, `key` | Absolute paths; installs multi-cert SNI for `domain` and live-reloads TLS |
| `force_ssl` | `1` enables frontend `redirect https`; `0` disables |

### `POST /admin/proxy-host-delete`

Removes a host route. Query parameters: `frontend`, `domain`.

## Certificates (NPM-style)

### `GET /admin/certificates`

Lists frontend TLS material, SNI entries, and PEMs discovered under ACME storage.
Includes helper settings (`webroot`, `storage`, `email`, `helper`, `helper_available`).
Role: **viewer**.

### `POST /admin/certificates/issue`

Runs external **lego** (HTTP-01 webroot), attaches cert as SNI for `domain` on
`frontend` (unless `attach=0`), persists managed vhosts, and triggers live TLS reload.

Query: `domain`, `frontend`, optional `email`, `attach`. Role: **admin**.

Requires `lego` on `PATH` (or `LEBA_ACME_HELPER` / `acme_helper`), plus
`LEBA_ACME_EMAIL` or `acme_email` / query `email`.

### `POST /admin/certificates/renew`

Runs `lego renew` on ACME storage and triggers TLS reload. Role: **admin**.

See `docs/ACME.md`.

## Access lists and app HTTP Basic

### `GET /admin/access-lists`

Returns ACL rules. Role: **viewer**.

### `POST /admin/access-list`

Query: `frontend`, `action` (`allow`|`deny`), `kind` (`src`, `path`, `path_prefix`,
`host`, `method`, `header`), `value`. Live-updates ACLs and writes `leba.access.conf`.
Role: **operator**.

### `POST /admin/access-list-delete`

Same query fields as add. Role: **operator**.

### `GET /admin/http-auth` · `POST /admin/http-auth` · `POST /admin/http-auth-delete` · `POST /admin/http-auth-realm`

Manage app HTTP Basic realms and users (passwords never returned on GET). Writes
require **admin** and trigger **full config reload** so `app_auth_users` reloads.

### `POST /admin/vhost-create`

Query parameters:

| Parameter | Required | Notes |
|-----------|----------|-------|
| `frontend` | yes | Existing HTTP frontend name. New listener sockets are not created live. |
| `domain` | yes | Hostname matched by `route host`. |
| `backend` | yes | Backend to create or update. |
| `server` | yes | Backend server name to create or update. |
| `addr` | yes | Backend server address as `host:port`. |
| `cert` | no | Absolute certificate path. With `domain`, installs multi-cert SNI for that host. |
| `key` | no | Absolute private-key path paired with `cert`. |

Example:

```bash
curl -u admin:change-this -X POST \
  'http://127.0.0.1:18404/admin/vhost-create?frontend=web&domain=app.example.com&backend=app&server=app1&addr=127.0.0.1%3A8080'
```

With a per-host certificate (live multi-cert SNI via Mako `tls_server_sni_add`):

```bash
curl -u admin:change-this -X POST \
  'http://127.0.0.1:18404/admin/vhost-create?frontend=web&domain=app.example.com&backend=app&server=app1&addr=127.0.0.1%3A8080&cert=%2Fetc%2Fleba%2Fapp.crt&key=%2Fetc%2Fleba%2Fapp.key'
```

Successful route/backend/server changes apply to the running process and are
also saved in `leba.vhosts.conf` next to the main config. If the main config
does not already include that file, Leba appends:

```text
include leba.vhosts.conf
```

When `cert`+`key` are provided, Leba upserts `tls_sni <domain> <cert> <key>` and
triggers a live TLS refresh (default cert reload + SNI install). No process
restart is required for TCP TLS.

### `POST /admin/vhost-cert`

Updates certificate paths for an existing frontend. Without a hostname, updates
the default `tls_cert`/`tls_key`. With `hostname` / `sni` / `domain`, upserts a
multi-cert SNI entry.

Query parameters:

| Parameter | Required | Notes |
|-----------|----------|-------|
| `frontend` | yes | Existing frontend name. |
| `cert` | yes | Absolute certificate path. |
| `key` | yes | Absolute private-key path. |
| `hostname` | no | SNI name (exact or `*.example.com`). Aliases: `sni`, `domain`. |

Requires `operator` or `admin`. Live-applied via the same path as
`POST /admin/tls-reload` (Mako multi-cert SNI).

## Stats And Metrics

### `GET /stats`

Returns runtime JSON containing current counters, frontends, routes, ACLs,
backends, servers, and non-secret **`oidc`** / **`peers`** status. The exact
JSON shape is tested in `leba_web_test.mko`.

### `GET /metrics`

Prometheus text metrics for scraping. In addition to request/error/byte
counters and per-server gauges, v0.10.0 exposes:

| Metric | Meaning |
|--------|---------|
| `leba_info{version=…}` | Build version (always 1) |
| `leba_stick_entries` | Stick table entry count |
| `leba_backend_stick` | Backend has `stick on src` |
| `leba_oidc_enabled` / `leba_oidc_ready` | Admin OIDC config |
| `leba_peers_enabled` / `leba_peers_ready` / `leba_peers_remotes` | Stick peers |
| `leba_frontend_tls` / `leba_frontend_sni_entries` | TLS / multi-cert SNI |

## CLI

The CLI wraps the same HTTP endpoints.

```text
leba admin servers [ADDR] [USER:PASS]
leba admin stats [ADDR] [USER:PASS]
leba admin metrics [ADDR] [USER:PASS]
leba admin readyz [ADDR]
leba admin livez [ADDR]
leba admin drain BACKEND SERVER [ADDR] [USER:PASS]
leba admin ready BACKEND SERVER [ADDR] [USER:PASS]
leba admin disable BACKEND SERVER [ADDR] [USER:PASS]
leba admin enable BACKEND SERVER [ADDR] [USER:PASS]
```

Default address:

```text
127.0.0.1:18404
```

Environment defaults:

```bash
export LEBA_ADMIN_ADDR=127.0.0.1:18404
export LEBA_ADMIN_AUTH=admin:change-this
```

Examples:

```bash
leba admin servers
leba admin stats
leba admin drain api api1
leba admin ready api api1
```

The address may include `http://` and a path; the CLI normalizes it to host and
port before connecting.

## Operational Notes

- Runtime state changes persist across restart when `state_file` is configured
  in `defaults`; without it, state changes remain in memory only.
- Draining prevents new assignments; it does not forcibly terminate existing
  client connections.
- A server disabled with `disable` can be made eligible again with `enable`.
- `ready` clears drain and marks the server alive.
- Health checks can later update `alive` according to backend health settings.
