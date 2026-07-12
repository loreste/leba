#!/usr/bin/env bash
# End-to-end smoke: demo backends + leba proxy + curl checks
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

MAKO_BIN="${MAKO_BIN:-}"
if [[ -z "$MAKO_BIN" ]]; then
  if [[ -x /Users/loreste/mako/target/release/mako ]]; then
    MAKO_BIN=/Users/loreste/mako/target/release/mako
  else
    MAKO_BIN="$(command -v mako)"
  fi
fi

echo "== using $MAKO_BIN =="
"$MAKO_BIN" version

echo "== unit tests =="
"$MAKO_BIN" test . -q || "$MAKO_BIN" test .

echo "== config check =="
"$MAKO_BIN" build main.mko -o /tmp/leba
/tmp/leba check configs/leba.conf

echo "== start demo backends =="
"$MAKO_BIN" build examples/demo_backend.mko -o /tmp/leba_origin
/tmp/leba_origin 19001 api1 &
P1=$!
/tmp/leba_origin 19002 api2 &
P2=$!
/tmp/leba_origin 19011 web1 &
P3=$!
/tmp/leba_origin 19012 web2 &
P4=$!
/tmp/leba_origin 19021 static1 &
P5=$!
cleanup() {
  kill $P1 $P2 $P3 $P4 $P5 $LEBA_PID 2>/dev/null || true
}
trap cleanup EXIT
sleep 0.3

echo "== start leba (max 40 requests) =="
/tmp/leba -f configs/leba.conf -n 40 &
LEBA_PID=$!
sleep 0.4

echo "== proxy requests =="
curl -sS "http://127.0.0.1:18080/api/hello" | tee /tmp/leba_api.json
echo
curl -sS "http://127.0.0.1:18080/" | tee /tmp/leba_web.json
echo
curl -sS "http://127.0.0.1:18080/static/app.js" | tee /tmp/leba_static.json
echo
code=$(curl -sS -o /tmp/leba_deny.txt -w "%{http_code}" "http://127.0.0.1:18080/admin/secret")
echo "deny status=$code"
test "$code" = "403"

echo "== stats =="
curl -sS "http://127.0.0.1:18404/health"
echo
curl -sS -u admin:leba "http://127.0.0.1:18404/stats" | head -c 400
echo

echo "SMOKE OK"
