# Request and body limits

Leba is a reverse proxy / load balancer, not a general streaming media pump.
This document is the operator-facing **body and streaming policy**.

## Defaults

| Knob | Default | Where |
|------|---------|--------|
| `request_body_limit` | `1MB` (defaults) | `defaults` / per-frontend |
| Client timeout | `30s` | `timeout_client` |
| Server timeout | `30s` | `timeout_server` |
| Connect timeout | `5s` | `timeout_connect` |
| Workers | `32` | `workers` (crew pool) |

Frontends inherit defaults; override with:

```text
frontend web
  request_body_limit 4MB
```

## What happens when a body is too large

1. Accept path reads until the configured limit.
2. Oversize raw HTTP requests are rejected with **HTTP 413** before upstream pick.
3. WAF / ACL / rate limit still run only on accepted request sizes.

Doctor emits a **WARN** when any frontend (or defaults) sets a limit above **16MB** —
large bodies increase memory pressure on the accept thread.

## Streaming / large bodies (honest limits)

| Scenario | Status |
|----------|--------|
| Typical API / HTML reverse proxy (&lt; 1–4 MB) | Supported |
| Multi-MB uploads under limit | Buffered; counts against limit |
| Unlimited chunked streaming upload | **Not supported** as a product mode |
| WebSocket upgrade (once accepted) | Pass-through tunnel |
| HTTP/2 multiplex | Supported on TLS ALPN `h2` |
| HTTP/3 / QUIC | Optional (quiche build); same body limit semantics |
| RTP / media relay | Non-goal (see roadmap) |

There is **no** separate “pump path” that streams request bodies without buffering
through the configured limit. Plan edge gateways accordingly:

- Terminate large uploads on an object store / app designed for streaming.
- Raise `request_body_limit` only when necessary; watch `maxconn` and workers.
- Prefer client → CDN / blob storage → app for multi-GB objects.

## Keep-alive

- **Cleartext HTTP client keep-alive** is supported (connection reuse on the frontend).
- **TLS client keep-alive** across many requests is best-effort; measure under your
  TLS stack before relying on it for connection budget planning.
- Upstream connections use short-lived or pooled I/O depending on path; do not assume
  HTTP keep-alive to backends without load testing.

## Operator checklist

- [ ] Set `request_body_limit` per frontend that accepts uploads
- [ ] `leba doctor` clean (no unexpected large-body warnings you did not intend)
- [ ] Soak with `make test-soak` after changing workers / maxconn / body limits
- [ ] Prometheus: `leba_requests_total`, `leba_errors_total`, `leba_active_connections`

## Related

- [CONFIG_REFERENCE.md](CONFIG_REFERENCE.md) — knobs
- [SECURITY.md](SECURITY.md) — hardening summary
- [PRODUCTION.md](PRODUCTION.md) — production runbook
