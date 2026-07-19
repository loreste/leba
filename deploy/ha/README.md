# Turnkey HA package (active/standby)

This directory is a dual-node recipe: **keepalived VIP** + two Leba processes with
optional **stick-table peers**.

## Topology

```text
        Clients
           │
        VIP :80/:443
           │
   ┌───────┴────────┐
   │                │
leba-a (MASTER)  leba-b (BACKUP)
   │                │
   └───────┬────────┘
        backends
```

| Component | Role |
|-----------|------|
| keepalived | VRRP VIP ownership; track script uses `readyz` |
| leba | Data plane on both nodes (identical config) |
| peers (optional) | Stick-table UPSERT sync over private TCP |

## Quick start (VMs / bare metal)

1. Install Leba on both nodes (`deploy/linux/` packaging).
2. Copy the same `leba.conf` (or CM-managed twin) to both nodes.
3. Bind public frontends on `0.0.0.0` (or the VIP interface).
4. Keep **stats/admin** on localhost or a private management network only.
5. Install keepalived:

```bash
sudo cp deploy/ha/keepalived.conf.example /etc/keepalived/keepalived.conf
# edit interface, VIP, priority (150 master / 100 backup), auth_pass
sudo cp deploy/ha/leba-healthcheck.sh /usr/local/bin/leba-healthcheck.sh
sudo chmod +x /usr/local/bin/leba-healthcheck.sh
sudo systemctl enable --now keepalived leba
```

6. (Optional) Enable stick peers — private network only:

```text
# on leba-a
peers
  enabled on
  name leba-a
  cluster edge
  bind 10.0.0.1:1024
  secret $LEBA_PEERS_SECRET
  peer leba-b 10.0.0.2:1024

# on leba-b — swap name/bind/peer
```

Firewall peer ports between nodes; never expose peers publicly.

## Docker Compose sketch

See [docker-compose.ha.yml](docker-compose.ha.yml). Compose cannot do true VRRP
on a laptop without host networking / privileged keepalived; use it to rehearse
**two leba + peers** (and front with an external LB or host VIP in production).

```bash
# From repo root (adjust secrets first)
docker compose -f deploy/ha/docker-compose.ha.yml up -d
```

## Local dual-node smoke (no VIP required)

From a source checkout with a built binary:

```bash
make test-ha-peers
# or: ./scripts/ha_peers_smoke.sh
```

This starts two Leba processes on localhost with stick peers enabled, checks
peers dial/HELLO and `/metrics`, drives proxy traffic with sessions open,
asserts stick UPSERT sync (`upserts_out`/`upserts_in` + live stick tables),
restarts the backup for reconnect, and proxies again. Complete the production
soak checklist below on real VMs (VIP failover, multi-hour soak) before calling
peers **production** in your environment.

## Soak checklist (peers still experimental)

Record results before calling peers **production**:

| Check | How |
|-------|-----|
| HELLO auth reject | Wrong secret → `leba_peers_auth_fail_total` increments; session closes |
| Reconnect | Kill peer process; within ~5s dial restores; `leba_peers_reconnects_total` |
| Stick continuity | Generate stick traffic on A; fail VIP to B; same client IP pins same server |
| Reload under HA | `POST /admin/reload` on backup first; then failover |
| Metrics | `/metrics`: `leba_peers_*`, `leba_stick_entries`, `leba_waf_*` |
| Local smoke | `make test-ha-peers` green on your toolchain |

## Admin / API

| Endpoint | Use |
|----------|-----|
| `GET /admin/stick-tables` | Summary + live counts |
| `GET /admin/stick-tables/entries` | Dump entries |
| `DELETE /admin/stick-tables` | Clear (optional `?backend=`) |
| `GET /admin/preview-reload` | Doctor before apply |
| `POST /admin/reload` | Full config apply + rebind |

## Files

| File | Purpose |
|------|---------|
| `keepalived.conf.example` | VRRP instance + track script |
| `leba-healthcheck.sh` | exit 0 when local Leba is ready |
| `docker-compose.ha.yml` | Dual leba + optional peers lab |
| `leba-a.conf` / `leba-b.conf` | Sample peer-enabled configs for compose |

## Honesty

- VIP HA works **without** peers; stick re-learns after failover if peers off.
- Peers improve stick continuity but remain **experimental** until soak is signed off.
- Prefer practicing reload on the backup node before VIP move.
