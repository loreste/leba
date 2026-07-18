# Operator Pain Points Leba Targets

Leba is built around a small set of operational problems that show up in load
balancer day-to-day work: unclear routing, risky config changes, limited
runtime control, and missing default observability.

This document describes the workflows Leba tries to make straightforward. It
does not claim complete coverage of every load-balancing feature.

## Target Workflows

| # | Pain | Leba capability |
|---|------|-----------------|
| 1 | Configs are hard to review | Plain `frontend`, `backend`, `route`, `deny`, and `server` lines |
| 2 | Validation output is not actionable | `leba doctor` prints errors, warnings, and suggested fixes |
| 3 | Request routing is hard to reason about | `leba explain METHOD PATH [HOST]` dry-runs routing and ACLs |
| 4 | Runtime control needs custom ceremony | HTTP admin API for drain, ready, disable, and enable |
| 5 | Backend deploys need safer traffic removal | First-class drain state; drained servers do not receive new traffic |
| 6 | Basic dashboard data is missing | Built-in HTML UI plus JSON and metrics endpoints |
| 7 | Probes are often bolted on later | `/readyz` and `/livez` are built in |
| 8 | Logs need manual formatting to be useful | Structured operational logs and access logs by default |
| 9 | Bad configs can reach runtime | Doctor catches port clashes, missing backends, empty backends, and malformed rules |
| 10 | Simple rate limits are too much work | `rate_limit 5000/s` on a frontend |
| 11 | Sticky sessions should be explicit | `sticky cookie NAME` |
| 12 | TLS file mistakes should be caught early | `tls_cert` and `tls_key` paths are validated by doctor |
| 13 | Server state changes should be scriptable | CLI admin commands wrap the HTTP admin API |
| 14 | Request correlation should be available | Access logs include request ids and trace ids; HTTP upstreams receive validated trace context |

## Common Workflows

### Validate A Config

```bash
leba doctor configs/leba.conf
```

Expected output includes a PASS/FAIL result, counts of errors and warnings, and
fix suggestions for each issue.

### Explain A Request

```bash
leba explain configs/leba.conf GET /api/v1 host.example
```

This does not open sockets. It parses the config and reports whether the request
would be denied and which backend would be selected.

### Drain A Server For Maintenance

```bash
curl -u user:pass -X POST http://127.0.0.1:18404/admin/drain/api/api1
# update api1
curl -u user:pass -X POST http://127.0.0.1:18404/admin/ready/api/api1
```

Drain state affects new traffic. Existing connection behavior depends on the
protocol path and the client/upstream behavior.

### Probe Runtime Health

```bash
curl http://127.0.0.1:18404/livez
curl http://127.0.0.1:18404/readyz
```

`livez` reports that the process is responsive. `readyz` reports whether the
configured runtime state has eligible upstream capacity.

## Honest Boundaries

- Leba covers a focused subset of load-balancing behavior.
- Config **table** reload is implemented (`SIGHUP` / `POST /admin/reload`)
  with HTTP/TCP/UDP/H3/**stats** listen rebind.
- Runtime server state is persisted across restart when `state_file` is
  configured; otherwise it is in-memory only.
- TLS, HTTP/2 (multiplexed request/response), and HTTP/3/QUIC ingress (when
  quiche is linked) are implemented; long-lived H2 streaming and RTP/media are not.
- SIP-over-TCP and SIP-over-UDP Call-ID affinity are implemented for signaling;
  RTP/media relay is not implemented.
- Service discovery is supported through `servers_file`; manual protected
  reload (admin API / SIGHUP) and automatic file watching are implemented when
  the runtime watcher backend is available.
- The test suite covers important local paths, but your own traffic patterns
  still need staging and validation.

## Roadmap

- [x] Config reload with HTTP/TCP/UDP/H3/stats/peers rebind + OIDC/peers live apply.
- [x] Protected manual `servers_file` reload.
- [x] Watched `servers_file` reload.
- [x] TLS termination for simple HTTP proxying.
- [x] Basic HTTP/2 ingress over TLS/ALPN.
- [x] HTTP/2 multiplexing, request bodies, and multi-request connections.
- [x] HTTP/3/QUIC ingress proxying (quiche-linked builds).
- [x] SIP-over-TCP Call-ID affinity.
- [x] SIP-over-UDP signaling proxying.
- [ ] RTP/media relay.
- [x] Metrics text endpoint.
- [x] Upstream connection pools wired (`init_server_pools`).
- [x] Cleartext client keep-alive (accept-thread sole closer).
- [x] Retry re-pick across servers (`RetryPlan`).
- [x] Header rules applied on the data plane.
- [x] Live TLS cert reload (`POST /admin/tls-reload`).
- [x] Docker packaging skeleton.
- [x] Concurrent smoke test (`make test-concurrent`).
- [x] ACME HTTP-01 webroot serve.
- [x] Proxy host create/delete API + admin UI.
- [x] App HTTP Basic (`auth_basic` / `auth_user`).
- [ ] Built-in ACME client (external lego/acme.sh + hook for now).
- [x] Config reload + HTTP/TCP/UDP/H3/stats/peers rebind + OIDC/peers apply (`SIGHUP` / `POST /admin/reload`).
- [x] Redirect hosts + dead hosts (`route … redirect|dead`).
- [x] Local stick tables (`stick on src`) including HTTP/3 (XFF or cookie).
- [x] Live OIDC/peers status on `GET /stats` + admin UI (v0.10.0).
- [x] Prometheus gauges for stick entries, OIDC/peers ready, TLS/SNI (v0.10.0).
- [x] HA active/standby docs (`docs/HA.md`).
- [x] WAF adapter (local + remote inspect).
- [x] DNS service discovery (`resolve` / `resolve_interval` / `expand`).
- [ ] More exhaustive concurrent connection tests.
- [x] Upstream forwarding of validated trace headers.

Full competitive plan: [`COMPETITIVE_ARCHITECTURE.md`](COMPETITIVE_ARCHITECTURE.md).
