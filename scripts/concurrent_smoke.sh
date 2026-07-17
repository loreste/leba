#!/usr/bin/env bash
# Concurrent smoke: spin origin + leba, fire parallel requests, check status.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LEBA="${ROOT}/leba"
TMP="$(mktemp -d /tmp/leba-conc-XXXXXX)"
cleanup() {
  if [[ -n "${LEBA_PID:-}" ]]; then kill "$LEBA_PID" 2>/dev/null || true; fi
  if [[ -n "${ORIGIN_PID:-}" ]]; then kill "$ORIGIN_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

python3 - <<'PY' &
from http.server import BaseHTTPRequestHandler, HTTPServer
class H(BaseHTTPRequestHandler):
    def do_GET(self):
        b = b"ok\n"
        self.send_response(200)
        self.send_header("Content-Length", str(len(b)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(b)
    def log_message(self, *a):
        pass
HTTPServer(("127.0.0.1", 19099), H).serve_forever()
PY
ORIGIN_PID=$!
sleep 0.3

cat >"$TMP/leba.conf" <<EOF
defaults
  timeout_client 5s
  timeout_server 5s
  timeout_connect 2s
  workers 8
  retries 1
frontend web
  bind 127.0.0.1:19080
  mode http
  route default -> app
frontend stats
  bind 127.0.0.1:19081
  mode stats
  auth admin:smokepass:admin
backend app
  balance round_robin
  server o1 127.0.0.1:19099 weight 100 no_check
EOF

"$LEBA" -f "$TMP/leba.conf" >"$TMP/leba.log" 2>&1 &
LEBA_PID=$!
sleep 0.5

N="${1:-100}"
# Parallel wave (do not wait on long-lived origin/leba PIDs)
pids=()
for i in $(seq 1 "$N"); do
  (
    curl -s -o /dev/null -w "%{http_code}" --max-time 2 -H "Connection: close" "http://127.0.0.1:19080/" || echo 000
  ) >"$TMP/c-$i.out" &
  pids+=($!)
  if (( ${#pids[@]} >= 32 )); then
    for p in "${pids[@]}"; do wait "$p" || true; done
    pids=()
  fi
done
if (( ${#pids[@]} > 0 )); then
  for p in "${pids[@]}"; do wait "$p" || true; done
fi

par_ok=0
par_fail=0
for i in $(seq 1 "$N"); do
  code=$(cat "$TMP/c-$i.out" 2>/dev/null || echo 000)
  if [[ "$code" == "200" ]]; then par_ok=$((par_ok+1)); else par_fail=$((par_fail+1)); fi
done

# Serial keep-alive sample
ser_ok=0
ser_fail=0
for i in $(seq 1 20); do
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 "http://127.0.0.1:19080/" || echo 000)
  if [[ "$code" == "200" ]]; then ser_ok=$((ser_ok+1)); else ser_fail=$((ser_fail+1)); fi
done

echo "concurrent_smoke: parallel ok=$par_ok fail=$par_fail (n=$N); serial ok=$ser_ok fail=$ser_fail"
if [[ "$par_fail" -gt 5 ]] || [[ "$ser_fail" -gt 0 ]]; then
  echo "FAIL" >&2
  tail -80 "$TMP/leba.log" >&2 || true
  exit 1
fi
echo "PASS"
