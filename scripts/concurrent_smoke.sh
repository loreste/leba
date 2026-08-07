#!/usr/bin/env bash
# Concurrent smoke: origin + leba under parallel load, keep-alive, POST, OPTIONS.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LEBA="${ROOT}/leba"
TMP="$(mktemp -d /tmp/leba-conc-XXXXXX)"
# Ephemeral-ish ports avoid collisions with leftover smoke/soak processes.
ORIGIN_PORT="${ORIGIN_PORT:-$((19090 + ($$ % 80)))}"
LEBA_PORT="${LEBA_PORT:-$((19190 + ($$ % 80)))}"
STATS_PORT="${STATS_PORT:-$((19290 + ($$ % 80)))}"

cleanup() {
  set +m 2>/dev/null || true
  if [[ -n "${LEBA_PID:-}" ]]; then kill "$LEBA_PID" 2>/dev/null || true; wait "$LEBA_PID" 2>/dev/null || true; fi
  if [[ -n "${ORIGIN_PID:-}" ]]; then kill "$ORIGIN_PID" 2>/dev/null || true; wait "$ORIGIN_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

http_code() {
  # Slightly longer timeout for shared CI runners under parallel load.
  curl -s -o /dev/null -w "%{http_code}" --max-time "${CURL_MAX_TIME:-3}" "$@" || true
}
export -f http_code
export CURL_MAX_TIME="${CURL_MAX_TIME:-3}"

python3 - "$ORIGIN_PORT" <<'PY' &
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

port = int(sys.argv[1])

class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _reply(self, code, body):
        b = body if isinstance(body, (bytes, bytearray)) else body.encode()
        self.send_response(code)
        self.send_header("Content-Length", str(len(b)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(b)

    def do_GET(self):
        self._reply(200, b"ok\n")

    def do_POST(self):
        n = int(self.headers.get("Content-Length", "0") or "0")
        body = self.rfile.read(n) if n > 0 else b""
        csrf = self.headers.get("X-CSRF-Token", "")
        origin = self.headers.get("Origin", "")
        ct = self.headers.get("Content-Type", "")
        payload = f"len={len(body)};csrf={csrf};origin={origin};ct={ct}\n".encode()
        self._reply(200, payload)

    def do_OPTIONS(self):
        self._reply(204, b"")

    def log_message(self, *a):
        pass

# Threading origin: single-thread HTTPServer serializes accepts and flakes
# keep-alive waves on small shared CI runners.
ThreadingHTTPServer(("127.0.0.1", port), H).serve_forever()
PY
ORIGIN_PID=$!

for _ in $(seq 1 50); do
  if curl -s -o /dev/null --max-time 0.2 "http://127.0.0.1:${ORIGIN_PORT}/" 2>/dev/null; then
    break
  fi
  sleep 0.05
done

cat >"$TMP/leba.conf" <<EOF
defaults
  timeout_client 5s
  timeout_server 5s
  timeout_connect 2s
  workers 8
  retries 1
frontend web
  bind 127.0.0.1:${LEBA_PORT}
  mode http
  route default -> app
frontend stats
  bind 127.0.0.1:${STATS_PORT}
  mode stats
  auth admin:smokepass:admin
backend app
  balance round_robin
  server o1 127.0.0.1:${ORIGIN_PORT} weight 100 no_check
EOF

"$LEBA" -f "$TMP/leba.conf" >"$TMP/leba.log" 2>&1 &
LEBA_PID=$!

ready=0
for _ in $(seq 1 80); do
  code=$(http_code -H "Connection: close" "http://127.0.0.1:${LEBA_PORT}/" || true)
  if [[ "$code" == "200" ]]; then
    ready=1
    break
  fi
  sleep 0.05
done
if [[ "$ready" != "1" ]]; then
  echo "FAIL: leba not ready on :${LEBA_PORT}" >&2
  tail -80 "$TMP/leba.log" >&2 || true
  exit 1
fi

N="${1:-100}"
# Allow CI to raise budget slightly; default still strict.
fail_budget="${CONC_FAIL_BUDGET:-8}"
# Parallelism cap (GHA shared runners choke at 32+ simultaneous curls).
wave_parallel="${CONC_PARALLEL:-12}"
BASE="http://127.0.0.1:${LEBA_PORT}"

run_wave() {
  local label="$1"
  local n="$2"
  local cmd="$3"
  local pids=()
  local i
  for i in $(seq 1 "$n"); do
    (
      eval "$cmd"
    ) >"$TMP/${label}-$i.out" 2>/dev/null &
    pids+=($!)
    if (( ${#pids[@]} >= wave_parallel )); then
      for p in "${pids[@]}"; do wait "$p" || true; done
      pids=()
    fi
  done
  if (( ${#pids[@]} > 0 )); then
    for p in "${pids[@]}"; do wait "$p" || true; done
  fi
  local ok=0 fail=0
  for i in $(seq 1 "$n"); do
    code=$(tr -d '[:space:]' <"$TMP/${label}-$i.out" 2>/dev/null || echo 000)
    if [[ "$code" == "200" || "$code" == "204" ]]; then
      ok=$((ok + 1))
    else
      fail=$((fail + 1))
    fi
  done
  echo "${label}: ok=$ok fail=$fail (n=$n)"
  if [[ "$fail" -gt "$fail_budget" ]]; then
    echo "FAIL wave $label" >&2
    tail -80 "$TMP/leba.log" >&2 || true
    exit 1
  fi
}

run_wave "get" "$N" "http_code -H \"Connection: close\" \"${BASE}/\""
ka_n=$((N / 2))
if (( ka_n < 20 )); then ka_n=20; fi
# HTTP keep-alive header (not curl --keepalive-time, which is TCP idle).
run_wave "ka" "$ka_n" "http_code -H \"Connection: keep-alive\" \"${BASE}/\""
post_n=$((N / 2))
if (( post_n < 20 )); then post_n=20; fi
run_wave "post" "$post_n" \
  "http_code -X POST -H \"Content-Type: application/json\" -H \"X-CSRF-Token: smoke-csrf\" -H \"Origin: https://ui.example.com\" -H \"X-Attachment-Filename: t.bin\" --data \"{\\\"n\\\":1}\" \"${BASE}/upload\""
run_wave "opt" 20 \
  "http_code -X OPTIONS -H \"Origin: https://ui.example.com\" -H \"Access-Control-Request-Method: POST\" \"${BASE}/upload\""

ser_ok=0
ser_fail=0
for i in $(seq 1 20); do
  code=$(http_code "${BASE}/")
  if [[ "$code" == "200" ]]; then ser_ok=$((ser_ok+1)); else ser_fail=$((ser_fail+1)); fi
done
echo "serial_ka: ok=$ser_ok fail=$ser_fail"
if [[ "$ser_fail" -gt 0 ]]; then
  echo "FAIL serial keep-alive" >&2
  tail -80 "$TMP/leba.log" >&2 || true
  exit 1
fi

body=$(curl -s --max-time 2 -X POST \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: smoke-csrf" \
  -H "Origin: https://ui.example.com" \
  --data '{"n":1}' \
  "${BASE}/upload" || true)
if [[ "$body" != *"csrf=smoke-csrf"* ]] || [[ "$body" != *"origin=https://ui.example.com"* ]]; then
  echo "FAIL header forward: got '$body'" >&2
  tail -80 "$TMP/leba.log" >&2 || true
  exit 1
fi

echo "concurrent_smoke: PASS (n=$N ports origin=${ORIGIN_PORT} leba=${LEBA_PORT})"
echo "PASS"
