# Leba Product Roadmap

| Field | Value |
|-------|-------|
| **Baseline** | Leba 0.15.x |
| **North star** | HAProxy-class data plane + Nginx Proxy Manager day-1 UX, delivered as one Mako-native binary |
| **Current focus** | Native ACME, production test gates, and honest performance scorecards |
| **Related** | [ACME.md](ACME.md), [PRODUCTION.md](PRODUCTION.md), [SCORECARD.md](SCORECARD.md), [LIMITS.md](LIMITS.md) |

## Positioning

| Audience | Promise |
|----------|---------|
| Homelab / SMB | Add a reverse-proxy host, request TLS, and force HTTPS from one UI/API flow |
| Edge / platform | Load balancing, drain, stick tables, hitless reload paths, Prometheus, doctor, and explain |
| Both | Single binary, plain config as source of truth, no hidden DB, no nginx sidecar |
| Performance | Beat nginx/HAProxy on targeted reverse-proxy efficiency before making broad replacement claims |

## Shipped

### Data Plane

- HTTP/1.1, HTTP/2, HTTP/3 when built with quiche, WebSocket, TCP, UDP/SIP.
- Balance modes: round-robin, least connections, IP hash, weighted, random, consistent hash, SIP Call-ID.
- TLS termination, mTLS, multi-cert SNI, and live TLS reload.
- ACLs, rate limits, header rules, WAF adapter, app HTTP Basic.
- Health checks, drain/ready/disable/enable, upstream pools, retry repick.
- DNS resolve/expand/SRV, Prometheus, `/stats`, `doctor`, and `explain`.

### Control Plane

- Admin UI for proxy hosts, certificates, access lists, server state, security, analytics, and config.
- Session auth, RBAC roles, and OIDC admin SSO.
- Native ACME HTTP-01 issue/renew in Mako with Let's Encrypt production/staging and custom ACME directory support.
- Managed include files for proxy hosts and access rules.
- Docker and systemd packaging under `deploy/`.

## Honest Gaps

| Gap | Why it matters |
|-----|----------------|
| Native DNS-01 provider adapters | Wildcards and closed-port-80 environments still need an explicit legacy helper or future provider adapters |
| Peers production sign-off | Dual-node smoke is green, but production HA still needs site-specific VIP soak |
| Streaming / large bodies / RTP | Not part of the day-1 edge-LB target; see [LIMITS.md](LIMITS.md) |
| Broad replacement claims | Do not claim full HAProxy Enterprise or NGINX Plus replacement until feature and soak gates are explicit |

## Beat Criteria

### Nginx Proxy Manager

| ID | Criterion | Status | Done when |
|----|-----------|--------|-----------|
| N1 | Install | Met | Binary, Docker, and Linux package layout documented |
| N2 | Proxy host CRUD | Met | Host to upstream from UI/API |
| N3 | Certificate lifecycle | Met for HTTP-01 | Native ACME issue/renew, SNI attach, live reload |
| N4 | Multi-host certificates | Met | Per-domain SNI certificate inventory |
| N5 | Access lists + Basic auth | Met | UI/API for ACL and app auth |
| N6 | Host editor parity | Met | Force SSL, Request SSL, WebSocket toggle, locations, enable/disable |
| N7 | Beyond NPM | Met | Real LB algorithms, drain, doctor, explain, metrics |

### HAProxy / NGINX Plus

| ID | Criterion | Status | Done when |
|----|-----------|--------|-----------|
| H1 | Reload paths | Met with documented limits | Full reload, TLS reload, and listener rebind tested |
| H2 | Runtime operations | Strong | Server state, hosts, certs, stick tables, WAF controls |
| H3 | HA pair | Partial | Keepalived docs + peers smoke; site VIP soak still required |
| H4 | WAF surface | Met | Local adapter, remote adapter, counters, UI/API controls |
| H5 | Observability | Strong | Prometheus, JSON stats, access logs, analytics |
| H6 | Performance claims | Ongoing | Release scorecard publishes RPS, p99, CPU, and RSS against nginx/HAProxy |

## Release Tracks

### 0.15.x Native ACME And Test Hardening

| Work | Priority | Status |
|------|----------|--------|
| Native ACME HTTP-01 issue/renew | P0 | Done |
| P-256 account key, ES256 JWS, CSR finalize | P0 | Done |
| Let’s Encrypt production/staging directory support | P0 | Done |
| Admin certificates tab and proxy-host Request SSL flow | P0 | Done |
| Linux/Docker native ACME defaults | P0 | Done |
| Full local gate with release-built adversarial smoke | P0 | Done |
| Native DNS-01 provider adapters | P1 | Future |

### 0.16+ Candidate Work

| Work | Notes |
|------|-------|
| Native DNS-01 adapters | Cloudflare first, then Route53/DigitalOcean if needed |
| Longer HA soak reports | Publish repeatable VIP failover evidence |
| Performance scorecard refresh | Compare nginx and HAProxy on the same release hardware |
| SAML admin SSO | Only if customer demand appears |
| Paid/open-core modules | WAF packs or multi-cluster control plane are product decisions |

## Validation Policy

- `make test-full` is the local pre-push gate: unit tests, assets, concurrent smoke, and adversarial smoke.
- `make doctor` must report 0 errors for the sample config.
- Native ACME unit coverage must include account helper, JWS construction, and CSR DER/base64url encoding.
- Live Let's Encrypt staging issuance requires a public DNS name and public port 80 reachability; local tests validate the ACME plumbing but cannot replace CA validation.

## Claim Policy

- It is fair to claim “native ACME HTTP-01, no nginx/certbot/lego process required.”
- It is fair to claim “Nginx Proxy Manager-style host and certificate workflow.”
- Do not claim “full NGINX Plus replacement” or “full HAProxy Enterprise replacement” without a parity matrix, scorecard, HA soak evidence, and support story.
