# Admin OIDC SSO

Leba can authenticate the **admin UI / stats plane** with an OpenID Connect
provider (authorization code flow). Successful login issues the same
`leba_session` cookie used by password login (RBAC: viewer / operator / admin).

## Config

```text
defaults
  state_key change-me-to-random-32chars

oidc
  enabled on
  issuer https://login.example.com/realms/leba
  client_id leba-admin
  client_secret $LEBA_OIDC_CLIENT_SECRET
  redirect_uri https://admin.example.com:9443/oidc/callback
  scopes openid email profile
  default_role viewer
  admin_group leba-admins
  operator_group leba-operators
  # optional endpoint overrides (skip discovery)
  # authorize_url https://...
  # token_url https://...
  # userinfo_url https://...
  # jwks_uri https://.../protocol/openid-connect/certs
  # rsa_public_key /etc/leba/idp.pub.pem
  # verify_jwt on
  # ca_file /etc/ssl/certs/ca-certificates.crt
  # insecure off
  # disable_password off
```

| Key | Meaning |
|-----|---------|
| `issuer` | OIDC issuer; used for discovery (`/.well-known/openid-configuration`) and `iss` check |
| `client_id` / `client_secret` | Confidential client credentials |
| `redirect_uri` | Must match IdP app config exactly (`/oidc/callback` on the **stats** frontend) |
| `scopes` | Default `openid email profile` |
| `jwks_uri` | JWKS URL for RS256 id_token verify (Mako `jwt_verify_jwks`). Filled from discovery when empty |
| `rsa_public_key` | Optional PEM path or PEM text for `jwt_verify_rs256` (overrides JWKS when set) |
| `verify_jwt` | `on` (default) = verify id_token when JWKS/PEM available; `off` = decode only |
| `default_role` | Role when no group maps (`viewer` recommended) |
| `admin_group` / `operator_group` | Values matched inside the groups claim |
| `groups_claim` | Claim name (default `groups`); also accepts `leba_role` = admin\|operator\|viewer |
| `email_claim` | Session username claim (default `email`, then `sub`) |
| `disable_password` | `on` = SSO only (hide local password form) |
| `ca_file` | CA PEM for IdP HTTPS (empty = Mako platform trust) |
| `insecure` | Skip TLS verify to IdP (**dev only**; bypasses verified `https_*`) |

## Endpoints

| Path | Purpose |
|------|---------|
| `GET /login` | Login page (SSO button when OIDC ready) |
| `GET /oidc/login` | Start authorize redirect (`state` cookie) |
| `GET /oidc/callback` | Code exchange → session cookie → `/` |

## Flow

1. Operator opens `/login` → **Sign in with SSO**.
2. Leba redirects to the IdP authorize URL with CSRF `state`.
3. IdP redirects to `redirect_uri` with `code` + `state`.
4. Leba POSTs to the token endpoint (Mako `oidc_token` / verified HTTPS),
   verifies `id_token` when JWKS/PEM is available, maps role, sets
   `leba_session`.

Token trust model (Mako **0.2.2+**):

1. **Transport** — discovery and token exchange use Mako `oidc_discovery` /
   `oidc_token` (verified TLS; empty `ca_file` = platform trust).
2. **Signature** — when `jwks_uri` is known (discovery or config), id_token is
   checked with `jwt_verify_jwks`. Optional `rsa_public_key` uses
   `jwt_verify_rs256`. Set `verify_jwt off` only for lab.
3. **Claims** — payload via `jwt_payload`; Leba still checks `exp`, `iss`,
   `aud` when present.

Userinfo fallback (Bearer access token) still uses the socket TLS client
because Mako `https_*` has no custom Authorization header API.

## Role mapping

1. Claim `leba_role` if exactly `admin` / `operator` / `viewer`.
2. Else if `admin_group` appears in `groups` claim → `admin`.
3. Else if `operator_group` appears → `operator`.
4. Else `default_role`.

## Requirements

- Stats frontend should use **TLS** so `Secure` session cookies work.
- Set `state_key` or `LEBA_SESSION_SECRET` (same as password sessions).
- Firewall IdP reachability from the Leba host (egress HTTPS).

## Reload

`SIGHUP` / `POST /admin/reload` reloads the `oidc` section and re-runs discovery
when the client is ready (`oidc_apply_reload`). No process restart is required
to change issuer, client credentials, or JWKS settings.

## Related

- `docs/ADMIN_API.md` — RBAC roles
- `docs/SECURITY.md` — admin plane hardening
