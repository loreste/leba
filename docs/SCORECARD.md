# Performance & HA scorecard

Directional local measurements — **not** a capacity-planning study.
Re-run with `make bench-nginx` / `make test-ha-peers` on your hardware before claims.

## Environment (scorecard host)

| Field | Value |
|-------|--------|
| Date | 2026-08-07 |
| Host | macOS arm64 (developer laptop) |
| Leba | 0.15.0 (`--backend c`) |
| nginx | 1.31.3 (Homebrew), `worker_processes 1` |
| Loadgen | Python `urllib` thread pool, Connection: close, tiny `ok\n` body |
| Origin | ThreadingHTTPServer (same for both) |
| Command | `./scripts/bench_vs_nginx.sh 8 40` × 3 runs |

## Leba vs nginx (median of 3 runs)

| Metric | Leba (workers 8) | nginx (workers 1) | Winner |
|--------|------------------|-------------------|--------|
| **RPS** | ~339 / 500 / 320 → **med ~339** | ~3063 / 2520 / 2954 → **med ~2954** | nginx |
| **p50 latency** | ~81–129 ms | ~1.4–1.7 ms | nginx |
| **p99 latency** | ~226–237 ms | ~96–143 ms | nginx |
| **errors** | 0 | 32–87 | leba (reliability) |
| **RSS after load** | ~6–18 MiB typical* | ~3–4 MiB | nginx |

\* One run reported ~342 MiB RSS after load (possible allocator growth under free-analysis / macOS accounting). Treat as **investigate**, not a published memory win.

### Honesty

On this **Connection: close / small GET** microbench, **nginx is substantially faster** today.
Leba’s north star (beat nginx on RPS/p99/CPU/RSS) is **not met** on this host/method.
Prefer multi-run medians on Linux server iron with `wrk`/`hey` before marketing claims.

```bash
make build
./scripts/bench_vs_nginx.sh 10 50
# optional: BREW nginx path
export PATH="/opt/homebrew/opt/nginx/bin:$PATH"
```

## HA peers smoke

| Check | Result |
|-------|--------|
| `make test-ha-peers` × 3 consecutive | **PASS** all three |
| HELLO auth | OK |
| Proxy both nodes | 200 |
| Stick UPSERT sync A→B | OK |
| Restart B + reconnect + proxy | OK |

**Still required for production HA:** multi-hour VIP/keepalive soak with real traffic and failover drills (`docs/HA.md`). Dual-node smoke is necessary, not sufficient.

```bash
make test-ha-peers
# Site: keepalived + dual-node compose under load for hours
```

## Related

- `docs/ROADMAP.md` — 0.15 performance track
- `docs/HA.md` — peers + VIP limits
- `docs/LIMITS.md` — body buffer / KA limits
