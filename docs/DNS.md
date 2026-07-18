# DNS service discovery

Leba can periodically re-resolve server hostnames and update the active IP used
for upstream connections (and connection pools).

## Config

```text
backend app
  balance least_conn
  resolve_interval 10s
  health_path /health
  health_interval 2s
  server app1 my-service.default.svc.cluster.local:8080 weight 100 check resolve
  server app2 10.0.0.5:8080 weight 100 check
```

| Setting | Meaning |
|---------|---------|
| `resolve` on a server line | Re-resolve this server's hostname |
| `resolve_interval` on backend | How often (default `10s`) |

Literal IP addresses ignore resolve (no-op).

## Behavior

1. On startup and on each interval, Leba calls DNS for `resolve_name`.
2. Prefers the first **IPv4** A record when multiple answers exist; otherwise first answer.
3. When the IP changes, the server's `host` is updated and its connection pool is reset.
4. Runtime state (drain/alive/counters) is preserved across config reload when the
   resolve name is unchanged.

## Notes

- This is **not** full SRV-based multi-target expansion yet (one configured server
  → one active IP). Multiple `server` lines or external automation remain the
  way to list N stable members.
- Pair with Kubernetes headless services or external DNS that returns the VIP /
  preferred address you want to hit.
- Failed lookups log `dns_resolve_failed` and keep the last known IP.

## Related

- Active health checks run on the resolved IP (`health_path` / `tcp`).
- Stick tables key on client IP, not upstream DNS name.
