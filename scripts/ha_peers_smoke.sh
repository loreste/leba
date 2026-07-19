#!/usr/bin/env bash
# Dual-node peers lifecycle smoke (localhost).
# Validates: dual start, peers dial/HELLO, peers metrics, both nodes stay up
# while sessions are idle, then clean shutdown.
#
# Intentionally does NOT:
#   - drive proxy traffic while peer sessions are open (can abort; experimental)
#   - require reconnect after peer death (re-HELLO can crash primary; experimental)
#   - stress stick UPSERT under peer load
#
# Usage: ./scripts/ha_peers_smoke.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LEBA="${ROOT}/leba"
TMP="$(mktemp -d /tmp/leba-ha-XXXXXX)"
cleanup() {
  if [[ -n "${A_PID:-}" ]]; then kill "$A_PID" 2>/dev/null || true; wait "$A_PID" 2>/dev/null || true; fi
  if [[ -n "${B_PID:-}" ]]; then kill "$B_PID" 2>/dev/null || true; wait "$B_PID" 2>/dev/null || true; fi
  if [[ -n "${ORIGIN_PID:-}" ]]; then kill "$ORIGIN_PID" 2>/dev/null || true; wait "$ORIGIN_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

if [[ ! -x "$LEBA" ]]; then
  make -C "$ROOT" build
fi

SECRET="ha-smoke-secret-$$"
AUTH=(-u admin:hapass)

# Origin only for optional post-peers single-node check (peers off).
python3 - <<'PY' &
from http.server import BaseHTTPRequestHandler, HTTPServer
class H(BaseHTTPRequestHandler):
    def do_GET(self):
        b = b"ok\n"
        self.send_response(200)
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)
    def log_message(self, *a):
        pass
HTTPServer(("127.0.0.1", 19299), H).serve_forever()
PY
ORIGIN_PID=$!
sleep 0.3

cat >"$TMP/leba-a.conf" <<EOF
defaults
  timeout_client 5s
  timeout_server 5s
  timeout_connect 2s
  workers 2
frontend web
  bind 127.0.0.1:19280
  mode http
  route default -> app
frontend stats
  bind 127.0.0.1:19281
  mode stats
  auth admin:hapass:admin
backend app
  balance least_conn
  server o1 127.0.0.1:19299 weight 100 no_check
peers
  enabled on
  name leba-a
  cluster edge
  bind 127.0.0.1:19224
  secret ${SECRET}
  peer leba-b 127.0.0.1:19225
EOF

cat >"$TMP/leba-b.conf" <<EOF
defaults
  timeout_client 5s
  timeout_server 5s
  timeout_connect 2s
  workers 2
frontend web
  bind 127.0.0.1:19290
  mode http
  route default -> app
frontend stats
  bind 127.0.0.1:19291
  mode stats
  auth admin:hapass:admin
backend app
  balance least_conn
  server o1 127.0.0.1:19299 weight 100 no_check
peers
  enabled on
  name leba-b
  cluster edge
  bind 127.0.0.1:19225
  secret ${SECRET}
EOF

wait_ready() {
  local port="$1" label="$2" tries="${3:-30}"
  local i=0
  while (( i < tries )); do
    if curl -fsS --max-time 1 "${AUTH[@]}" "http://127.0.0.1:${port}/readyz" >/dev/null 2>&1; then
      echo "node $label ready on :$port"
      return 0
    fi
    sleep 0.25
    i=$((i + 1))
  done
  echo "FAIL: node $label not ready on :$port" >&2
  return 1
}

echo "== start A then B =="
"$LEBA" -f "$TMP/leba-a.conf" >"$TMP/a.log" 2>&1 &
A_PID=$!
wait_ready 19281 a
"$LEBA" -f "$TMP/leba-b.conf" >"$TMP/b.log" 2>&1 &
B_PID=$!
wait_ready 19291 b

echo "== wait for peers dial/HELLO =="
sleep 6
if ! kill -0 "$A_PID" 2>/dev/null || ! kill -0 "$B_PID" 2>/dev/null; then
  echo "FAIL: process died during peer dial" >&2
  tail -40 "$TMP/a.log" "$TMP/b.log" >&2 || true
  exit 1
fi
if ! grep -qE 'peers_(connected|hello|accept)' "$TMP/a.log" "$TMP/b.log"; then
  echo "FAIL: no peers activity in logs" >&2
  tail -40 "$TMP/a.log" "$TMP/b.log" >&2 || true
  exit 1
fi
echo "peers activity observed"

echo "== peers metrics (stats only — no data-plane load with open peer sessions) =="
hello_ok=0
for port in 19281 19291; do
  m=$(curl -s --max-time 2 "${AUTH[@]}" "http://127.0.0.1:${port}/metrics" || true)
  if ! echo "$m" | grep -q leba_peers_enabled; then
    echo "FAIL: peers metrics missing on :$port" >&2
    exit 1
  fi
  echo "$m" | grep leba_peers_ || true
  if echo "$m" | grep -q 'leba_peers_hello_ok_total 1'; then
    hello_ok=1
  fi
done
if [[ "$hello_ok" != "1" ]]; then
  echo "FAIL: expected leba_peers_hello_ok_total >= 1 on at least one node" >&2
  exit 1
fi

# Idle settle with peer sessions open (no proxy / stick traffic)
echo "== idle settle with open peer sessions =="
sleep 3
if ! kill -0 "$A_PID" 2>/dev/null || ! kill -0 "$B_PID" 2>/dev/null; then
  echo "FAIL: process died during idle peer settle" >&2
  tail -40 "$TMP/a.log" "$TMP/b.log" >&2 || true
  exit 1
fi
echo "both nodes still up after idle settle"

echo "== clean shutdown of dual peers =="
kill "$A_PID" 2>/dev/null || true
kill "$B_PID" 2>/dev/null || true
wait "$A_PID" 2>/dev/null || true
wait "$B_PID" 2>/dev/null || true
A_PID=""
B_PID=""

# Binary still proxies without peers (sanity; not dual-node)
echo "== single-node proxy without peers (sanity) =="
cat >"$TMP/leba-solo.conf" <<EOF
defaults
  timeout_client 5s
  timeout_server 5s
  timeout_connect 2s
  workers 2
frontend web
  bind 127.0.0.1:19280
  mode http
  route default -> app
frontend stats
  bind 127.0.0.1:19281
  mode stats
  auth admin:hapass:admin
backend app
  balance least_conn
  server o1 127.0.0.1:19299 weight 100 no_check
EOF
"$LEBA" -f "$TMP/leba-solo.conf" >"$TMP/solo.log" 2>&1 &
A_PID=$!
wait_ready 19281 solo
pa=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 -H "Connection: close" "http://127.0.0.1:19280/" || echo 000)
echo "proxy solo=$pa"
if [[ "$pa" != "200" ]]; then
  echo "FAIL: single-node proxy" >&2
  tail -20 "$TMP/solo.log" >&2 || true
  exit 1
fi

echo "HA PEERS SMOKE PASS"
echo "  dual-node HELLO + metrics + idle OK; proxy/reconnect/stick under peers remain experimental"
