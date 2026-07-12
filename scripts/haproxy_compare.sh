#!/usr/bin/env bash
# Compare Leba behavior against HAProxy running in Docker.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

MAKO="${MAKO_BIN:-/Users/loreste/mako/target/release/mako}"
[[ -x "$MAKO" ]] || MAKO="$(command -v mako)"

HAPROXY_IMAGE="${HAPROXY_IMAGE:-haproxy:2.9-alpine}"
HAPROXY_NAME="${HAPROXY_NAME:-leba-haproxy-compare}"
LEBA_BIN="${LEBA_BIN:-/tmp/leba_compare}"
ORIGIN_BIN="${ORIGIN_BIN:-/tmp/leba_compare_origin}"
TCP_ORIGIN_BIN="${TCP_ORIGIN_BIN:-/tmp/leba_compare_tcp_origin}"
LEBA_CFG="${LEBA_CFG:-/tmp/leba_compare.conf}"
BENCH_REQUESTS="${BENCH_REQUESTS:-80}"

cleanup() {
  docker rm -f "$HAPROXY_NAME" >/dev/null 2>&1 || true
  kill ${LEBA_PID:-} ${P1:-} ${P2:-} ${P3:-} ${P4:-} ${P5:-} ${P6:-} 2>/dev/null || true
}
trap cleanup EXIT

for port in 13306 13307 18080 18081 18404 19001 19002 19011 19012 19021 19100; do
  for p in $(lsof -ti :"$port" 2>/dev/null || true); do
    kill "$p" 2>/dev/null || true
  done
done
docker rm -f "$HAPROXY_NAME" >/dev/null 2>&1 || true

echo "== build leba + demo origins =="
"$MAKO" build main.mko -o "$LEBA_BIN"
"$MAKO" build examples/demo_backend.mko -o "$ORIGIN_BIN"
"$MAKO" build examples/tcp_echo.mko -o "$TCP_ORIGIN_BIN"

cp configs/leba.conf "$LEBA_CFG"
cat >>"$LEBA_CFG" <<'EOF'

frontend db
  bind 13306
  mode tcp
  default_backend db

backend db
  balance least_conn
  connect_timeout 3s
  server db1 127.0.0.1:19100 weight 100 maxconn 500 no_check
EOF

echo "== start demo origins =="
"$ORIGIN_BIN" 19001 api1 >/tmp/leba_cmp_api1.log 2>&1 &
P1=$!
"$ORIGIN_BIN" 19002 api2 >/tmp/leba_cmp_api2.log 2>&1 &
P2=$!
"$ORIGIN_BIN" 19011 web1 >/tmp/leba_cmp_web1.log 2>&1 &
P3=$!
"$ORIGIN_BIN" 19012 web2 >/tmp/leba_cmp_web2.log 2>&1 &
P4=$!
"$ORIGIN_BIN" 19021 static1 >/tmp/leba_cmp_static1.log 2>&1 &
P5=$!
"$TCP_ORIGIN_BIN" 19100 db1 >/tmp/leba_cmp_db1.log 2>&1 &
P6=$!
sleep 0.4

echo "== start leba =="
"$LEBA_BIN" -f "$LEBA_CFG" -n 260 >/tmp/leba_cmp_leba.log 2>&1 &
LEBA_PID=$!
sleep 0.5

echo "== start haproxy docker =="
docker run -d --name "$HAPROXY_NAME" \
  -p 18081:8080 \
  -p 13307:3306 \
  --add-host=host.docker.internal:host-gateway \
  -v "$ROOT/configs/haproxy-compare.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro" \
  "$HAPROXY_IMAGE" >/tmp/leba_cmp_haproxy.cid
sleep 1.0

check_contains() {
  local label="$1"
  local body="$2"
  local needle="$3"
  if ! grep -q "$needle" "$body"; then
    echo "FAIL $label expected $needle"
    cat "$body"
    exit 1
  fi
}

bench_http() {
  local label="$1"
  local url="$2"
  python3 - "$label" "$url" "$BENCH_REQUESTS" <<'PY'
import sys
import time
import urllib.request

label, url, raw_n = sys.argv[1], sys.argv[2], sys.argv[3]
n = int(raw_n)
ok = 0
start = time.perf_counter()
for _ in range(n):
    try:
        with urllib.request.urlopen(url, timeout=3) as resp:
            body = resp.read()
            if resp.status == 200 and body:
                ok += 1
    except Exception:
        pass
elapsed = time.perf_counter() - start
rps = ok / elapsed if elapsed > 0 else 0.0
print(f"bench {label}: requests={ok}/{n} seconds={elapsed:.3f} req_per_sec={rps:.1f}")
if ok * 100 < n * 95:
    raise SystemExit(1)
PY
}

compare_route() {
  local path="$1"
  local class="$2"
  curl -sS "http://127.0.0.1:18080$path" >"/tmp/leba_cmp_leba_${class}.json"
  curl -sS "http://127.0.0.1:18081$path" >"/tmp/leba_cmp_haproxy_${class}.json"
  check_contains "leba $path" "/tmp/leba_cmp_leba_${class}.json" "\"server\":\"$class"
  check_contains "haproxy $path" "/tmp/leba_cmp_haproxy_${class}.json" "\"server\":\"$class"
}

echo "== compare routes =="
compare_route /api/hello api
compare_route /static/app.js static
compare_route / web

echo "== compare deny =="
leba_code=$(curl -sS -o /tmp/leba_cmp_leba_deny.txt -w "%{http_code}" http://127.0.0.1:18080/admin/secret)
haproxy_code=$(curl -sS -o /tmp/leba_cmp_haproxy_deny.txt -w "%{http_code}" http://127.0.0.1:18081/admin/secret)
echo "deny leba=$leba_code haproxy=$haproxy_code"
[[ "$leba_code" == "403" ]]
[[ "$haproxy_code" == "403" ]]

echo "== compare health =="
curl -sS http://127.0.0.1:18080/api/health-probe >/tmp/leba_cmp_leba_health.json
curl -sS http://127.0.0.1:18081/api/health-probe >/tmp/leba_cmp_haproxy_health.json
check_contains "leba health route" /tmp/leba_cmp_leba_health.json '"server":"api'
check_contains "haproxy health route" /tmp/leba_cmp_haproxy_health.json '"server":"api'

echo "== compare tcp database =="
printf 'ping' | nc 127.0.0.1 13306 >/tmp/leba_cmp_leba_tcp.txt
printf 'pong' | nc 127.0.0.1 13307 >/tmp/leba_cmp_haproxy_tcp.txt
check_contains "leba tcp db" /tmp/leba_cmp_leba_tcp.txt 'db1:ping'
check_contains "haproxy tcp db" /tmp/leba_cmp_haproxy_tcp.txt 'db1:pong'

echo "== serial req/s sample =="
bench_http leba http://127.0.0.1:18080/api/bench
bench_http haproxy http://127.0.0.1:18081/api/bench

echo "HAPROXY COMPARE OK"
