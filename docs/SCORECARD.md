# Performance & HA scorecard

Directional local measurements — **not** a capacity-planning study.
Re-run with `make bench-nginx` / `wrk` on your hardware before claims.

## Environment (scorecard host)

| Field | Value |
|-------|--------|
| Date | 2026-08-07 |
| Host | macOS arm64 (developer laptop) |
| Leba | 0.15.0 (`--backend c`) — drain-done, immediate accept, single-slot stats |
| nginx | 1.31.3 (Homebrew), `worker_processes 1` |
| Origin | Python ThreadingHTTPServer, tiny `ok\n` body |
| Bench harness | `./scripts/bench_vs_nginx.sh` (access_log off, retries 0; same as nginx access_log off) |

## wrk — Connection: close (preferred signal)

Clean sequential runs (one proxy at a time; do not overload the shared origin).

| Proxy | Concurrency | RPS | p50 | p99 | Notes |
|-------|-------------|-----|-----|-----|--------|
| **Leba** | c=4 | **~4300** | ~0.7 ms | ~2 ms | access_log off; workers 8 |
| **Leba** | c=40 | **~300** | ~100–130 ms | ~200 ms | Serial accept-thread still limits fan-out |
| **nginx** | c=4 | **~4000** | ~1 ms | ~2 ms | worker_processes 1 |
| **nginx** | c=40 | **~2500–5000** | ~1 ms | varies | Often wins at high fan-out on laptop |

At **low concurrency**, Leba is competitive with nginx on this host.  
At **high concurrency**, nginx still wins — Leba’s accept thread does read/parse/prepare/kick serially.

```bash
make build
# leba: access_log off, retries 0 recommended for pure proxy microbench
wrk -t1 -c4  -d5s --latency -H "Connection: close" http://127.0.0.1:<leba>/
wrk -t2 -c40 -d5s --latency -H "Connection: close" http://127.0.0.1:<leba>/
```

## Python harness (`make bench-nginx`)

| Metric | Leba (workers 8) | nginx (workers 1) | Winner |
|--------|------------------|-------------------|--------|
| **RPS** (typical good run) | ~250–320 | ~2500–4700 | nginx |
| **p50** | ~120–150 ms | ~1.5 ms | nginx |
| **errors** | often 0 | occasional | leba |

Python `urllib` thread pool is a **noisier** loadgen than wrk; use wrk for headline numbers.

## Hot-path work landed (this session)

1. Single-slot `server_conn_delta` / done health (no full `server_array_clone`).
2. `plan.reserved` — skip maxconn reserve when unused; empty Dispatch keeps live tables.
3. Drain up to 256 `done` completions per accept-loop tick (free worker slots faster).
4. Immediate cleartext read+dispatch when request bytes already present and a slot is free.
5. Short poll (1 ms) when pending/inflight — never 0 ms busy-spin (starves crew).
6. Bench harness: `access_log off`, `retries 0` for fairer nginx comparison.

## Honesty

- North star (**beat nginx on RPS/p99**) is **not met at high concurrency** on this laptop microbench.
- Competitive at modest concurrency with access log off is a real improvement; do not market as “faster than nginx” without high-c wrk on Linux iron.
- RSS after heavy load can still grow under free-analysis — investigate separately.

## HA peers smoke

| Check | Result |
|-------|--------|
| `make test-ha-peers` | PASS (prior) |
| Multi-hour VIP soak | **Still site-only** — required for production HA sign-off |

```bash
make test-ha-peers
```

## Related

- `docs/ROADMAP.md` — performance track + hot-path rules  
- `docs/HA.md` — peers + VIP limits  
- `docs/LIMITS.md` — body buffer / KA limits  
