# Security Notes

Leba is intended to be deployed on internet-facing hosts, so the default
operational stance should be conservative.

## Admin Surface

Configure role-specific credentials on every `mode stats` frontend before
exposing it.

```text
frontend stats
  bind 18404
  mode stats
  admin_users_file /etc/leba/admin-users.conf
```

`admin-users.conf` stores role-specific password hashes:

```text
admin $argon2id$v=19$m=19456,t=2,p=1$SALT$HASH admin
viewer leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX viewer
operator leba-kdf-v1:ITERATIONS:SALT_HEX:HASH_HEX operator
```

New hashes use Argon2id when the linked Mako crypto backend supports it and
fall back to `leba-kdf-v1` otherwise. Legacy SHA-256 hashes are accepted only
for compatibility.

When credentials are configured, Leba protects:

- the admin dashboard,
- `/stats`,
- `/metrics`,
- `/admin/*` runtime actions.

Probe endpoints remain unauthenticated:

- `/health`
- `/livez`
- `/readyz`

Unauthenticated admin requests return `401` JSON.
Authenticated users without enough role privilege receive `403`.

Roles:

| Role | Access |
|------|--------|
| `viewer` | Dashboard, stats, metrics, and server listing. |
| `operator` | Viewer access plus drain, ready, disable, enable, and `servers_file` reload. |
| `admin` | Full admin access. |

## Credentials

The sample local config uses demo plaintext credentials. The Linux template
uses `admin_users_file` with `CHANGE_ME_*` placeholders. `leba doctor` warns on
placeholder/demo values and errors on malformed hashes.

Use long random passwords and store only salted, iterated hashes in production
configs. Generate hashes with `leba admin hash-password 'strong-password'`.
Treat the admin endpoint as a privileged control plane because it can drain,
enable, disable, and reload upstream server membership.

## Network Exposure

Recommended deployment shape:

- expose only the public HTTP/TCP/SIP frontend ports to the internet,
- keep the stats/admin port on a private interface or behind trusted network
  controls,
- use host firewall rules to restrict admin access,
- run `leba doctor` before restarting a production instance.

## Request Handling

Current hardening:

- raw HTTP requests larger than 1 MiB are rejected before upstream forwarding,
- raw HTTP request limits are configurable with `request_body_limit`,
- protected admin requests write audit logs with request id, authenticated
  user, role, method, path, status, and outcome,
- ACL denies are enforced before backend selection,
- rate limits are enforced before upstream forwarding,
- backend and server `maxconn` caps fail closed when saturated,
- all-drained or all-down pools fail closed instead of silently choosing an
  unavailable server,
- trace headers are validated before use and forwarded upstream only after
  validation or regeneration.

## Session Cookies

Admin UI session cookies are signed with material from, in order:

1. `state_key` in defaults (preferred),
2. `LEBA_SESSION_SECRET` environment variable,
3. an insecure local-dev default (doctor warns when neither 1 nor 2 is set).

## Remaining Security Work

These are still open:

- configurable probe authentication policy,
- broader TLS and HTTP/2 accept-path hardening,
- RTP/media handling.
