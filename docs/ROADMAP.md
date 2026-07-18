# Leba Product Roadmap

| Field | Value |
|-------|-------|
| **Baseline** | Leba **0.12.0** (2026-07) — host parity + DNS-01 |
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
- Sticky cookie + stick-on-src tables (H1–H3); peers **experimental**
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
| Peers / HA are experimental + keepalived DIY | Ent buyers expect turnkey active/standby stick sync |
| No native ACME DNS-01 product path | Many hosts can’t open :80 |
| Streaming / large bodies / RTP | Not edge-LB day-1; don’t block 0.12–0.13 |
| Mako SAFE free still maturing | CI green with workarounds; watch for regressions |

---

## Beat criteria (scorecard)

### vs Nginx Proxy Manager

| ID | Criterion | 0.11 status | Done when |
|----|-----------|-------------|-----------|
| **N1** | Install | Partial | Published image + `docker compose up` docs; default password story |
| **N2** | Proxy host CRUD | **Met** | GUI/API HTTPS host → upstream &lt; 5 min |
| **N3** | Cert renew without restart | **Met** (TCP TLS) | lego + `tls_server_reload`; H3 may restart |
| **N3b** | Multi-host multi-cert | **Met** (SNI) | Per-domain cert in UI without process restart |
| **N4** | Access list + Basic | **Met** (API/UI) | Per-frontend; polish per-host binding later |
| **N5** | Beyond NPM | **Met** | least_conn, drain, doctor, real LB |
| **N6** | Host editor parity | Partial | Force SSL, WS flag, custom path locations, enable/disable host |
| **N7** | Cert lifecycle UX | Partial | Expiry display, renew reminders, DNS-01 option |

### vs HAProxy Enterprise / NGINX Plus

| ID | Criterion | 0.11 status | Done when |
|----|-----------|-------------|-----------|
| **H1** | Hitless full reload | **Met** (with documented limits) | Soak tests; workers change = restart |
| **H2** | Stick tables local | **Met** (~100k design) | Runtime dump/clear API + UI |
| **H3** | HA pair | Partial | Docs + keepalived examples; **peers production** optional |
| **H4** | WAF path | Partial | Adapter shipped; rule packs + UI + metrics productized |
| **H5** | Runtime object API | Partial | Servers/hosts live; full object CRUD later |
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

### 0.13 — Enterprise ops surface

**Goal:** HAProxy Enterprise “day 2” without Fusion.

| Work | Priority | Acceptance |
|------|----------|------------|
| Stick-table runtime API: list / clear / stats | P0 | `GET/DELETE /admin/stick-tables…` + UI |
| Peers production path: auth soak, reconnect, metrics | P0 | Flag still experimental until soak report |
| WAF product surface: mode toggle, blocked counters, sample rules | P1 | UI + Prometheus series |
| Turnkey HA package: dual-node compose + keepalived template | P1 | `deploy/ha/` end-to-end README |
| Runtime object API expansion (backends/servers CRUD) | P1 | Operator role; doctor on write |
| Config “apply” preview (doctor + explain before reload) | P2 | UI button |

**Exit:** H2 runtime, H3 docs+recipe, H4 usable; peers still honest if not soak-complete.

---

### 0.14 — Platform quality

**Goal:** Trust for production edge.

| Work | Priority | Acceptance |
|------|----------|------------|
| Concurrent / soak harness (connection budget, KA, reload under load) | P0 | CI job or `make test-soak` |
| Streaming / large body policy (document limits; optional pump path) | P1 | Doc + doctor warning |
| H3 cert reload strategy (recreate vs restart_required) | P1 | Consistent API field |
| TLS client keep-alive (if needed) | P2 | Benchmark gated |
| OpenTelemetry export (optional) | P2 | Or stick to Prometheus |
| Supply chain: signed releases, SBOM, multi-arch images | P0 | gh releases + checksums |

**Exit:** Can recommend for production SMB/edge with runbook.

---

### 0.15+ — Stretch / non-blocking

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
3. **Honesty** — experimental peers, H3 cert limits, external ACME.
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
3. **0.13** — Stick-table admin API + peers soak + WAF product surface

That sequence maximizes “feels like NPM” first while keeping the HAProxy-class plane credible for the enterprise track.

---

## Changelog of this roadmap

| Date | Change |
|------|--------|
| 2026-07-18 | Initial roadmap from 0.11.0 baseline (NPM control plane shipped) |
| 2026-07-18 | 0.11.1 stabilize complete (ACME UX, expiry, compose demo, doctor hardening) |
| 2026-07-18 | 0.12.0 NPM host parity (locations, WS, enable, host ACL/auth, DNS-01, renew timer) |
