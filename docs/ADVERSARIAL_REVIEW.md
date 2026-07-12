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
