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

## Multi-IP expand

```text
backend app
  resolve_interval 10s
  server svc my-headless.default.svc.cluster.local:8080 weight 100 check resolve expand
```

When `expand` is set (implies `resolve`):

1. The configured server is a **template** (not pickable).
2. Each A/AAAA answer becomes a synthetic member `svc-<sanitized-ip>` (e.g. `svc-10_0_0_5`).
3. Members inherit weight/port/check/maxconn from the template.
4. Obsolete IPs are drained (if still busy) then removed when idle.

Use with **headless** Kubernetes services or DNS that returns multiple pod IPs.

Without `expand`, only the preferred single IP is used on that server row.

## DNS SRV

```text
backend sip
  resolve_interval 10s
  # port on the template is fallback only; live members use SRV ports
  server sip _sip._udp.sip.voice.google.com:5060 weight 100 check resolve srv
```

When `srv` is set (implies `resolve` + `expand`):

1. The configured server is a **template** (not pickable).
2. Leba queries DNS **SRV** for `resolve_name` (UDP to the first `nameserver` in
   `/etc/resolv.conf`, with optional `dig +short SRV` fallback).
3. Records are ordered by priority (then weight). Each answer’s **target** is
   A/AAAA-resolved; the **port** and **weight** come from the SRV RR (weight
   falls back to the template weight when the RR weight is 0).
4. Synthetic members are named `parent-<target>-<port>` (sanitized).
5. Obsolete targets are drained then removed when idle.

Typical uses: SIP (`_sip._udp…`), Consul/Nomad (`_http._tcp.service.consul`),
Kubernetes headless + SRV.

## Notes

- Failed lookups log `dns_resolve_failed` / `dns_srv` and keep the last known members/IP.
- Pair expand/SRV with health checks so bad targets leave the pool.
- SRV names may only contain letters, digits, dots, underscores, and hyphens.

## Related

- Active health checks run on the resolved IP (`health_path` / `tcp`).
- Stick tables key on client IP, not upstream DNS name.
