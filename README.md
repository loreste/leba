# Leba

Leba is a load balancer written in [Mako](https://github.com/loreste/mako),
showcasing what the language can do in a real systems program.

```bash
gh repo clone loreste/leba
```

## Features

### Load Balancing
- Round-robin, least-connection, IP-hash, weighted, random, SIP Call-ID,
  and consistent-hash algorithms
- Sticky cookie session persistence
- Per-server weight and maxconn limits
- Per-backend maxconn limits
- Connection draining (graceful removal from pool)

### Protocol Support
- HTTP/1.1 reverse proxy
- HTTP/2 over TLS (ALPN `h2`) with stream multiplexing
- HTTP/3/QUIC ingress (requires quiche-linked Mako build)
- WebSocket tunneling (Upgrade: websocket pass-through)
- TCP forwarding (databases, Redis, etc.)
- UDP/SIP signaling with Call-ID affinity
- HTTPS redirect (`redirect https [code]`)
- Static file serving (`root /path/to/files`)
- CORS preflight handling (auto 204 for OPTIONS)

### TLS & Security
- TLS termination for HTTP frontends
- mTLS with client certificate validation (`tls_client_ca`)
- TLS on the admin/stats frontend
- Encrypted state file at rest (AES-128-GCM via `state_key`)
- ACL engine: deny/allow by path, host, method, header, source IP
- IP allowlist/blocklist via `src` ACL rules
- Per-frontend and per-client-IP rate limiting (token bucket)
- Request body size limits
- Directory traversal prevention for static file serving
- RBAC for admin API (viewer, operator, admin roles)
- Password hashing (argon2id, SHA-256, PBKDF2)
- No auto-login: unconfigured auth denies access
- WWW-Authenticate header on 401 responses (browser login dialog)

### Health Checks
- Active HTTP path probes with configurable interval and timeout
- Active TCP connect probes
- Rise/fall thresholds to prevent flapping
- Passive health detection (auto-mark DOWN after consecutive 5xx)

### Observability
- JSON structured logs with RFC 3339 timestamps
- Access logs to stdout and optional file (`access_log_file`)
- W3C traceparent propagation (UUID v7 trace IDs)
- Prometheus text metrics (`/metrics`)
- JSON stats API (`/stats`)
- Admin audit logging with request IDs and roles
- Kubernetes-style probes (`/readyz`, `/livez`)

### Admin & Operations
- Built-in web admin dashboard (SPA)
- REST API for drain, ready, disable, enable, reload
- Vhost management API (create domains, backends, certificates)
- Config doctor with validation and fix suggestions
- Request explainer (dry-run routing decisions)
- CLI for all admin operations
- Hot reload via SIGHUP (servers_file changes)
- File watch for automatic servers_file reload
- Cooperative drain on SIGTERM/SIGINT (session cancel tokens)
- Process-level connection budget (Limits API)
- Runtime state persistence with optional encryption

### Configuration
- Line-oriented config with section-based grammar
- Environment variable expansion (`$VAR` and `${VAR}`)
- Include directive for multi-file configs
- Duration parsing (`30s`, `2m`), size parsing (`1MB`), rate parsing (`1000/s`)
- Header manipulation rules (`request_header_set`, `response_header_set`,
  `request_header_del`, `response_header_add`)

## Build

```bash
gh repo clone loreste/leba
cd leba
make build
make test
```

## Quick Start

```bash
./leba doctor configs/leba.conf   # validate config
./leba -f configs/leba.conf       # run
```

Sample frontends:
- HTTP: `http://127.0.0.1:18080/`
- Admin: `http://127.0.0.1:18404/`

## Configuration Example

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
  rate_limit 5000/s
  root /var/www/static
  access_log_file /var/log/leba/access.log
  deny src 10.0.0.99
  allow src 10.0.0.
  route host app.example.com -> app
  route default -> app
  request_header_set X-Forwarded-Proto https
  response_header_set X-Frame-Options DENY

frontend secure
  bind 443
  mode http
  tls_cert /etc/leba/certs/server.crt
  tls_key /etc/leba/certs/server.key
  protocols http/1.1,h2
  route default -> app

backend app
  balance least_conn
  health_path /health
  health_interval 2s
  server app1 127.0.0.1:8080 weight 100 check
  server app2 127.0.0.1:8081 weight 100 check

frontend stats
  bind 127.0.0.1:9443
  mode stats
  tls_cert /etc/leba/certs/admin.crt
  tls_key /etc/leba/certs/admin.key
  tls_client_ca /etc/leba/certs/client-ca.pem
  admin_user_hash admin $argon2id$... admin
```

## Admin API

The `mode stats` frontend serves:

| Endpoint | Method | Role | Description |
|----------|--------|------|-------------|
| `/` | GET | viewer | Admin dashboard |
| `/stats` | GET | viewer | Runtime JSON |
| `/metrics` | GET | viewer | Prometheus text metrics |
| `/readyz` | GET | public | Readiness probe |
| `/livez` | GET | public | Liveness probe |
| `/admin/servers` | GET | viewer | Server state |
| `/admin/drain/{be}/{srv}` | POST | operator | Drain server |
| `/admin/ready/{be}/{srv}` | POST | operator | Mark ready |
| `/admin/disable/{be}/{srv}` | POST | operator | Force DOWN |
| `/admin/enable/{be}/{srv}` | POST | operator | Force UP |
| `/admin/reload-servers` | POST | operator | Reload servers_file |
| `/admin/vhosts` | GET | viewer | List vhosts |
| `/admin/vhost-create` | POST | admin | Create vhost |
| `/admin/vhost-cert` | POST | admin | Update certificate paths |

## CLI

```text
leba -f <config> [-n MAX]                 Run the proxy
leba doctor <config>                      Validate config
leba explain <config> METHOD PATH [HOST]  Dry-run routing
leba admin servers [ADDR] [USER:PASS]     List servers
leba admin drain BE SRV [ADDR] [AUTH]     Drain a server
leba admin ready BE SRV [ADDR] [AUTH]     Mark ready
leba admin hash-password PASSWORD         Generate hash
leba version                              Print version
```

## Deployment

```bash
leba doctor /etc/leba/leba.conf    # validate
leba -f /etc/leba/leba.conf        # run
```

Typical paths:
```text
/usr/local/bin/leba
/etc/leba/leba.conf
/etc/leba/admin-users.conf
/var/lib/leba/state
/var/log/leba/access.log
```

## Status

Leba is working software with 80+ automated tests. It handles HTTP/1-3, TCP,
UDP/SIP, WebSocket, TLS/mTLS, and has been deployed behind real traffic.

Known limits:
- HTTP/2 covers multiplexed request/response; long-lived streaming and server
  push are not goals.
- HTTP/3 requires a quiche-linked build. Upstream is HTTP/1.1.
- SIP support is signaling-focused; media relay is not implemented.
- No response compression (gzip/brotli) or response caching yet.
- No WAF/request body inspection yet.
