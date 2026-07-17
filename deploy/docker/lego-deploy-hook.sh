#!/bin/sh
# lego / acme.sh deploy hook: after certs land on disk, ask Leba to live-reload TLS.
# Requires stats frontend on localhost or reachable admin URL with Basic auth.
set -eu

ADMIN_URL="${LEBA_ADMIN_URL:-http://127.0.0.1:8404}"
AUTH="${LEBA_ADMIN_AUTH:-}"

if [ -z "$AUTH" ]; then
  echo "leba deploy-hook: set LEBA_ADMIN_AUTH=user:pass" >&2
  exit 1
fi

# Optional: copy lego PEM into paths referenced by leba.conf
# CERT_PATH / KEY_PATH may be set by the ACME tool environment.

curl -fsS -u "$AUTH" -X POST "${ADMIN_URL}/admin/tls-reload" \
  || curl -fsS -u "$AUTH" -X POST "${ADMIN_URL}/admin/tls-reload/"

echo "leba deploy-hook: tls-reload requested"
