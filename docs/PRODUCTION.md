# Production runbook (SMB / edge)

Use this checklist before calling Leba **production-ready** for a site.

## Baseline (0.14+)

| Item | Check |
|------|--------|
| Version | `leba version` ≥ 0.14.0 |
| Doctor | `leba doctor /etc/leba/leba.conf` → 0 errors |
| Auth | Hashed admin users; no demo passwords |
| Session | `state_key` or `LEBA_SESSION_SECRET` set |
| Admin plane | Not on public VIP; private NIC / firewall |
| TLS | Valid PEMs; ACME lego or external certs on both HA nodes |
| Soak | `make test-soak` green in CI or pre-flight |
| Metrics | Scrape `/metrics` (auth required when configured) |

**Note:** Prefer process supervisor (`systemd` Restart=) and practice reload on the
backup node before VIP move. Re-run `make test-soak` on your Mako toolchain before
cutover.

## Install paths

| Method | Notes |
|--------|--------|
| Linux package layout | `deploy/linux/` systemd unit + env |
| Docker | Image from GHCR; see compose under `deploy/docker/` and `deploy/ha/` |
| Binary | GitHub Releases: checksums + optional SBOM |

## Day-2 operations

```bash
# Config health before apply
curl -su admin:… https://admin:8404/admin/preview-reload -X POST

# Full config apply (rebinds listeners)
curl -su admin:… -X POST https://admin:8404/admin/reload

# Live TLS (TCP/TLS + H3 recreate when cert paths change)
curl -su admin:… -X POST https://admin:8404/admin/tls-reload

# Stick tables
curl -su admin:… https://admin:8404/admin/stick-tables
```

### TLS / H3 cert reload

| Surface | Behavior on `/admin/tls-reload` |
|---------|----------------------------------|
| TCP TLS (HTTP/1–2) | In-place `tls_server_reload` + SNI sync |
| HTTP/3 | **Recreate** H3 listener when cert/key paths change |
| API fields | `tcp_tls_reloaded`, `h3_strategy`, `h3_recreate`, `h3_restart_required` |

`h3_restart_required` is `false` when Leba can recreate H3 listeners in-process;
full process restart is only needed if the linked Mako/quiche build cannot open H3.

## HA

See [HA.md](HA.md) and [deploy/ha/README.md](../deploy/ha/README.md).

- VIP with keepalived + `readyz`
- Optional stick peers (experimental until soak signed off)
- Practice reload on backup before VIP move

## Limits

See [LIMITS.md](LIMITS.md) for body size and streaming honesty.

## Supply chain

Release artifacts (when published via CI):

- Multi-arch container images (`linux/amd64`, `linux/arm64`) on GHCR
- Binary checksums (`SHA256SUMS`)
- CycloneDX SBOM (`sbom.cdx.json`)
- **Sigstore cosign** keyless signature bundle (`leba-linux-amd64.cosign.bundle`)
- Container image signed with the same GitHub OIDC identity (when image push succeeds)

Verify binary:

```bash
gh release download v0.14.0 -p 'leba-linux-amd64' -p 'SHA256SUMS' -p 'leba-linux-amd64.cosign.bundle'
sha256sum -c SHA256SUMS
cosign verify-blob \
  --bundle leba-linux-amd64.cosign.bundle \
  --certificate-identity-regexp 'https://github.com/loreste/leba/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  leba-linux-amd64
```

Verify container (example):

```bash
cosign verify \
  --certificate-identity-regexp 'https://github.com/loreste/leba/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/loreste/leba:0.14.0
```

## Do not claim yet

- “HAProxy Enterprise replacement” without H1–H4 + soak + support story
- Unlimited streaming reverse proxy
- Peers **production** without a signed soak report
