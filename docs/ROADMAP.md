# Leba Product Roadmap

| Field | Value |
|-------|-------|
| **Baseline** | Leba **0.15.0** (2026-08) — Let's Encrypt + auto SSL + full CI matrix |
| **North star** | HAProxy-class data plane + Nginx Proxy Manager (NPM) day-1 UX, open-core price |
| **Status** | Living roadmap — update each release |
| **Related** | [COMPETITIVE_ARCHITECTURE.md](COMPETITIVE_ARCHITECTURE.md) (design depth), [PAINPOINTS.md](PAINPOINTS.md) (ops workflows) |

---

## Positioning

| Audience | Promise |
|----------|---------|
| **Homelab / SMB (NPM users)** | Add a reverse-proxy host, get TLS, open the UI — under 5 minutes |
| **Edge / platform (HAProxy users)** | Real LB algorithms, drain, stick tables, hitless reload, Prometheus, doctor/explain |
| **Both** | Single binary, plain config as source of truth, no separate DB + nginx process pair |
| **Performance** | **Beat nginx on reverse-proxy efficiency** — higher RPS and lower CPU/RSS for the same edge workload |

### Performance north star (vs nginx)

Leba’s data plane goal is not feature parity with every nginx module. It is to be a **tighter reverse proxy / LB**:

| Metric | Target |
|--------|--------|
| **RPS** (small GET, local origin, Connection: close) | ≥ nginx under same concurrency / same machine |
| **p99 latency** | ≤ nginx on the same bench |
| **CPU** | Lower %CPU at equal RPS (crew workers + upstream pools, no per-request process model) |
| **RSS** | Lower steady-state memory for a fixed connection budget |

**How we measure (local):**

```bash
make build
./scripts/bench_vs_nginx.sh 10 50   # seconds, concurrency
```

**Hot-path rules (do not regress):**

1. **No full stick-table copy** unless stick/cookie sticky is active (`stick_dirty`).
2. **No rate-bucket clone** unless the frontend has `rate_limit` (`rate_dirty`).
3. **No full `server_array_clone` on reserve/release** — `server_conn_delta` / `mark_server_req` deep-own **one slot** via ownership transfer (`servers = f(servers)`). Skip reserve entirely when no frontend/backend `maxconn` and balance ≠ `least_conn` (`plan.reserved=0`).
4. Prefer **`http_forward_fd` + `tcp_pool_*`** over `http_forward_full` (pools already wired via `init_server_pools`).
5. Accept thread does ACL/pick; workers only do upstream I/O (kick-safe).
6. **Empty `Dispatch.servers` / `backends`** when not dirty — accept thread must not wipe live tables (main adopts only non-empty arrays).

**Honest limits today:** buffered request model (see LIMITS.md); TLS client KA best-effort; free-analysis still forces owned returns on some TLS paths — cleartext hot path is the primary optimization track. On laptop Connection:close microbench, **serial accept-thread work** (read/parse/prepare/kick) still dominates (~few ms/req); beating nginx needs further accept-path architecture work, not only clone skips.

**Marketing gate (do not claim until checked):**

| Claim | Ready when |
|-------|------------|
| “NPM replacement” | N1–N5 (below) all green, including multi-host cert UX that operators trust |
| “HAProxy Enterprise alternative” | H1–H4 green; peers/HA called **production** not experimental |
| “HAProxy Enterprise replacement” | **Never** without full parity review — avoid this phrase until H1–H4 + soak + support story |

---

## Where we are (0.11.0)

### Shipped — data plane (HAProxy-class core)

- HTTP/1.1, H2 (ALPN), H3/QUIC (quiche builds), WebSocket, TCP, SIP/UDP Call-ID affinity
- Balance: RR, least_conn, ip_hash, weighted, random, consistent-hash, SIP Call-ID
- Sticky cookie + stick-on-src tables (H1–H3); peers dual-node smoke green (VIP soak still for prod sign-off)
- Drain / ready / disable / enable; active + passive health
- TLS termination, mTLS, multi-cert SNI, live `tls-reload`
- ACL, rate limit, header rules (wired), WAF adapter, app HTTP Basic
- Upstream pools + cleartext client keep-alive + retry re-pick
- Full config reload + listener rebind (HTTP/TCP/UDP/H3/stats/peers)
- DNS resolve / expand / SRV; Prometheus + `/stats` + doctor + explain

### Shipped — control plane (NPM path)

- Admin UI: Proxy Hosts (upsert, Force SSL, SNI certs), Certificates, Access Lists
- Session auth + RBAC (viewer / operator / admin) + OIDC SSO
- ACME via **external lego** (issue/renew API + UI + Docker bundle)
- Managed includes: `leba.vhosts.conf`, `leba.access.conf`
- Docker compose + lego profile; systemd packaging under `deploy/`

### Honest gaps

| Gap | Why it matters |
|-----|----------------|
| ACME is lego-orchestrated, not pure in-process JOSE | NPM feels “one click”; we need lego + port 80 + email |
| Access lists UI is ACL + Basic only | NPM has richer per-host toggles (WS, block exploits, custom locations) |
| Peers VIP multi-hour soak not productized | Dual-node smoke + ownership fixes green; keepalived DIY + site soak remain |
| No native ACME DNS-01 product path | Many hosts can’t open :80 (lego DNS-01 API exists; product packaging later) |
| Streaming / large bodies / RTP | Not edge-LB day-1; see LIMITS.md |
| Mako SAFE free still maturing | Ownership fixes + CI installs Mako; watch free-analysis regressions |

---

## Beat criteria (scorecard)

### vs Nginx Proxy Manager

| ID | Criterion | 0.11 status | Done when |
|----|-----------|-------------|-----------|
| **N1** | Install | **Met** (0.14) | Published binary + GHCR image + `docker compose` / `LEBA_IMAGE` docs |
| **N2** | Proxy host CRUD | **Met** | GUI/API HTTPS host → upstream &lt; 5 min |
| **N3** | Cert renew without restart | **Met** (TCP TLS) | lego + `tls_server_reload`; H3 may recreate |
| **N3b** | Multi-host multi-cert | **Met** (SNI) | Per-domain cert in UI without process restart |
| **N4** | Access list + Basic | **Met** (API/UI) | Per-frontend; polish per-host binding later |
| **N5** | Beyond NPM | **Met** | least_conn, drain, doctor, real LB |
| **N6** | Host editor parity | **Met** (0.15) | Per-host Force SSL, Request SSL, WS, locations, enable/disable |
| **N7** | Cert lifecycle UX | **Met** (0.15) | Official LE prod/staging, issue/renew UI, timer, DNS-01 |

### vs HAProxy Enterprise / NGINX Plus

| ID | Criterion | 0.14 status | Done when |
|----|-----------|-------------|-----------|
| **H1** | Hitless full reload | **Met** (with documented limits) | Soak tests; workers change = restart |
| **H2** | Stick tables local | **Met** (~100k design) | Runtime dump/clear API + UI |
| **H3** | HA pair | Partial | Docs + keepalived + dual-node peers smoke; **site VIP multi-hour soak** still required for “peers production” |
| **H4** | WAF path | **Met** (adapter + UI + metrics; rule packs open-core later) | Adapter shipped; mode toggle + blocked counters productized |
| **H5** | Runtime object API | Partial | Servers/hosts/stick live; full object CRUD later |
| **H6** | SSO | Partial | OIDC admin yes; SAML later if demanded |
| **P1** | Observability | Strong | Trace + Prometheus + analytics; dashboards templates |

---

## Release roadmap

### 0.11.x — Stabilize NPM control plane ✅ *(0.11.1)*

Shipped: ACME preflight UX, cert expiry, compose demo, doctor hardening, tests.

---

### 0.12 — NPM host parity ✅ *(0.12.0)*

**Goal:** Operators stop missing NPM host toggles.

| Work | Priority | Status |
|------|----------|--------|
| Per-host enable/disable | P0 | ✅ `enable=0` → dead 503; UI toggle |
| WebSocket on/off per host | P1 | ✅ `websocket off` route flag + 403 |
| Custom locations | P0 | ✅ `path_prefix` + `host_match` + API/UI |
| Redirect / dead from UI | P1 | ✅ `action=redirect\|dead` on proxy-host |
| Host-scoped IP access list | P1 | ✅ ACL kind `host_src` domain\|ip |
| Host-scoped HTTP Basic | P1 | ✅ `auth_user … host DOMAIN` + API |
| DNS-01 via lego | P1 | ✅ `challenge=dns&dns_provider=` |
| Bulk cert renew schedule | P2 | ✅ `deploy/linux/leba-acme-renew.{service,timer}` |

**Exit:** N6 green; N7 mostly green.

---

### 0.13 — Enterprise ops surface ✅ *(0.13.0)*

**Goal:** HAProxy Enterprise “day 2” without Fusion.

| Work | Priority | Status |
|------|----------|--------|
| Stick-table runtime API: list / clear / stats | P0 | ✅ `GET/DELETE /admin/stick-tables…` + UI |
| Peers production path: auth soak, reconnect, metrics | P0 | ✅ metrics + reconnect + free-alias fix; `make test-ha-peers` (still multi-hour soak for prod sign-off) |
| WAF product surface: mode toggle, blocked counters, sample rules | P1 | ✅ UI + Prometheus + `/admin/waf-*` |
| Turnkey HA package: dual-node compose + keepalived template | P1 | ✅ `deploy/ha/` README + compose |
| Runtime object API expansion (backends/servers CRUD) | P1 | ✅ `POST /admin/server` + delete |
| Config “apply” preview (doctor + explain before reload) | P2 | ✅ `/admin/preview-reload` + doctor/explain UI |

**Exit:** H2 runtime, H3 docs+recipe, H4 usable; peers still honest if not soak-complete.

---

### 0.14 — Platform quality ✅ *(0.14.0)*

**Goal:** Trust for production edge.

| Work | Priority | Status |
|------|----------|--------|
| Concurrent / soak harness (connection budget, KA, reload under load) | P0 | ✅ `make test-soak` / `scripts/soak.sh` + CI |
| Streaming / large body policy (document limits; optional pump path) | P1 | ✅ `docs/LIMITS.md` + doctor WARN &gt;16MB |
| H3 cert reload strategy (recreate vs restart_required) | P1 | ✅ `h3_strategy=recreate` + in-process rebind |
| TLS client keep-alive (if needed) | P2 | Deferred — cleartext KA only; see LIMITS.md |
| OpenTelemetry export (optional) | P2 | Deferred — Prometheus remains default |
| Supply chain: signed releases, SBOM, multi-arch images | P0 | ✅ release workflow + SHA256SUMS + SBOM + multi-arch |

**Exit:** Can recommend for production SMB/edge with runbook (`docs/PRODUCTION.md`).

---

### 0.15 — NPM LE day-1 + perf harden ✅ *(0.15.0)*

**Goal:** Add a host, get a Let's Encrypt cert, force SSL — under five minutes; keep the hot path lean.

| Work | Priority | Status |
|------|----------|--------|
| Dirty-flag adopt / lean pick / retry / headers | P0 | ✅ |
| TLS/H2/H3 free-analysis dirty adopt | P0 | ✅ |
| Full CI matrix (units + concurrent + adversarial + soak + peers) | P0 | ✅ |
| Per-host `force_ssl` + Request SSL on proxy-host | P0 | ✅ |
| Let's Encrypt directories (prod/staging) via lego `--server` | P0 | ✅ |
| Certificates admin tab + Linux ACME template + renew timer | P0 | ✅ |
| `bench_vs_nginx.sh` harness | P0 | ✅ |
| Publish CPU/RSS scorecard numbers per release | P1 | ✅ `docs/SCORECARD.md` (2026-08-07 laptop: nginx wins RPS; peers 3× PASS) |

**Exit:** N6/N7 green with official LE; production template has ACME defaults.

**Perf honesty (0.15.0 scorecard):** local Connection:close microbench still favors **nginx** on RPS/p50; Leba reliability (0 fails) and peers smoke are green. See `docs/SCORECARD.md`.

### 0.16+ — Stretch / non-blocking

| Item | Notes |
|------|--------|
| Native ACME JOSE (if Mako gains sign primitives) | Optional; lego remains default |
| SAML admin SSO | Only if customers demand |
| RTP / media relay | Explicit non-goal until SIP product push |
| Paid open-core modules (WAF packs, Fusion-like CP) | Product decision |
| Graphite / multi-cluster control plane | After single-node product is loved |

---

## Priority principles

1. **Day-1 UX before more algorithms** — NPM users leave on certs + hosts, not least_conn.
2. **Config remains source of truth** — managed includes, no hidden DB.
3. **Honesty** — peers need site VIP soak; H3 cert limits; external ACME.
4. **Hitless where it counts** — TLS reload + full table reload; document restarts.
5. **Measure** — each release: N/H scorecard + soak notes.

---

## Suggested sequencing (DAG)

```text
0.11.x stabilize ──► 0.12 NPM host parity ──► “NPM ready” messaging
                           │
                           ▼
                    0.13 Enterprise ops ──► “edge LB for production” messaging
                           │
                           ▼
                    0.14 Platform quality ──► LTS / support discussion
                           │
                           ▼
                    0.15+ stretch (SAML, native ACME, open-core)
```

Parallel tracks allowed:

- **Track A (UX):** 0.12 host editor + DNS-01
- **Track B (Ops):** stick-table API + peers soak + HA recipe
- **Track C (Trust):** soak tests + release signing

A and B can run in parallel after 0.11.x; C continuous.

---

## Release checklist (every version)

- [ ] Version aligned: `mako.toml`, `main.mko`, metrics `leba_info`, README
- [ ] `make test` green; note soak if any
- [ ] Scorecard N/H updated in this file
- [ ] `docs/PAINPOINTS.md` roadmap bullets
- [ ] ACME / HA / ADMIN_API docs match API
- [ ] No claim of “Enterprise replacement” unless H1–H4 green

---

## Near-term recommendation (next 4–6 weeks)

1. ~~**0.11.1** stabilize~~ ✅
2. ~~**0.12** host parity~~ ✅
3. ~~**0.13** enterprise ops~~ ✅
4. ~~**0.14** platform quality~~ ✅
5. ~~**0.15** Let's Encrypt + auto SSL + full CI~~ ✅
6. **0.16+** — stretch (SAML, native ACME JOSE, open-core) only when demanded

That sequence maximizes “feels like NPM” first while keeping the HAProxy-class plane credible for the enterprise track.

---

## Changelog of this roadmap

| Date | Change |
|------|--------|
| 2026-07-18 | Initial roadmap from 0.11.0 baseline (NPM control plane shipped) |
| 2026-07-18 | 0.11.1 stabilize complete (ACME UX, expiry, compose demo, doctor hardening) |
| 2026-07-18 | 0.12.0 NPM host parity (locations, WS, enable, host ACL/auth, DNS-01, renew timer) |
| 2026-07-18 | 0.13.0 enterprise ops (stick API, peers metrics, WAF surface, HA package, preview) |
| 2026-07-18 | 0.14.0 platform quality (soak, LIMITS/PRODUCTION, H3 recreate, release/SBOM CI) |
| 2026-07-19 | v0.14.0 GitHub Release published (binary, SHA256SUMS, SBOM, multi-arch GHCR image) |
| 2026-07-19 | Cosign keyless signing on release + dual-node `make test-ha-peers` smoke |
| 2026-07-19 | Peers free-alias fix: proxy + stick UPSERT + reconnect stable (`stick_table_own`) |
| 2026-07-19 | Stick table residual ownership (`stick_table_clear` + all accept-thread adoptions) |
| 2026-07-19 | CI installs Mako (clone+path), runs unit/build/soak/`test-ha-peers` honestly |
| 2026-08-06 | Production hardening: LF/CRLF HTTP framing, browser header path tests, expanded concurrent smoke; scorecard N6/N7/H4 marked Met |
| 2026-08-06 | Free-alias production fixes: config frontend clone, peers stick own, dispatch deep-own of rate/server arrays (proxy no longer aborts under free-analysis) |
| 2026-08-06 | Perf track: dirty-flag rate/stick adopt, skip stick map own when unused, single server clone; `bench_vs_nginx.sh`; roadmap 0.15 performance north star |
| 2026-08-06 | Faster pick/retry/header path: single-pass pick_server, retries-0 plan, one header build per request, empty-rule skip |
| 2026-08-06 | Lean TLS/H2/H3: dirty servers/backends/rate/stick; adversarial no empty-wipe; H2 session-own once |
| 2026-08-06 | Stick free-alias fix (no intermediate map assign); concurrent smoke ephemeral ports; pending buffer rebuild |
| 2026-08-07 | Defer backend/rate clone + header render until after ACME/static/CORS short-circuit |
| 2026-08-07 | **v0.14.1** production harden + performance: free-alias fixes, dirty adopt, lean hot path, bench harness |
| 2026-08-07 | Full test matrix in CI; TCP/TLS free-alias fixes under adversarial |
| 2026-08-07 | Auto SSL on proxy-host; per-host Force SSL; Certificates UI |
| 2026-08-07 | **v0.15.0** first-class Let's Encrypt (prod/staging directories), Linux ACME defaults, doctor lego check |
| 2026-08-07 | Scorecard: bench harness ephemeral ports + RSS; published laptop medians vs nginx; HA peers ×3 PASS |
| 2026-08-07 | Hot path: single-slot `server_conn_delta`/`mark_server_req`; `plan.reserved` skip; empty Dispatch keep live tables; scorecard rebench (nginx still wins RPS) |
