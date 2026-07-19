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

Local stick tables (`stick on src`) are **per process** by default. After
failover without peers:

- New connections re-learn stick entries.
- Cookie sticky (`sticky cookie NAME`) survives better if the cookie names the
  upstream server and that server is still eligible.

### Stick-table peers (HA sync)

Leba can sync stick-table upserts between nodes over a private TCP channel:

```text
peers
  enabled on
  name leba-a
  cluster edge
  bind 0.0.0.0:1024
  secret $LEBA_PEERS_SECRET
  peer leba-b 10.0.0.2:1024
```

| Setting | Meaning |
|---------|---------|
| `name` | Local node id (must be unique) |
| `cluster` | Shared cluster label (HELLO must match) |
| `bind` | Listen address for peer connections |
| `secret` | Shared HMAC secret (`hmac_sha256(secret, name\|cluster)`) |
| `peer` | Remote `NAME HOST:PORT` (repeatable) |

Behavior:

1. On connect: `HELLO` auth + `SYNC` full dump of non-expired entries.
2. On local stick write: broadcast `UPSERT key server expires_ms` to ready peers.
3. Incoming upserts prefer the entry with the later absolute expiry.
4. Serviced only on the accept thread (no map races with workers).
5. Stick maps and peer protocol fields are **deep-owned** (`stick_table_own` /
   `own_string`) so proxy traffic and reconnect stay stable under Mako ownership.

**Status:** dual-node automation green (`make test-ha-peers` — HELLO, proxy with
peers open, UPSERT sync, reconnect). Still call **production** only after your
VIP multi-hour soak (see [deploy/ha/README.md](../deploy/ha/README.md)). Prefer
private networks / firewall peer ports. Peers are not required for active/standby
VIP HA; they improve stick continuity after failover for `stick on src` on
**cleartext, TLS HTTP/1–2, and HTTP/3** frontends. Stick keys use the client
**IP** (not source port); with `xff on`, the first `X-Forwarded-For` hop is used
when present. On HTTP/3, peer address is not available from Mako yet, so stick
falls back to XFF when present (and still shares the same in-process table +
peer UPSERT path as TCP/TLS).

**Local smoke:**

```bash
make test-ha-peers
```

### Peers metrics (0.13+)

| Series | Meaning |
|--------|---------|
| `leba_peers_enabled` / `leba_peers_ready` | Config |
| `leba_peers_remotes` | Configured remotes |
| `leba_peers_hello_ok_total` | Successful HELLO |
| `leba_peers_auth_fail_total` | Auth / protocol rejects |
| `leba_peers_upserts_in_total` / `leba_peers_upserts_out_total` | Stick sync |
| `leba_peers_reconnects_total` | Outbound dials that connected |

Reconnect: accept thread re-dials missing remotes about every 5s via
`peers_dial_missing`. Identity/secret changes on reload reset sessions.

### Stick runtime API

| Method | Path |
|--------|------|
| GET | `/admin/stick-tables` |
| GET | `/admin/stick-tables/entries?backend=&limit=` |
| DELETE | `/admin/stick-tables?backend=` |
| DELETE | `/admin/stick-tables/entry?key=` |

### Soak notes

See **[deploy/ha/README.md](../deploy/ha/README.md)** for the turnkey dual-node
package, keepalived, compose lab, and soak checklist.

## Config reload under HA

- `SIGHUP` / `POST /admin/reload` swaps routes, ACLs, backends, servers (state
  preserved by name), header rules, app auth, **OIDC**, and **peers** config,
  and **rebinds** HTTP, TCP, UDP, HTTP/3, **stats**, and **peers** listen
  sockets when bind sets change (reuse when host:port match).
- Peers identity/secret/remote-list changes reset sessions and re-dial.
- Prefer practicing reload on the backup node before VIP failover.

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
