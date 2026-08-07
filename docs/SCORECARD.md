# Performance & HA scorecard

Directional local measurements — re-run on your hardware before capacity claims.

## Environment (scorecard host)

| Field | Value |
|-------|--------|
| Date | 2026-08-07 |
| Host | macOS arm64 (developer laptop) |
| Leba | 0.15.0+ (`--backend c`) — **HTTP fast path** (worker-owned KA loop) |
| nginx | 1.31.3 (Homebrew), `worker_processes 1`, upstream keepalive 64 |
| Origin | nginx static `return 200 'ok\n'` with **upstream keep-alive enabled** |
| Loadgen | `wrk` keep-alive (default) |
| Limits | `ulimit -n 65536` required for high concurrency |

## Headline: keep-alive reverse proxy (wrk)

**Method:** same KA-capable nginx static origin; Leba vs nginx reverse proxy; 3 runs × 4s @ c=40.

| Proxy | RPS (3 runs) | Median | p50 | p99 (typical) |
|-------|--------------|--------|-----|----------------|
| **Leba** (workers 48, fast path) | 75.3k / 75.5k / 72.2k | **~75k** | ~0.45 ms | ~2–4 ms |
| **nginx** (1 worker + upstream KA) | 28.8k / 25.6k / 25.6k | **~26k** | ~1.5 ms | ~2–6 ms |

**Winner: Leba ~2.7–2.9× nginx RPS**, with **~3× lower p50** and competitive p99.

```bash
ulimit -n 65536
make build
# origin MUST allow keep-alive (keepalive_timeout 65s) so pools work
# leba: access_log off, single-server backend, workers >= concurrency
wrk -t4 -c40 -d5s --latency http://127.0.0.1:<leba>/
wrk -t4 -c40 -d5s --latency http://127.0.0.1:<nginx-proxy>/
```

### Fair-bench rules (easy to get wrong)

1. **Origin keep-alive** — `keepalive_timeout 0` on the origin forces reconnect per hop and collapses both proxies (and hides Leba’s pool). Use `keepalive_timeout 65s`.
2. **workers ≥ concurrency** — each keep-alive client holds one crew worker for the KA series. At c=40 use `workers 48`+ or p99 spikes (queued pending).
3. **nginx reverse** — `upstream … keepalive N` + `proxy_set_header Connection ""` for a fair KA comparison.
4. **`ulimit -n 65536`** — soft 256 will fail high-c benches.

### Latency (same method)

| Metric | Leba | nginx |
|--------|------|-------|
| p50 | **~450 µs** | ~1.5 ms |
| p75 (warm) | ~0.8 ms | — |
| p90 (warm) | ~1.5 ms | — |
| p99 (warm) | ~4 ms | ~2–6 ms |

### Memory (honest)

| State | Leba RSS | nginx RSS |
|-------|----------|-----------|
| Idle | ~10–12 MiB | ~4–6 MiB |
| After wrk load | **hundreds of MiB–1+ GiB** | ~4 MiB |

Leba’s C free-analysis allocator **does not return freed pages to the OS**. RSS under load is not comparable to nginx’s slab/pool model. Prefer RPS/latency for the “faster” claim; RSS is a known platform limit until a releasing allocator or pooling strategy lands.

### Concurrency scaling (Leba, workers ≥ c, fast path)

| Mode | c=4 | c=40 | c=100 |
|------|-----|------|-------|
| **Keep-alive RPS** | high | **~72–75k** | scales with workers |

**Config for high concurrency:**

```
defaults
  workers 128   # max 512; size workers >= peak concurrent KA clients
```

Also: `ulimit -n 65536`. Runtime logs `sched_threads` and `done_cap`.

## What unlocked the win

1. **HTTP fast path** — single-server, no ACL/stick/rate/WAF: accept only kicks worker.
2. **Worker-owned keep-alive loop** — next request stays on the worker (no accept requeue).
3. **Scheduler headroom** — `sched_threads = 2× workers + 8` so blocking IO cannot starve accept/done.
4. **Hot-path CPU cuts** — skip full `http_parse` when no body; cheap Content-Type scan; skip pass-header extract when unused; no per-request host clone.
5. **Bench hygiene** — KA origin; workers ≥ c; `ulimit -n 65536`; access_log off.

Fast path eligibility is logged: `event=http_fast_path frontend=...`.

## Python harness (`make bench-nginx`)

Uses Connection: close + thread pool. Useful for regression smoke; **prefer wrk KA** for the north-star claim.

```bash
ulimit -n 65536
./scripts/bench_vs_nginx.sh 5 40   # origin KA + nginx upstream KA
```

## HA peers smoke

| Check | Result |
|-------|--------|
| `make test-ha-peers` | PASS (prior) |
| Multi-hour VIP soak | **Still site-only** for production HA sign-off |

## Related

- `docs/ROADMAP.md` — performance track
- `docs/HA.md` — peers + VIP limits
- `docs/LIMITS.md` — body buffer / KA limits
