#!/usr/bin/env bash
# Docker-backed reverse-proxy benchmark matrix: Leba vs nginx vs HAProxy.
#
# This is a local regression/claim gate, not a lab-grade capacity study.
# It uses one local keep-alive origin and sends the same concurrent HTTP load to
# each proxy. Set LEBA_REQUIRE_WIN=1 to fail if Leba does not beat both peers.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LEBA_BIN="${LEBA_BIN:-${ROOT}/leba}"
SEC="${1:-8}"
CONC="${2:-40}"
REQUIRE_WIN="${LEBA_REQUIRE_WIN:-0}"
NGINX_IMAGE="${NGINX_IMAGE:-nginx:1.27-alpine}"
HAPROXY_IMAGE="${HAPROXY_IMAGE:-haproxy:2.9-alpine}"
TMP="$(mktemp -d /tmp/leba-proxy-matrix-XXXXXX)"

ORIGIN_PORT="${ORIGIN_PORT:-$((21000 + ($$ % 1000)))}"
LEBA_PORT="${LEBA_PORT:-$((22000 + ($$ % 1000)))}"
STATS_PORT="${STATS_PORT:-$((23000 + ($$ % 1000)))}"
NGINX_PORT="${NGINX_PORT:-$((24000 + ($$ % 1000)))}"
HAPROXY_PORT="${HAPROXY_PORT:-$((25000 + ($$ % 1000)))}"
NGINX_NAME="${NGINX_NAME:-leba-bench-nginx-$$}"
HAPROXY_NAME="${HAPROXY_NAME:-leba-bench-haproxy-$$}"
DOCKER_READY=0

cleanup() {
  if [ "$DOCKER_READY" = "1" ]; then
    docker rm -f "$NGINX_NAME" "$HAPROXY_NAME" >/dev/null 2>&1 || true
  fi
  kill ${ORIGIN_PID:-} ${LEBA_PID:-} 2>/dev/null || true
  wait ${ORIGIN_PID:-} ${LEBA_PID:-} 2>/dev/null || true
  rm -rf "$TMP"
}
trap cleanup EXIT

run_timeout() {
  local seconds="$1"
  shift
  "$@" &
  local pid=$!
  local waited=0
  while kill -0 "$pid" 2>/dev/null; do
    if [ "$waited" -ge "$seconds" ]; then
      kill "$pid" 2>/dev/null || true
      sleep 1
      kill -9 "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
      return 124
    fi
    sleep 1
    waited=$((waited + 1))
  done
  wait "$pid"
}

rss_kib() {
  local pid="$1"
  ps -o rss= -p "$pid" 2>/dev/null | tr -d ' ' || true
}

docker_mem() {
  local name="$1"
  docker stats --no-stream --format '{{.MemUsage}}' "$name" 2>/dev/null | awk '{print $1 $2}' || true
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}

require_cmd python3
require_cmd curl
require_cmd docker

if [ ! -x "$LEBA_BIN" ]; then
  echo "missing Leba binary: $LEBA_BIN; run make build first" >&2
  exit 1
fi

echo "checking Docker..."
if ! run_timeout 8 docker info >/dev/null 2>&1; then
  echo "Docker is not responsive; start Docker and retry" >&2
  exit 2
fi
DOCKER_READY=1

ulimit -n 65536 2>/dev/null || true

cat >"$TMP/origin.py" <<'PY'
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import sys

class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def do_GET(self):
        body = b"ok\n"
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Connection", "keep-alive")
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, fmt, *args):
        pass

ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PY

python3 "$TMP/origin.py" "$ORIGIN_PORT" >/tmp/leba_bench_origin.log 2>&1 &
ORIGIN_PID=$!

cat >"$TMP/leba.conf" <<EOF
defaults
  workers 64
  timeout_client 30s
  timeout_server 30s
  timeout_connect 3s

frontend web
  bind 127.0.0.1:${LEBA_PORT}
  mode http
  access_log off
  route default -> app

frontend stats
  bind 127.0.0.1:${STATS_PORT}
  mode stats
  auth admin:benchpass:admin

backend app
  balance round_robin
  server origin 127.0.0.1:${ORIGIN_PORT} weight 100 no_check
EOF

cat >"$TMP/nginx.conf" <<EOF
worker_processes 1;
error_log /tmp/nginx-error.log crit;
events { worker_connections 16384; multi_accept on; }
http {
  access_log off;
  upstream origin {
    server host.docker.internal:${ORIGIN_PORT};
    keepalive 128;
  }
  server {
    listen 8080;
    location / {
      proxy_http_version 1.1;
      proxy_set_header Connection "";
      proxy_pass http://origin;
    }
  }
}
EOF

cat >"$TMP/haproxy.cfg" <<EOF
global
  maxconn 20000

defaults
  mode http
  timeout connect 3s
  timeout client 30s
  timeout server 30s
  option http-keep-alive

frontend web
  bind *:8080
  default_backend origin

backend origin
  http-reuse always
  server origin host.docker.internal:${ORIGIN_PORT} maxconn 20000
EOF

"$LEBA_BIN" -f "$TMP/leba.conf" >/tmp/leba_bench_matrix.log 2>&1 &
LEBA_PID=$!

echo "starting proxies..."
docker rm -f "$NGINX_NAME" "$HAPROXY_NAME" >/dev/null 2>&1 || true
run_timeout 30 docker run -d --name "$NGINX_NAME" \
  -p "127.0.0.1:${NGINX_PORT}:8080" \
  --add-host=host.docker.internal:host-gateway \
  -v "$TMP/nginx.conf:/etc/nginx/nginx.conf:ro" \
  "$NGINX_IMAGE" >/dev/null
run_timeout 30 docker run -d --name "$HAPROXY_NAME" \
  -p "127.0.0.1:${HAPROXY_PORT}:8080" \
  --add-host=host.docker.internal:host-gateway \
  -v "$TMP/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro" \
  "$HAPROXY_IMAGE" >/dev/null

wait_ready() {
  local label="$1"
  local url="$2"
  local i code
  for i in $(seq 1 80); do
    code="$(curl -s -o /dev/null -w "%{http_code}" --max-time 1 "$url" || true)"
    if [ "$code" = "200" ]; then
      return 0
    fi
    sleep 0.1
  done
  echo "not ready: $label $url" >&2
  tail -60 /tmp/leba_bench_matrix.log 2>/dev/null || true
  docker logs "$NGINX_NAME" 2>/dev/null || true
  docker logs "$HAPROXY_NAME" 2>/dev/null || true
  exit 1
}

bench_one() {
  local label="$1"
  local url="$2"
  python3 - "$label" "$url" "$SEC" "$CONC" <<'PY'
import concurrent.futures
import statistics
import sys
import time
import urllib.request

label, url, raw_sec, raw_conc = sys.argv[1:5]
seconds = float(raw_sec)
conc = int(raw_conc)
deadline = time.perf_counter() + seconds
lat = []
ok = 0
fail = 0

def one():
    start = time.perf_counter()
    try:
        req = urllib.request.Request(url, headers={"Connection": "keep-alive"})
        with urllib.request.urlopen(req, timeout=3) as resp:
            resp.read()
            good = resp.status == 200
    except Exception:
        good = False
    elapsed = (time.perf_counter() - start) * 1000.0
    return good, elapsed

with concurrent.futures.ThreadPoolExecutor(max_workers=conc) as ex:
    pending = set()
    while time.perf_counter() < deadline or pending:
        while time.perf_counter() < deadline and len(pending) < conc:
            pending.add(ex.submit(one))
        done, pending = concurrent.futures.wait(
            pending, timeout=0.05, return_when=concurrent.futures.FIRST_COMPLETED
        )
        for fut in done:
            good, elapsed = fut.result()
            if good:
                ok += 1
                lat.append(elapsed)
            else:
                fail += 1

elapsed = seconds
lat.sort()
def pct(p):
    if not lat:
        return 0.0
    idx = min(len(lat) - 1, int((len(lat) - 1) * p))
    return lat[idx]
rps = ok / elapsed if elapsed > 0 else 0.0
print(f"SCORE {label} rps={rps:.1f} ok={ok} fail={fail} p50_ms={pct(0.50):.2f} p99_ms={pct(0.99):.2f}")
PY
}

wait_ready origin "http://127.0.0.1:${ORIGIN_PORT}/"
wait_ready leba "http://127.0.0.1:${LEBA_PORT}/"
wait_ready nginx "http://127.0.0.1:${NGINX_PORT}/"
wait_ready haproxy "http://127.0.0.1:${HAPROXY_PORT}/"

echo "== proxy matrix ${SEC}s concurrency=${CONC} =="
echo "origin=127.0.0.1:${ORIGIN_PORT} leba=${LEBA_PORT} nginx=${NGINX_PORT} haproxy=${HAPROXY_PORT}"

bench_one leba "http://127.0.0.1:${LEBA_PORT}/" | tee "$TMP/leba.score"
bench_one nginx "http://127.0.0.1:${NGINX_PORT}/" | tee "$TMP/nginx.score"
bench_one haproxy "http://127.0.0.1:${HAPROXY_PORT}/" | tee "$TMP/haproxy.score"

LEBA_RSS="$(rss_kib "$LEBA_PID")"
NGINX_MEM="$(docker_mem "$NGINX_NAME")"
HAPROXY_MEM="$(docker_mem "$HAPROXY_NAME")"
echo "RSS/MEM leba_kib=${LEBA_RSS:-unknown} nginx=${NGINX_MEM:-unknown} haproxy=${HAPROXY_MEM:-unknown}"

leba_rps="$(awk -F'rps=' '/SCORE/ {split($2,a," "); print a[1]}' "$TMP/leba.score")"
nginx_rps="$(awk -F'rps=' '/SCORE/ {split($2,a," "); print a[1]}' "$TMP/nginx.score")"
haproxy_rps="$(awk -F'rps=' '/SCORE/ {split($2,a," "); print a[1]}' "$TMP/haproxy.score")"

python3 - "$REQUIRE_WIN" "$leba_rps" "$nginx_rps" "$haproxy_rps" <<'PY'
import sys
require, leba, nginx, haproxy = sys.argv[1], *(float(x) for x in sys.argv[2:5])
print(f"SUMMARY leba_vs_nginx={leba / nginx if nginx else 0:.2f}x leba_vs_haproxy={leba / haproxy if haproxy else 0:.2f}x")
if require == "1" and not (leba > nginx and leba > haproxy):
    raise SystemExit("FAIL: LEBA_REQUIRE_WIN=1 but Leba did not beat both nginx and HAProxy")
PY

echo "PASS"
