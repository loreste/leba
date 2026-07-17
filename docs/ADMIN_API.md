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
`/admin/*` require HTTP Basic auth. Probe endpoints remain unauthenticated.

Roles:

| Role | Access |
|------|--------|
| `viewer` | Dashboard, `/stats`, `/metrics`, `/admin/servers` |
| `operator` | Viewer access plus server state changes, virtual-host changes, and `servers_file` reload |
| `admin` | Full admin access |

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

Full **config table swap** without process restart: re-reads the main config
file, runs doctor, preserves server runtime state by `(backend, name)`, re-inits
pools for address changes, and swaps routes / ACLs / backends / frontends /
header rules / app auth users.

**Not included in v1:** opening or closing listen sockets (bind changes still
need a process restart). TLS material on disk still uses `/admin/tls-reload`.

Also triggered by **SIGHUP**. File-watch on `servers_file` remains servers-only.

Requires `operator` or `admin`.

### `POST /admin/tls-reload`

Hot-reloads TLS certificates for HTTP frontends (and the stats frontend when TLS
is enabled) from the paths already configured on each frontend (`tls_cert` /
`tls_key`). Uses Mako `tls_server_reload` — no process restart for TCP TLS.

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

### `POST /admin/proxy-host-delete`

Removes a host route. Query parameters: `frontend`, `domain`.

### `POST /admin/vhost-create`

Query parameters:

| Parameter | Required | Notes |
|-----------|----------|-------|
| `frontend` | yes | Existing HTTP frontend name. New listener sockets are not created live. |
| `domain` | yes | Hostname matched by `route host`. |
| `backend` | yes | Backend to create or update. |
| `server` | yes | Backend server name to create or update. |
| `addr` | yes | Backend server address as `host:port`. |
| `cert` | no | Absolute certificate path for the frontend. |
| `key` | no | Absolute private-key path for the frontend. |

Example:

```bash
curl -u admin:change-this -X POST \
  'http://127.0.0.1:18404/admin/vhost-create?frontend=web&domain=app.example.com&backend=app&server=app1&addr=127.0.0.1%3A8080'
```

Successful route/backend/server changes apply to the running process and are
also saved in `leba.vhosts.conf` next to the main config. If the main config
does not already include that file, Leba appends:

```text
include leba.vhosts.conf
```

Certificate path changes are saved, but the already-created TLS context is not
replaced live; restart the process for a new certificate/key to take effect.

### `POST /admin/vhost-cert`

Updates saved certificate paths for an existing frontend.

Query parameters:

| Parameter | Required | Notes |
|-----------|----------|-------|
| `frontend` | yes | Existing frontend name. |
| `cert` | yes | Absolute certificate path. |
| `key` | yes | Absolute private-key path. |

Requires `operator` or `admin`.

## Stats And Metrics

### `GET /stats`

Returns runtime JSON containing current counters, frontends, routes, ACLs,
backends, and servers. The exact JSON shape is tested in `leba_web_test.mko`.

### `GET /metrics`

Returns text metrics for scraping or command-line inspection.

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
