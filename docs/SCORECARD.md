# Performance & HA scorecard

Directional local measurements — **not** a capacity-planning study.
Re-run with `make bench-nginx` / `make test-ha-peers` on your hardware before claims.

## Environment (scorecard host)

| Field | Value |
|-------|--------|
| Date | 2026-08-07 |
| Host | macOS arm64 (developer laptop) |
| Leba | 0.15.0 (`--backend c`), post single-slot conn delta + reserve-skip |
| nginx | 1.31.3 (Homebrew), `worker_processes 1` |
| Loadgen | Python `urllib` thread pool, Connection: close, tiny `ok\n` body |
| Origin | ThreadingHTTPServer (same for both) |
| Command | `./scripts/bench_vs_nginx.sh 6 40` × 3 runs |

## Leba vs nginx (median of 3 runs)

| Metric | Leba (workers 8) | nginx (workers 1) | Winner |
|--------|------------------|-------------------|--------|
| **RPS** | ~325 / 308 / 136 → **med ~308** | ~4773 / 2480 / 2523 → **med ~2523** | nginx |
| **p50 latency** | ~123–154 ms | ~1.5–1.6 ms | nginx |
| **p99 latency** | ~224–244 ms (outlier 3s under load) | ~91–124 ms | nginx |
| **errors** | 0 / 0 / 40 | 1 / 58 / 41 | mixed |
| **RSS after load** | ~0.5–1.0 GiB (growth)* | ~4 MiB | nginx |

\* RSS after load remains a **free-analysis / allocator growth** investigation item — not a memory win. Steady idle RSS is much lower (~9 MiB before load in these runs).

### wrk cross-check (same host, 6s, c=40)

| Proxy | Mode | RPS | p50 | p99 |
|-------|------|-----|-----|-----|
| Leba (no maxconn, retries 0) | Connection: close | ~287 | ~134 ms | ~248 ms |
| Leba | keep-alive | ~173 | ~219 ms | ~371 ms |
| nginx | Connection: close | ~3314 | ~1.1 ms | ~512 ms |

KA under wrk is **not** better today — treat cleartext KA as best-effort, not a published win.

### Honesty

On this **Connection: close / small GET** microbench, **nginx is still substantially faster**.

Hot-path clone work is improved (single-slot `server_conn_delta` / `mark_server_req`; skip reserve when no maxconn / not least_conn; empty Dispatch arrays keep live tables). With **one backend server**, full-array vs single-slot cost is nearly identical — the remaining gap is **serial accept-thread** read/parse/prepare/kick (~few ms/req), not O(n) server clones.

Prefer multi-run medians on Linux server iron with `wrk`/`hey` before marketing claims.

```bash
make build
./scripts/bench_vs_nginx.sh 10 50
# optional: BREW nginx path
export PATH="/opt/homebrew/opt/nginx/bin:$PATH"
# wrk cross-check
wrk -t2 -c40 -d10s --latency -H "Connection: close" http://127.0.0.1:<port>/
```

## HA peers smoke

| Check | Result |
|-------|--------|
| `make test-ha-peers` × 3 consecutive | **PASS** all three (prior session) |
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

- `docs/ROADMAP.md` — 0.15 performance track + hot-path rules
- `docs/HA.md` — peers + VIP limits
- `docs/LIMITS.md` — body buffer / KA limits
