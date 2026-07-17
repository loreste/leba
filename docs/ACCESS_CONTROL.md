# Access control for proxied applications

Leba supports two complementary data-plane controls on HTTP frontends:

1. **IP allow/deny** (existing ACL engine)
2. **HTTP Basic auth** for applications (`auth_basic` + `auth_user`)

These are independent of **admin** RBAC on the stats frontend.

## IP access lists

```text
frontend web
  bind 80
  mode http
  deny src 203.0.113.
  allow src 10.0.0.
  allow src 192.168.1.
  route default -> app
```

- `deny` wins over `allow` when both match.
- If any `allow` rules exist for a frontend, unmatched clients are denied (allow-list mode).
- Match kinds: `src`, `path`, `path_prefix`, `host`, `method`, etc. (see CONFIG_REFERENCE).

Combine with Basic auth for defense in depth.

## HTTP Basic for apps

```text
frontend secure_app
  bind 8080
  mode http
  auth_basic "Internal Tools"
  auth_user alice s3cret
  auth_user bob hunter2
  route host tools.example.com -> tools
  route default -> tools
```

Behavior:

| Request | Result |
|---------|--------|
| Missing/invalid `Authorization` | `401` + `WWW-Authenticate: Basic realm="…"` |
| Valid user/pass | Proxied normally |
| `/.well-known/acme-challenge/*` with `acme_webroot` | **No** Basic required |

Credentials are plaintext in the config file. Restrict file permissions and prefer
admin hashed users (`admin_user_hash`) for the control plane.

## Admin plane (not app Basic)

Stats frontend uses `admin_users_file` / session cookies / RBAC. Do not reuse
`auth_user` for admin accounts.

## NPM mental model

| NPM concept | Leba |
|-------------|------|
| Access List (IP) | `allow` / `deny` `src` ACLs |
| Access List (HTTP Basic) | `auth_basic` + `auth_user` |
| Proxy host | `POST /admin/proxy-host` or `route host` |
