# High availability (active/standby)

Leba is a single-process data plane. Multi-node HA uses an external VIP manager
(e.g. keepalived / VRRP) plus shared or synchronized config.

## Recommended pattern: active/standby

```text
        Internet
            │
         VIP :80/:443
            │
    ┌───────┴───────┐
    │               │
 leba-a (MASTER)  leba-b (BACKUP)
    │               │
    └───────┬───────┘
         backends
```

1. Run identical `leba.conf` on both nodes (or pull from config management).
2. Bind public frontends on `0.0.0.0` (or the VIP interface).
3. Keep **stats/admin** on localhost or a private management network only.
4. Use keepalived (or cloud LB) to move the VIP to the healthy node.
5. Health-check the VIP with `GET /readyz` on the public port **or** a dedicated
   health frontend — only advertise the VIP when `readyz` returns 200.

## Stick tables and HA

Local stick tables (`stick on src`) are **per process**. After failover:

- New connections re-learn stick entries.
- Cookie sticky (`sticky cookie NAME`) survives better if the cookie names the
  upstream server and that server is still eligible.

Cluster-wide stick sync (peers) is experimental and not required for standby HA.

## Config reload under HA

- `SIGHUP` / `POST /admin/reload` swaps routes, ACLs, backends, servers (state
  preserved by name), header rules, and app auth.
- **Bind address/port changes still require a rolling restart** (drain VIP →
  restart leba → restore VIP).

## Sample keepalived sketch

See `deploy/ha/keepalived.conf.example`.

```bash
# On both nodes after packaging leba:
systemctl enable --now keepalived
systemctl enable --now leba
```

## Checklist

- [ ] Identical configs (or automated sync)
- [ ] Shared VIP + VRRP priority / preemption policy decided
- [ ] Admin plane not exposed on VIP
- [ ] `readyz` used in track script
- [ ] TLS certs present on both nodes (or shared volume)
- [ ] Drain + `POST /admin/reload` practice run documented
