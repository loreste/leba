# WAF adapter

Leba includes a lightweight **local** signature engine and an optional **remote**
inspect hook for a sidecar WAF (Coraza, ModSecurity, custom).

## Config

```text
frontend web
  bind 80
  mode http
  waf block
  waf_url http://127.0.0.1:8999/v1/inspect
  waf_fail open
  waf_timeout 50ms
  route default -> app
```

| Key | Values | Meaning |
|-----|--------|---------|
| `waf` | `off` / `detect` / `block` / `on` | `on` = `block`. `detect` logs and allows. |
| `waf_url` | HTTP URL | Optional remote `POST` inspect endpoint |
| `waf_fail` | `open` (default) / `closed` | On remote error / timeout |
| `waf_timeout` | duration | Remote call budget (default 50ms) |

ACME challenge paths are evaluated **before** WAF and are not inspected.

## Local rules (always when mode active)

Blocks common probes in path/query/body (case-insensitive):

- Path traversal (`../`)
- XSS (`<script`, `javascript:`)
- SQLi (`union select`, `or 1=1`, `drop table`)
- LFI (`/etc/passwd`)
- Simple RCE markers (`;wget`, `$(()`)

This is **not** a full CRS ruleset — use a sidecar for production WAF depth.

## Remote inspect contract

```text
POST {waf_url path}
Headers:
  Content-Type: application/octet-stream
  X-Leba-Request-Id: <id>
Body:
  raw HTTP request (truncated at 64KiB)

Response:
  200 + {"action":"allow"}     → allow
  200 + {"action":"block"}     → block 403
  403                          → block 403
  5xx / timeout                → fail open or closed per waf_fail
```

## Detect vs block

| Mode | Local/remote hit |
|------|------------------|
| `detect` | Log `waf_detect`, still proxy |
| `block` / `on` | Respond 403 (or 503 if fail-closed error) |

## Compose sketch

```yaml
services:
  leba:
    # ...
    environment:
      # optional
  waf:
    image: your-coraza-sidecar:latest
    network_mode: service:leba
    # expose inspect on 127.0.0.1:8999
```

## Operator notes

- Prefer `waf detect` in staging, then `waf block`.
- Keep `waf_timeout` low so a slow sidecar cannot stall the edge.
- Pair with rate limits and ACLs for defense in depth.
