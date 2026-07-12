# Leba

Leba is a small Linux load balancer written in
[Mako](https://github.com/loreste/mako).

This project exists to demonstrate what the Mako language can do in a real
systems program: sockets, TLS, HTTP parsing, concurrency, config parsing,
runtime state, tests, and a small built-in operator interface. It is also a
working load balancer for the features described below, but it should be read
as an honest engineering project and a language showcase, not as a finished
platform.

Repository:

```bash
gh repo clone loreste/leba
```

## What It Is

Leba accepts traffic on configured frontends, chooses backend servers, forwards
requests or streams, and exposes runtime controls through a local admin
surface.

Current focus areas:

- HTTP reverse proxying for straightforward request/response traffic.
- TLS termination for HTTP frontends.
- Basic HTTP/2 ingress for simple request/response flows.
- TCP forwarding for database-like services and other stream protocols.
- SIP signaling forwarding over TCP or UDP with `Call-ID` affinity.
- Routing by host, path, path prefix, method, and defaults.
- Backend selection with round-robin, least-connection, weighted, random,
  IP-hash, and SIP `Call-ID` strategies.
- Health checks, rate limits, max connection limits, sticky cookies, and
  request-size limits.
- Runtime server actions: drain, ready, disable, enable, and reload
  `servers_file` entries.
- Built-in stats/admin frontend with RBAC, JSON stats, text metrics, probes,
  and an HTML admin UI.
- Admin vhost setup for domains, backends, backend servers, and certificate
  paths.
- CLI commands for config validation, request explanation, and admin actions.
- Linux service files and deployment examples.

## Why We Built It

Leba was built to push Mako against a practical networking workload.

The goal is to make the language prove itself in code that has to parse config,
open sockets, handle concurrent traffic, call into runtime libraries, maintain
state, expose a UI/API, and survive adversarial tests. The project gives Mako a
concrete benchmark for ergonomics and runtime capability while producing a tool
that can be inspected, run, and improved.

## Status

Leba is early software. It has automated tests and has been exercised on Linux,
but it is still evolving quickly.

Known limits:

- Linux is the intended runtime target. macOS is used for development and
  testing.
- HTTP/2 support is narrow and intended for simple request/response handling.
- HTTP/3 ingress is not implemented.
- Config changes generally require restart. Some admin changes, such as server
  state and vhost route/backend updates, apply live.
- Certificate path changes are saved by the admin UI/API, but replacing an
  active TLS context requires restart.
- SIP support is signaling-focused; media relay is not implemented.
- Operators should test with their own traffic before exposing it to critical
  production paths.

## Build

Install or build Mako first, then clone this repository:

```bash
gh repo clone loreste/leba
cd leba
make build
```

Run the tests:

```bash
make test
```

Validate the sample config:

```bash
./leba doctor configs/leba.conf
```

Run locally:

```bash
./leba -f configs/leba.conf
```

The sample HTTP frontend listens on:

```text
http://127.0.0.1:18080/
```

The sample stats/admin frontend listens on:

```text
http://127.0.0.1:18404/
```

## Configuration

A small HTTP setup:

```text
defaults
  timeout_client 30s
  timeout_server 30s
  timeout_connect 3s
  state_file /var/lib/leba/state
  maxconn 10000
  retries 2
  workers 32

frontend web
  bind 80
  mode http
  route host app.example.com -> app
  route default -> app

backend app
  balance round_robin
  server app1 127.0.0.1:8080 check
  server app2 127.0.0.1:8081 check

frontend stats
  bind 127.0.0.1:18404
  mode stats
  admin_users_file /etc/leba/admin-users.conf
```

See `docs/CONFIG_REFERENCE.md` for the fuller configuration reference.

## Admin

The `mode stats` frontend serves the operator surface:

- `GET /` for the built-in admin UI.
- `GET /stats` for runtime JSON.
- `GET /metrics` for text metrics.
- `GET /readyz` and `GET /livez` for probes.
- `GET /admin/servers` for server state.
- `POST /admin/drain/{backend}/{server}`.
- `POST /admin/ready/{backend}/{server}`.
- `POST /admin/disable/{backend}/{server}`.
- `POST /admin/enable/{backend}/{server}`.
- `POST /admin/reload-servers`.
- `GET /admin/vhosts`.
- `POST /admin/vhost-create`.
- `POST /admin/vhost-cert`.

When admin users are configured, the dashboard, stats, metrics, and admin
actions require HTTP Basic auth. Roles are `viewer`, `operator`, and `admin`.

Example admin user file:

```text
admin $argon2id$v=19$m=19456,t=2,p=1$SALT$HASH admin
viewer leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX viewer
operator leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX operator
```

Generate a password hash:

```bash
./leba admin hash-password 'strong-password'
```

## CLI

```text
leba -f <config> [-n MAX]                 Run the proxy
leba doctor <config>                      Validate config and print fixes
leba check <config>                       Alias for doctor
leba explain <config> METHOD PATH [HOST]  Dry-run routing and ACL decisions
leba admin servers [ADDR] [USER:PASS]     List runtime servers
leba admin stats [ADDR] [USER:PASS]       Fetch /stats
leba admin reload-servers [ADDR] [AUTH]   Reload servers_file backends
leba admin drain BE SRV [ADDR] [AUTH]     Drain a server
leba admin ready BE SRV [ADDR] [AUTH]     Mark server ready
leba admin disable BE SRV [ADDR] [AUTH]   Force DOWN
leba admin enable BE SRV [ADDR] [AUTH]    Force UP
leba admin hash-password PASSWORD         Print a password hash
leba version                              Print version
leba help                                 Print help
```

The admin CLI can target a local port, remote endpoint, NAT, or port-forward:

```bash
export LEBA_ADMIN_ADDR=127.0.0.1:18404
export LEBA_ADMIN_AUTH=admin:strong-password
./leba admin servers
```

## Linux Deployment

Deployment examples live under `deploy/linux/`.

Typical paths:

```text
/usr/local/bin/leba
/etc/leba/leba.conf
/etc/leba/admin-users.conf
/var/lib/leba/state
```

Validate before running:

```bash
leba doctor /etc/leba/leba.conf
```

Then start it with your service manager.

## Documentation

- `docs/CONFIG_REFERENCE.md`
- `docs/ADMIN_API.md`
- `docs/SECURITY.md`
- `docs/PAINPOINTS.md`

## Project Values

Leba tries to be:

- clear in configuration,
- explicit about limits,
- observable by default,
- easy to test,
- useful as a Mako systems-programming example.

The project will keep changing as Mako changes. That is intentional: Leba is
both a tool and a proving ground for the language.
