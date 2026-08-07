#!/usr/bin/env bash
# Fair-ish local HTTP reverse-proxy microbench: Leba vs nginx (and optional HAProxy).
# Not a lab-grade capacity study — regression + directional signal for CPU/latency.
#
# Usage:
#   make build
#   ./scripts/bench_vs_nginx.sh [seconds] [concurrency]
#
# Requires: nginx (or openresty), curl, python3. Optional: hey or wrk if installed.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LEBA="${ROOT}/leba"
SEC="${1:-5}"
CONC="${2:-50}"
TMP="$(mktemp -d /tmp/leba-bench-XXXXXX)"
cleanup() {
  set +m 2>/dev/null || true
  kill ${ORIGIN_PID:-} ${LEBA_PID:-} ${NGINX_PID:-} 2>/dev/null || true
  wait ${ORIGIN_PID:-} ${LEBA_PID:-} ${NGINX_PID:-} 2>/dev/null || true
  rm -rf "$TMP"
}
trap cleanup EXIT

if [[ ! -x "$LEBA" ]]; then
  echo "build leba first: make build" >&2
  exit 1
fi
if ! command -v nginx >/dev/null 2>&1; then
  echo "nginx not found in PATH — install nginx for comparison" >&2
  exit 1
fi

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
for _ in $(seq 1 40); do
  curl -s -o /dev/null --max-time 0.2 "http://127.0.0.1:19099/" && break
  sleep 0.05
done

cat >"$TMP/leba.conf" <<EOF
defaults
  timeout_client 5s
  timeout_server 5s
  timeout_connect 2s
  workers 32
  retries 1
  maxconn 20000
frontend web
  bind 127.0.0.1:19080
  mode http
  route default -> app
frontend stats
  bind 127.0.0.1:19081
  mode stats
  auth admin:benchpass:admin
backend app
  balance round_robin
  server o1 127.0.0.1:19099 weight 100 no_check
EOF

cat >"$TMP/nginx.conf" <<EOF
worker_processes 1;
error_log ${TMP}/nginx-error.log crit;
pid ${TMP}/nginx.pid;
events { worker_connections 4096; multi_accept on; }
http {
  access_log off;
  keepalive_timeout 0;
  server {
    listen 127.0.0.1:19082;
    location / {
      proxy_http_version 1.1;
      proxy_set_header Connection close;
      proxy_pass http://127.0.0.1:19099;
    }
  }
}
EOF

"$LEBA" -f "$TMP/leba.conf" >"$TMP/leba.log" 2>&1 &
LEBA_PID=$!
nginx -c "$TMP/nginx.conf" -g "daemon off;" >"$TMP/nginx.out" 2>&1 &
NGINX_PID=$!
sleep 0.5

# Readiness
for url in "http://127.0.0.1:19080/" "http://127.0.0.1:19082/"; do
  ok=0
  for _ in $(seq 1 50); do
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 1 "$url" || true)
    if [[ "$code" == "200" ]]; then ok=1; break; fi
    sleep 0.05
  done
  if [[ "$ok" != "1" ]]; then
    echo "FAIL: not ready $url" >&2
    exit 1
  fi
done

run_python_bench() {
  local label="$1"
  local url="$2"
  python3 - "$label" "$url" "$SEC" "$CONC" <<'PY'
import sys, time, urllib.request, concurrent.futures, statistics
label, url, sec_s, conc_s = sys.argv[1:5]
sec, conc = float(sec_s), int(conc_s)
end = time.perf_counter() + sec
lat = []
ok = fail = 0
lock_end = end

def one(_):
    global ok, fail
    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(url, timeout=2) as r:
            r.read()
            if r.status == 200:
                ok += 1
            else:
                fail += 1
    except Exception:
        fail += 1
    lat.append((time.perf_counter() - t0) * 1000.0)

with concurrent.futures.ThreadPoolExecutor(max_workers=conc) as ex:
    futs = []
    while time.perf_counter() < lock_end:
        if len(futs) < conc * 4:
            futs.append(ex.submit(one, 0))
        else:
            # drain some
            concurrent.futures.wait(futs[:conc], timeout=0.01)
            futs = [f for f in futs if not f.done()]
    concurrent.futures.wait(futs)

elapsed = sec
rps = ok / elapsed if elapsed > 0 else 0
lat_sorted = sorted(lat) if lat else [0.0]
def pct(p):
    if not lat_sorted:
        return 0.0
    i = min(len(lat_sorted) - 1, int(len(lat_sorted) * p))
    return lat_sorted[i]
print(f"{label:8s}  rps={rps:8.1f}  ok={ok:6d}  fail={fail:4d}  p50={pct(0.50):6.2f}ms  p99={pct(0.99):6.2f}ms  n={len(lat)}")
PY
}

echo "== microbench ${SEC}s conc=${CONC} (Connection:close, tiny body) =="
echo "origin: 127.0.0.1:19099  leba:19080  nginx:19082"
run_python_bench "leba" "http://127.0.0.1:19080/"
run_python_bench "nginx" "http://127.0.0.1:19082/"
echo ""
echo "Notes:"
echo "  - Same origin, close after each request, 1 nginx worker for fair single-process compare."
echo "  - Leba workers=32 (crew). Tune both before claiming production superiority."
echo "  - Prefer multi-hour soak + CPU% (ps/top) + RSS for memory claims."
echo "PASS"
