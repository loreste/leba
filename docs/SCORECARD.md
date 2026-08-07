# Performance & HA scorecard

Directional local measurements — re-run on your hardware before capacity claims.

## Environment (scorecard host)

| Field | Value |
|-------|--------|
| Date | 2026-08-07 |
| Host | macOS arm64 (developer laptop) |
| Leba | 0.15.0+ (`--backend c`) — **HTTP fast path** (worker-owned KA loop) |
| nginx | 1.31.3 (Homebrew), `worker_processes 1`, upstream keepalive 32 |
| Origin | nginx static `return 200 'ok\n'` (not Python) |
| Loadgen | `wrk` keep-alive (default) |
| Limits | `ulimit -n 65536` required for high concurrency |

## Headline: keep-alive reverse proxy (wrk)

**Method:** same nginx static origin; Leba vs nginx reverse proxy; 3 runs × 4s @ c=40.

| Proxy | RPS (3 runs) | Median | p50 (typical) |
|-------|--------------|--------|----------------|
| **Leba** (workers 32, fast path) | 55.3k / 63.6k / 55.3k | **~55.3k** | ~0.45–0.7 ms |
| **nginx** (1 worker + upstream KA) | 28.5k / 29.0k / 25.5k | **~28.5k** | ~1.5–3.7 ms |

**Winner: Leba ~1.9–2.4× nginx RPS** on this host/method, with lower p50.

```bash
ulimit -n 65536
make build
# origin: nginx static; leba access_log off, single-server backend
wrk -t4 -c40 -d5s --latency http://127.0.0.1:<leba>/
wrk -t4 -c40 -d5s --latency http://127.0.0.1:<nginx-proxy>/
```

### Concurrency scaling (Leba, workers 128, fast path)

| Mode | c=4 | c=40 | c=100 | c=200 | c=500 |
|------|-----|------|-------|-------|-------|
| **Keep-alive RPS** | ~24k | **~72k** | ~59k | ~48–62k | ~47k |
| **Connection: close RPS** | ~14k | ~8–25k | lower* | lower* | — |

\* Connection: close burns ephemeral ports / TIME_WAIT on localhost; prefer KA for concurrency claims.  
At KA c=200 vs nginx (same host): Leba **~62k** vs nginx **~8k** (nginx showed many non-2xx under that burst).

**Config for high concurrency:**

```
defaults
  workers 128   # max 512; default is 64
```

Also: `ulimit -n 65536`. Runtime logs `sched_threads` and `done_cap`.

## What unlocked the win

1. **HTTP fast path** — single-server, no ACL/stick/rate/WAF: accept only kicks worker.
2. **Worker-owned keep-alive loop** — next request stays on the worker (no accept requeue).
3. **Scheduler headroom** — `sched_threads = 2× workers` so blocking IO cannot starve accept/done.
4. **Bench hygiene** — nginx static origin; `ulimit -n 65536`; access_log off.

Fast path eligibility is logged: `event=http_fast_path frontend=...`.

## Python harness (`make bench-nginx`)

Uses Connection: close + thread pool. Useful for regression smoke; **prefer wrk KA** for the north-star claim.

```bash
ulimit -n 65536
./scripts/bench_vs_nginx.sh 5 40   # now uses nginx static origin
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
