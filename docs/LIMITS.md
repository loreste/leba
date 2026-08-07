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
- Each keep-alive client holds **one crew worker** for the whole series. Size
  `workers` ≥ peak concurrent KA clients or pending queues and p99 will grow.

## Memory safety (bounded process memory)

Leba targets **bounded, ownership-correct** memory under Mako free-analysis:

| Bound | Default policy |
|-------|----------------|
| Pending accept queue | `min(4096, max(128, workers×16))` — hard cap; overflow closes/rejects |
| Done channel | `min(2048, max(128, workers×4))` |
| Upstream pool per server | default 32 idle sockets, hard cap 128 (or `server maxconn`) |
| Request body | `request_body_limit` (default 1MB) — reject 413 over limit |
| Workers | config `workers` (cap 512) — each thread costs stack RSS |
| Sched threads | `2×workers+8` — needed so accept never starves; stacks cost RSS |

**Ownership (no free-alias):** pending requeue always deep-owns buffers
(`pending_client_clone` / `pending_client_with_buffer`); stick maps use
`stick_table_own`; success fast-path completions skip `servers[]` clone when
no maxconn reservation (LiveStats still updated).

**Allocator:** Prefer production builds with **mimalloc** (`make build` auto-detects
Homebrew `libmimalloc.a` — see [MAKO.md](MAKO.md)). System malloc under free-analysis
can leave process RSS high after spikes even when live queues are empty. Live
structures remain hard-capped above either way.

Runtime logs: `event=memory_bounds pending_limit=… done_cap=… workers=…`.

## Operator checklist

- [ ] Set `request_body_limit` per frontend that accepts uploads
- [ ] Size `workers` ≥ peak concurrent keep-alive clients (and not much larger)
- [ ] `leba doctor` clean (no unexpected large-body warnings you did not intend)
- [ ] Soak with `make test-soak` after changing workers / maxconn / body limits
- [ ] Prometheus: `leba_requests_total`, `leba_errors_total`, `leba_active_connections`
- [ ] After load tests, expect RSS plateaus under free-analysis; restart to reclaim OS pages

## Related

- [CONFIG_REFERENCE.md](CONFIG_REFERENCE.md) — knobs
- [SECURITY.md](SECURITY.md) — hardening summary
- [PRODUCTION.md](PRODUCTION.md) — production runbook
