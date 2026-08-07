#!/usr/bin/env bash
# Fair-ish local HTTP reverse-proxy microbench: Leba vs nginx.
# Not a lab-grade capacity study — regression + directional signal for RPS/latency/RSS.
#
# Usage:
#   make build
#   ./scripts/bench_vs_nginx.sh [seconds] [concurrency]
#
# Requires: nginx, curl. Optional: PATH includes /opt/homebrew/opt/nginx/bin.
# Prefer nginx static origin (fast). Raise nofile for high concurrency:
#   ulimit -n 65536
set -euo pipefail
# High-concurrency benches need more than the common macOS soft limit of 256.
ulimit -n 65536 2>/dev/null || true
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LEBA="${ROOT}/leba"
SEC="${1:-8}"
CONC="${2:-40}"
TMP="$(mktemp -d /tmp/leba-bench-XXXXXX)"
# Ephemeral ports avoid collisions with leftover smoke processes.
ORIGIN_PORT="${ORIGIN_PORT:-$((19100 + ($$ % 80)))}"
LEBA_PORT="${LEBA_PORT:-$((19200 + ($$ % 80)))}"
STATS_PORT="${STATS_PORT:-$((19300 + ($$ % 80)))}"
NGINX_PORT="${NGINX_PORT:-$((19400 + ($$ % 80)))}"

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
  if [[ -x /opt/homebrew/opt/nginx/bin/nginx ]]; then
    export PATH="/opt/homebrew/opt/nginx/bin:$PATH"
  else
    echo "nginx not found in PATH — install nginx for comparison" >&2
    exit 1
  fi
fi

rss_kb() {
  local pid="$1"
  ps -o rss= -p "$pid" 2>/dev/null | tr -d ' ' || echo 0
}

# Fast static origin (nginx). Python ThreadingHTTPServer collapses under wrk load.
ORIGIN_PREFIX="${TMP}/origin"
mkdir -p "$ORIGIN_PREFIX"
cat >"$ORIGIN_PREFIX/nginx.conf" <<EOF
worker_processes 2;
error_log ${ORIGIN_PREFIX}/error.log crit;
pid ${ORIGIN_PREFIX}/nginx.pid;
events { worker_connections 16384; multi_accept on; }
http {
  access_log off;
  keepalive_timeout 0;
  server {
    listen 127.0.0.1:${ORIGIN_PORT};
    location / { default_type text/plain; return 200 'ok\n'; }
  }
}
EOF
nginx -c "$ORIGIN_PREFIX/nginx.conf" -p "$ORIGIN_PREFIX" -g "daemon off;" >"$ORIGIN_PREFIX/out.log" 2>&1 &
ORIGIN_PID=$!
for _ in $(seq 1 50); do
  if curl -s -o /dev/null --max-time 0.2 "http://127.0.0.1:${ORIGIN_PORT}/"; then
    break
  fi
  sleep 0.05
done

cat >"$TMP/leba.conf" <<EOF
defaults
  timeout_client 5s
  timeout_server 5s
  timeout_connect 2s
  workers 64
  retries 0
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
  server o1 127.0.0.1:${ORIGIN_PORT} weight 100 no_check
EOF

PROXY_PREFIX="${TMP}/proxy"
mkdir -p "$PROXY_PREFIX"
cat >"$PROXY_PREFIX/nginx.conf" <<EOF
worker_processes 1;
error_log ${PROXY_PREFIX}/error.log crit;
pid ${PROXY_PREFIX}/nginx.pid;
events { worker_connections 16384; multi_accept on; }
http {
  access_log off;
  keepalive_timeout 0;
  server {
    listen 127.0.0.1:${NGINX_PORT};
    location / {
      proxy_http_version 1.1;
      proxy_set_header Connection close;
      proxy_pass http://127.0.0.1:${ORIGIN_PORT};
    }
  }
}
EOF

"$LEBA" -f "$TMP/leba.conf" >"$TMP/leba.log" 2>&1 &
LEBA_PID=$!
nginx -c "$PROXY_PREFIX/nginx.conf" -p "$PROXY_PREFIX" -g "daemon off;" >"$PROXY_PREFIX/out.log" 2>&1 &
NGINX_PID=$!
sleep 0.6

for url in "http://127.0.0.1:${LEBA_PORT}/" "http://127.0.0.1:${NGINX_PORT}/"; do
  ok=0
  for _ in $(seq 1 60); do
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 1 "$url" || true)
    if [[ "$code" == "200" ]]; then ok=1; break; fi
    sleep 0.05
  done
  if [[ "$ok" != "1" ]]; then
    echo "FAIL: not ready $url" >&2
    tail -30 "$TMP/leba.log" "$TMP/nginx.out" "$TMP/nginx-error.log" 2>/dev/null || true
    exit 1
  fi
done

run_python_bench() {
  local label="$1"
  local url="$2"
  python3 - "$label" "$url" "$SEC" "$CONC" <<'PY'
import sys, time, urllib.request, concurrent.futures
label, url, sec_s, conc_s = sys.argv[1:5]
sec, conc = float(sec_s), int(conc_s)
end = time.perf_counter() + sec
lat = []
ok = fail = 0

def one(_):
    global ok, fail
    t0 = time.perf_counter()
    try:
        req = urllib.request.Request(url, headers={"Connection": "close"})
        with urllib.request.urlopen(req, timeout=3) as r:
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
    while time.perf_counter() < end:
        if len(futs) < conc * 4:
            futs.append(ex.submit(one, 0))
        else:
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
# machine line for SCORECARD parse
print(f"SCORE {label} rps={rps:.1f} ok={ok} fail={fail} p50_ms={pct(0.50):.2f} p99_ms={pct(0.99):.2f}")
PY
}

echo "== microbench ${SEC}s conc=${CONC} (Connection:close, tiny body) =="
echo "origin: 127.0.0.1:${ORIGIN_PORT}  leba:${LEBA_PORT}  nginx:${NGINX_PORT}"
echo "workers: leba=8  nginx=1  (fair local signal; not production capacity)"
LEBA_RSS_BEFORE=$(rss_kb "$LEBA_PID")
NGINX_RSS_BEFORE=$(rss_kb "$NGINX_PID")

run_python_bench "leba" "http://127.0.0.1:${LEBA_PORT}/" | tee "$TMP/leba_bench.txt"
run_python_bench "nginx" "http://127.0.0.1:${NGINX_PORT}/" | tee "$TMP/nginx_bench.txt"

LEBA_RSS=$(rss_kb "$LEBA_PID")
NGINX_RSS=$(rss_kb "$NGINX_PID")
echo ""
echo "RSS (ps, KiB after load): leba=${LEBA_RSS}  nginx=${NGINX_RSS}  (before leba=${LEBA_RSS_BEFORE} nginx=${NGINX_RSS_BEFORE})"
echo "SCORE rss leba_kib=${LEBA_RSS} nginx_kib=${NGINX_RSS}"
echo ""
echo "Notes:"
echo "  - Same origin, close after each request, 1 nginx worker vs 8 Leba crew workers."
echo "  - Directional only; publish multi-run medians before production superiority claims."
echo "  - HA: make test-ha-peers (dual-node smoke). Multi-hour VIP soak still site-specific."
echo "PASS"
