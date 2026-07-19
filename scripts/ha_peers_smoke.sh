#!/usr/bin/env bash
# Dual-node peers smoke (localhost).
# Validates: dual start, HELLO, metrics, proxy with peers open, stick UPSERT sync,
# reconnect after backup restart, proxy after reconnect.
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
  stick on src
  stick_table size 1000 expire 1h
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
  stick on src
  stick_table size 1000 expire 1h
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

alive() {
  kill -0 "$1" 2>/dev/null
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
if ! alive "$A_PID" || ! alive "$B_PID"; then
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

echo "== peers metrics =="
for port in 19281 19291; do
  m=$(curl -s --max-time 2 "${AUTH[@]}" "http://127.0.0.1:${port}/metrics" || true)
  if ! echo "$m" | grep -q leba_peers_enabled; then
    echo "FAIL: peers metrics missing on :$port" >&2
    exit 1
  fi
  echo "$m" | grep leba_peers_ || true
done

echo "== proxy with peers open (stick on src) =="
for i in 1 2 3; do
  pa=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 -H "Connection: close" "http://127.0.0.1:19280/" || echo 000)
  pb=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 -H "Connection: close" "http://127.0.0.1:19290/" || echo 000)
  echo "  round $i a=$pa b=$pb"
  if [[ "$pa" != "200" || "$pb" != "200" ]]; then
    echo "FAIL: proxy with peers open" >&2
    exit 1
  fi
  if ! alive "$A_PID" || ! alive "$B_PID"; then
    echo "FAIL: process died during proxy" >&2
    exit 1
  fi
done

echo "== stick UPSERT peer sync =="
# A should have broadcast at least one UPSERT; B should have received it.
ma=$(curl -s --max-time 2 "${AUTH[@]}" "http://127.0.0.1:19281/metrics" || true)
mb=$(curl -s --max-time 2 "${AUTH[@]}" "http://127.0.0.1:19291/metrics" || true)
out_a=$(echo "$ma" | awk '/^leba_peers_upserts_out_total /{print $2; exit}')
in_b=$(echo "$mb" | awk '/^leba_peers_upserts_in_total /{print $2; exit}')
echo "  upserts_out A=${out_a:-0} upserts_in B=${in_b:-0}"
if [[ "${out_a:-0}" -lt 1 ]]; then
  echo "FAIL: expected leba_peers_upserts_out_total >= 1 on A" >&2
  exit 1
fi
if [[ "${in_b:-0}" -lt 1 ]]; then
  echo "FAIL: expected leba_peers_upserts_in_total >= 1 on B" >&2
  exit 1
fi
sa=$(curl -s --max-time 2 "${AUTH[@]}" "http://127.0.0.1:19281/admin/stick-tables" || true)
sb=$(curl -s --max-time 2 "${AUTH[@]}" "http://127.0.0.1:19291/admin/stick-tables" || true)
echo "  stick A: $sa"
echo "  stick B: $sb"
if ! echo "$sa" | grep -q '"live_count":[1-9]'; then
  echo "FAIL: stick table empty on A" >&2
  exit 1
fi
if ! echo "$sb" | grep -q '"live_count":[1-9]'; then
  echo "FAIL: stick table empty on B (sync failed)" >&2
  exit 1
fi

echo "== restart B (reconnect) =="
kill "$B_PID" 2>/dev/null || true
wait "$B_PID" 2>/dev/null || true
B_PID=""
sleep 1
if ! alive "$A_PID"; then
  echo "FAIL: A died when B stopped" >&2
  exit 1
fi
"$LEBA" -f "$TMP/leba-b.conf" >"$TMP/b2.log" 2>&1 &
B_PID=$!
wait_ready 19291 b_restart 30
sleep 5
if ! alive "$A_PID"; then
  echo "FAIL: A died after B re-HELLO" >&2
  tail -20 "$TMP/a.log" >&2 || true
  exit 1
fi
if ! alive "$B_PID"; then
  echo "FAIL: B died after restart" >&2
  exit 1
fi

echo "== proxy after reconnect =="
pa=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 -H "Connection: close" "http://127.0.0.1:19280/" || echo 000)
pb=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 -H "Connection: close" "http://127.0.0.1:19290/" || echo 000)
echo "  a=$pa b=$pb"
if [[ "$pa" != "200" || "$pb" != "200" ]]; then
  echo "FAIL: proxy after reconnect" >&2
  exit 1
fi
if ! alive "$A_PID" || ! alive "$B_PID"; then
  echo "FAIL: process died after reconnect proxy" >&2
  exit 1
fi

echo "HA PEERS SMOKE PASS"
echo "  HELLO + proxy + stick UPSERT sync + reconnect OK"
