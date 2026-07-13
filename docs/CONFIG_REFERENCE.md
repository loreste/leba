# Config Reference

Leba config is line-oriented. Indentation is optional and used only for
readability. Blank lines and lines beginning with `#` are ignored. Inline
comments are supported when the comment begins with a space followed by `#`.

The top-level sections are:

```text
defaults
frontend NAME
backend NAME
listen NAME PORT [MODE]
include PATH
```

`listen` is shorthand for creating a frontend with a bind port.

`include PATH` appends another config file before validation. Relative include
paths are resolved from the main config directory. The admin vhost workflow uses
`include leba.vhosts.conf` for persisted host routes, backend members, and saved
certificate paths.

## Defaults

Defaults apply to runtime behavior unless a frontend, backend, or server gives a
more specific value.

```text
defaults
  timeout_client 30s
  timeout_server 30s
  timeout_connect 3s
  maxconn 10000
  retries 2
  workers 32
```

Supported keys:

| Key | Value | Notes |
|-----|-------|-------|
| `timeout_client` | duration | Client idle/read timeout. |
| `timeout_client_ms` | integer | Same value in milliseconds. |
| `timeout_server` | duration | Upstream idle/read timeout. |
| `timeout_server_ms` | integer | Same value in milliseconds. |
| `timeout_connect` | duration | Upstream connect timeout. |
| `timeout_connect_ms` | integer | Same value in milliseconds. |
| `state_file` | path | Optional runtime server state persistence file. |
| `maxconn` | integer | Default frontend max connections. |
| `retries` | integer | Default upstream retry count. |
| `workers` | integer | Worker slot count, clamped at runtime. |
| `request_body_limit` | size | Default raw HTTP request limit, e.g. `1MB`. |
| `request_body_limit_bytes` | integer | Same value in bytes. |

Durations accept millisecond and second style values such as `500ms`, `2s`, or
plain integer milliseconds.

## Frontends

Frontends define listener ports and request handling mode.

```text
frontend web
  bind 18080
  mode http
  maxconn 5000
  rate_limit 5000/s
  request_body_limit 1MB
  sticky cookie LEBAID
  xff on
  access_log on
  deny path_prefix /admin/secret
  route path_prefix /api -> api
  route default -> web
```

Supported keys:

| Key | Value | Notes |
|-----|-------|-------|
| `bind` | `PORT`, `:PORT`, or `HOST:PORT` | Current runtime binds by port. |
| `mode` | `http`, `tcp`, `udp`, `stats` | Listener mode. UDP is currently SIP signaling only. |
| `maxconn` | integer | Frontend pressure limit. |
| `rate_limit` | rate | Example: `5000/s`. |
| `request_body_limit` | size | Raw HTTP request limit for this frontend. |
| `request_body_limit_bytes` | integer | Same value in bytes. |
| `sticky cookie` | name | Enables HTTP sticky cookie selection. |
| `xff` | `on` or `off` | Enables/disables forwarded-for handling where supported. |
| `access_log` | `on` or `off` | Controls access log lines for this frontend. |
| `tls_cert` | path | Enables TLS termination for this HTTP frontend. |
| `tls_key` | path | Private key for `tls_cert`. |
| `protocols` / `alpn` | list | `http/1.1`, `h2`, and `h3` (aliases `http/2`, `http/3`, `quic`). `h2`/`h3` require TLS certs. `h3` also requires a quiche-linked build (`h3_server_available`). |
| `redirect https` | optional code | Force HTTP→HTTPS with `301` (default) or `302`/`307`/`308`. No backend required. Example: `redirect https` or `redirect https 308`. |
| `auth` | `user:pass[:role]` | Backward-compatible stats admin credential. Role defaults to `admin`. |
| `admin_user` | `user pass role` | Additional stats user. Role is `viewer`, `operator`, or `admin`. |
| `auth_hash` | `user hash role` | Stats admin credential using Argon2id PHC, `leba-kdf-v1`, or legacy SHA-256. |
| `admin_user_hash` | `user hash role` | Additional hashed stats user. |
| `admin_users_file` | path | External hashed stats users file. |
| `default_backend` | backend name | Default backend for HTTP/TCP frontend. |
| `backend` | backend name | Compatibility shorthand for default backend. |
| `route` | match expression | HTTP routing rule. |
| `allow` | match expression | HTTP allow ACL. |
| `deny` | match expression | HTTP deny ACL. |

Use `default_backend NAME` for TCP frontends:

```text
frontend db
  bind 13306
  mode tcp
  default_backend db
```

Use `mode stats` for the admin UI/API:

```text
frontend stats
  bind 18404
  mode stats
  admin_users_file /etc/leba/admin-users.conf
```

`admin-users.conf`:

```text
admin $argon2id$v=19$m=19456,t=2,p=1$SALT$HASH admin
viewer leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX viewer
operator leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX operator
```

Generate hashes with:

```bash
leba admin hash-password 'strong-password'
```

When Argon2id is not available in the linked Mako crypto backend, the command
falls back to `leba-kdf-v1`. Use `leba admin hash-password-legacy` to force the
portable format.

Plaintext `auth` and `admin_user` remain supported for local development and
compatibility.

## Routes

Routes are evaluated in config order. First match wins.

```text
route path_prefix /api -> api
route host api.example -> api
route path /health -> health
route default -> web
```

The arrow is optional:

```text
route path_prefix /api api
route default web
```

Common match kinds:

| Kind | Meaning |
|------|---------|
| `default` | Fallback route. |
| `path` | Exact path match. |
| `path_prefix` | Path prefix match. |
| `path_suffix` | Path suffix match. |
| `path_contains` | Path substring match. |
| `host` | Host match, with port normalized. |
| `method` | HTTP method match. |

## ACLs

ACLs use the same match style as routes.

```text
deny path_prefix /admin/secret
allow host internal.example
```

Deny rules take precedence when a request matches a deny condition. Empty match
values are rejected by `leba doctor`.

## Backends

Backends define balancing policy, health behavior, and servers.

```text
backend api
  balance least_conn
  maxconn 1000
  health_path /health
  health_interval 2s
  health_timeout 1s
  health_rise 2
  health_fall 3
  connect_timeout 3s
  retries 2
  server api1 127.0.0.1:19001 weight 100 maxconn 500 check
  server api2 127.0.0.1:19002 weight 100 maxconn 500 check
```

Supported keys:

| Key | Value | Notes |
|-----|-------|-------|
| `balance` | policy | `round_robin`, `least_conn`, `ip_hash`, `sip_call_id`, `weighted`, `random`. |
| `servers_file` | path | Loads server lines at startup and via protected admin reload. |
| `health_path` | path or `tcp` | HTTP path or TCP connect probe. |
| `health_interval` | duration | Probe interval. |
| `health_interval_ms` | integer | Probe interval in milliseconds. |
| `health_timeout` | duration | Probe timeout. |
| `health_timeout_ms` | integer | Probe timeout in milliseconds. |
| `health_rise` | integer | Successes required to mark healthy. |
| `health_fall` | integer | Failures required to mark unhealthy. |
| `connect_timeout` | duration | Upstream connect timeout. |
| `connect_timeout_ms` | integer | Same value in milliseconds. |
| `maxconn` | integer | Backend-level pressure limit. |
| `retries` | integer | Upstream retry count. |
| `server` | server line | Adds a backend member. |

TCP health check example:

```text
backend db
  balance least_conn
  health_path tcp
  server db1 10.0.40.11:3306 weight 100 maxconn 1000 check
```

SIP affinity examples:

```text
frontend sip_tcp
  bind 15060
  mode tcp
  default_backend sip

frontend sip_udp
  bind 5060
  mode udp
  default_backend sip

backend sip
  balance sip_call_id
  health_path tcp
  server sip1 10.0.50.11:5060 weight 100 maxconn 1000 check
  server sip2 10.0.50.12:5060 weight 100 maxconn 1000 check
```

`sip_call_id` inspects the first TCP payload or UDP datagram, extracts `Call-ID` or compact
`i`, and hashes that value for stable server selection. This is for SIP
signaling. RTP/media relay is not implemented.

## Servers

Server lines:

```text
server NAME HOST:PORT [weight N] [maxconn N] [check|no_check]
```

Examples:

```text
server api1 127.0.0.1:19001 weight 100 maxconn 500 check
server api2 127.0.0.1:19002 weight 50 no_check
```

Defaults:

| Field | Default |
|-------|---------|
| `weight` | `100` |
| `maxconn` | `0`, meaning no server-specific cap |
| health check flag | `check` |

## Server Files

`servers_file` loads backend members at startup and can be reloaded manually.

```text
backend api
  balance least_conn
  servers_file ./api.servers
```

File format:

```text
api1 127.0.0.1:19001 weight 100 maxconn 500 check
server api2 127.0.0.1:19002 weight 100 maxconn 500 no_check
```

Reload command:

```bash
leba admin reload-servers 127.0.0.1:18404 admin:change-this
```

The reload preserves runtime state for retained servers. Removed servers with
active sessions are kept in drain state until those sessions complete. There is
no automatic file watcher yet.

The leading `server` keyword is optional in server files. Relative paths resolve
relative to the config file directory.

## Validation

Run:

```bash
leba doctor /path/to/leba.conf
```

Doctor checks for errors such as:

- missing frontends or backends,
- routes pointing at unknown backends,
- empty backends,
- unknown balance policies,
- port conflicts,
- TCP frontends without a backend,
- empty ACL/route match values,
- unreadable `servers_file`,
- missing TLS files,
- placeholder or missing admin credentials.
