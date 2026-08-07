# Adversarial review notes (Leba)

Findings from hostile self-review + automated tests.

## Bugs found and fixed

| Severity | Issue | Fix |
|----------|--------|-----|
| **High** | Sticky sessions could pin traffic to **DRAIN** servers (`alive==1` only) | Sticky requires `server_eligible` (UP ∧ ¬DRAIN) |
| **High** | `path_prefix ""` matches every path (`str_has_prefix` true for empty) | Empty prefix/suffix/contains never match; doctor errors on empty route/ACL |
| **High** | All members **DRAIN** still received traffic via last-resort pool | Drain fails closed → `502 no server` |
| **High** | Rate limiting configured but **not applied** on proxy path | Restored token-bucket gate → `429` |
| **Medium** | `select_backend` duplicated match rules (diverged from ACL) | Single `match_kind` implementation |
| **Medium** | Rate limiter refilled only in coarse 1s bursts | Continuous elapsed-ms token accrual with fractional carry |
| **Medium** | Stats/admin `auth user:pass` parsed but not enforced | HTTP Basic auth plus viewer/operator/admin RBAC on dashboard, `/stats`, `/metrics`, and `/admin/*` |
| **Medium** | `http_respond_ct` arg order swapped (admin HTML body was content-type) | Fixed CT-then-body |
| **Low** | Multi-file `mako test .` C redefinition / codegen panics | Split suites; avoid `[]int` multi-return patterns |
| **High** | TLS/stack LF-only request framing dropped all headers after the request line (`raw_header` split only on CRLF) | Split on LF; trim CR; headers-end accepts `\r\n\r\n` or `\n\n` |
| **High** | Credentialed browser uploads lost Origin/CSRF/Content-Type on TLS path | Explicit request header extras + `LEBA_CORS_ORIGIN` credential-safe CORS |
| **Medium** | Concurrent smoke only exercised serial GET | Multi-wave concurrent GET/KA/POST/OPTIONS + header forward assert |
| **High** | `parse_config_text` free-aliased `[]Frontend` store-back (SIGABRT under free-analysis) | `frontend_array_clone` / `frontend_clone` on every frontend write path |
| **High** | Peers stick map free-aliased across `peers_apply_line` / `peers_feed` | `stick_table_own` after each apply and before multi-return set/delete |
| **Medium** | Stats rebind multi-return free-aliased `Frontend.tls_sni` | `frontend_clone` in `rebind_stats_listener` returns |
| **Critical** | Proxy accept path double-freed `[]RateBucket` (`prepare_raw_dispatch` aliased param → ASAN abort on first request) | Deep-own servers/backends/buckets/stick at entry of all dispatch paths; `client_rate_take_ip` clones |
| **Critical** | Lean TLS early reject could wipe live `servers` if main always assigned `servers = tr.servers` | `servers_dirty` / `backends_dirty` / rate/stick dirty; main+H2 adopt only when dirty |
| **High** | H2 multi-stream free-alias if `let mut srv = servers` then return without clone | One session-owned clone per H2 connection; per-stream dirty merge only |
| **Critical** | `let mut table = stick_table` then `table = stick_table_own(...)` free-aliased stick maps (first stick-on-src request aborted) | Always `let mut table = stick_table_own(stick_table)` with no intermediate alias |
| **High** | PendingClient.buffer free-aliased with `raw_pending` (ASAN heap-use-after-free) | Rebuild buffer with `str_builder` before requeue |
| **High** | Map “preowned transfer” into `stick_table_set` double-freed under free-analysis | Keep deep-own on every set; do not transfer maps |
| **Medium** | Pending requeue free-aliased `PendingClient` (slots-full / not-ready / partial body) | `pending_client_clone` / `pending_client_with_buffer` on every requeue |
| **Low** | `extract_pass_headers` re-split every value on `:` | Colon index + substr (hot upstream path) |
| **Medium** | ACME/static/CORS still paid backend_array_clone cost before short-circuit | Clone + header render only after short-circuit returns |

## Test inventory

```bash
make test                 # unit suites
make test-adversarial     # units + e2e hostile smoke
make test-haproxy-compare # behavior compare + serial req/s sample
```

| File | Focus |
|------|--------|
| `leba_core1_test.mko` | util, config, ACL, LB, sticky, drain |
| `leba_core2_test.mko` | rate, doctor, explain, admin API, health |
| `leba_web_test.mko` | stats JSON shape, webadmin HTML completeness |
| `scripts/adversarial_smoke.sh` | doctor pass/fail, explain DENY, drain, 502 when pool empty, admin HTML size |
| `scripts/haproxy_compare.sh` | local behavior comparison plus a modest serial HTTP req/s sample |

## Remaining risks (honest)

- The compiler has struggled with some large/complex functions in this codebase;
  keep modules thin
- Integration tests for concurrent accept / keep-alive not exhaustive
- Web admin client-side explain can drift if server ACL semantics change (shared rules reduce risk)
- The comparison req/s sample is serial and local; it is for regression signal,
  not capacity planning

## Always run before release

```bash
make test-adversarial
make test-haproxy-compare
```
