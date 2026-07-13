#!/usr/bin/env bash
# Adversarial end-to-end smoke — expects failures to be caught
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
MAKO="${MAKO_BIN:-/Users/loreste/mako/target/release/mako}"
[[ -x "$MAKO" ]] || MAKO="$(command -v mako)"

echo "== unit tests =="
"$MAKO" test leba_core1_test.mko
"$MAKO" test leba_core2_test.mko
"$MAKO" test leba_web_test.mko

echo "== build =="
"$MAKO" build main.mko -o leba

echo "== doctor sample must pass =="
./leba doctor configs/leba.conf | tee /tmp/leba_doc.txt
grep -q "Result: PASS" /tmp/leba_doc.txt
./leba check configs/leba.conf >/tmp/leba_check.txt
grep -q "Result: PASS" /tmp/leba_check.txt

echo "== doctor bad config must fail =="
cat > /tmp/leba_bad.conf <<'EOF'
frontend web
  bind 1
  mode http
  route default -> missing
backend empty
  balance nope
EOF
if ./leba doctor /tmp/leba_bad.conf > /tmp/leba_doc_bad.txt; then
  echo "bad config unexpectedly passed"
  cat /tmp/leba_doc_bad.txt
  exit 1
fi
cat /tmp/leba_doc_bad.txt
grep -q "Result: FAIL" /tmp/leba_doc_bad.txt
if ./leba check /tmp/leba_bad.conf >/tmp/leba_check_bad.txt; then
  echo "bad config check unexpectedly passed"
  cat /tmp/leba_check_bad.txt
  exit 1
fi
hash_out=$(./leba admin hash-password leba | tr -d '\n')
[[ "$hash_out" == leba-kdf-v1:* ]]

echo "== explain deny =="
./leba explain configs/leba.conf GET /admin/secret localhost | tee /tmp/leba_ex.txt
grep -q "DENY" /tmp/leba_ex.txt

echo "== servers_file config =="
cat > /tmp/leba_servers_file.txt <<'EOF'
sf1 127.0.0.1:19001 weight 100 check
server sf2 127.0.0.1:19002 weight 100 no_check
EOF
cat > /tmp/leba_servers_file.conf <<'EOF'
frontend web
  bind 18180
  mode http
  default_backend api
frontend stats
  bind 18504
  mode stats
backend api
  balance round_robin
  servers_file /tmp/leba_servers_file.txt
EOF
./leba doctor /tmp/leba_servers_file.conf | tee /tmp/leba_servers_file_doc.txt
grep -q "Result: PASS" /tmp/leba_servers_file_doc.txt

echo "== tcp database frontends =="
TCP_BASE=$((30000 + ($$ % 10000)))
TCP_HTTP_PORT=$((TCP_BASE + 1))
TCP_STATS_PORT=$((TCP_BASE + 2))
TCP_DB_PORT=$((TCP_BASE + 3))
TCP_DB_RO_PORT=$((TCP_BASE + 4))
TCP_DB_ADMIN_PORT=$((TCP_BASE + 5))
TCP_ORIGIN_PORT=$((TCP_BASE + 6))
TCP_SIP_PORT=$((TCP_BASE + 7))
TCP_SIP_ORIGIN1_PORT=$((TCP_BASE + 8))
TCP_SIP_ORIGIN2_PORT=$((TCP_BASE + 9))
UDP_SIP_PORT=$((TCP_BASE + 10))
UDP_SIP_ORIGIN1_PORT=$((TCP_BASE + 11))
UDP_SIP_ORIGIN2_PORT=$((TCP_BASE + 12))
TLS_HTTP_PORT=$((TCP_BASE + 13))
TLS_ORIGIN_PORT=$((TCP_BASE + 14))
TLS_STATS_PORT=$((TCP_BASE + 15))
for port in "$TCP_HTTP_PORT" "$TCP_STATS_PORT" "$TCP_DB_PORT" "$TCP_DB_RO_PORT" "$TCP_DB_ADMIN_PORT" "$TCP_ORIGIN_PORT" "$TCP_SIP_PORT" "$TCP_SIP_ORIGIN1_PORT" "$TCP_SIP_ORIGIN2_PORT" "$UDP_SIP_PORT" "$UDP_SIP_ORIGIN1_PORT" "$UDP_SIP_ORIGIN2_PORT" "$TLS_HTTP_PORT" "$TLS_ORIGIN_PORT" "$TLS_STATS_PORT"; do
  for p in $(lsof -ti :$port 2>/dev/null || true); do kill "$p" 2>/dev/null || true; done
done
"$MAKO" build examples/tcp_echo.mko -o /tmp/leba_tcp_echo
"$MAKO" build examples/udp_echo.mko -o /tmp/leba_udp_echo
/tmp/leba_tcp_echo "$TCP_ORIGIN_PORT" db1 >/tmp/leba_tcp_echo.log 2>&1 &
TCP_ORIGIN_PID=$!
/tmp/leba_tcp_echo "$TCP_SIP_ORIGIN1_PORT" sip1 >/tmp/leba_sip1.log 2>&1 &
TCP_SIP1_PID=$!
/tmp/leba_tcp_echo "$TCP_SIP_ORIGIN2_PORT" sip2 >/tmp/leba_sip2.log 2>&1 &
TCP_SIP2_PID=$!
cat > /tmp/leba_tcp.conf <<EOF
frontend web
  bind $TCP_HTTP_PORT
  mode http
  default_backend web
frontend stats
  bind $TCP_STATS_PORT
  mode stats
frontend db
  bind $TCP_DB_PORT
  mode tcp
  default_backend db
frontend db_ro
  bind $TCP_DB_RO_PORT
  mode tcp
  default_backend db
frontend db_admin
  bind $TCP_DB_ADMIN_PORT
  mode tcp
  default_backend db
frontend sip_tcp
  bind $TCP_SIP_PORT
  mode tcp
  default_backend sip
backend web
  server web1 127.0.0.1:19011 weight 1 no_check
backend db
  balance round_robin
  connect_timeout 2s
  server db1 127.0.0.1:$TCP_ORIGIN_PORT weight 1 no_check
backend sip
  balance sip_call_id
  connect_timeout 2s
  server sip1 127.0.0.1:$TCP_SIP_ORIGIN1_PORT weight 1 no_check
  server sip2 127.0.0.1:$TCP_SIP_ORIGIN2_PORT weight 1 no_check
EOF
./leba doctor /tmp/leba_tcp.conf | grep -q "Result: PASS"
./leba -f /tmp/leba_tcp.conf -n 5 >/tmp/leba_tcp.log 2>&1 &
TCP_LB_PID=$!
sleep 0.5
tcp_body=$(printf 'ping' | nc 127.0.0.1 "$TCP_DB_PORT")
echo "tcp body=$tcp_body"
[[ "$tcp_body" == "db1:ping" ]]
tcp_body2=$(printf 'pong' | nc 127.0.0.1 "$TCP_DB_RO_PORT")
echo "tcp body2=$tcp_body2"
[[ "$tcp_body2" == "db1:pong" ]]
tcp_body3=$(printf 'admin' | nc 127.0.0.1 "$TCP_DB_ADMIN_PORT")
echo "tcp body3=$tcp_body3"
[[ "$tcp_body3" == "db1:admin" ]]
sip_msg=$'INVITE sip:bob@example.com SIP/2.0\r\nVia: SIP/2.0/TCP client;branch=z9hG4bK-a\r\nCall-ID: leba-call-42\r\nCSeq: 1 INVITE\r\n\r\n'
sip_body1=$(printf '%s' "$sip_msg" | nc 127.0.0.1 "$TCP_SIP_PORT")
sip_body2=$(printf '%s' "$sip_msg" | nc 127.0.0.1 "$TCP_SIP_PORT")
echo "sip body1=$sip_body1"
echo "sip body2=$sip_body2"
sip_srv1="${sip_body1%%:*}"
sip_srv2="${sip_body2%%:*}"
[[ "$sip_srv1" == "$sip_srv2" ]]
wait "$TCP_LB_PID"
kill "$TCP_ORIGIN_PID" 2>/dev/null || true
kill "$TCP_SIP1_PID" "$TCP_SIP2_PID" 2>/dev/null || true
wait "$TCP_ORIGIN_PID" 2>/dev/null || true
wait "$TCP_SIP1_PID" 2>/dev/null || true
wait "$TCP_SIP2_PID" 2>/dev/null || true
grep -q "event=listener_bound frontend=db mode=tcp bind=.*:$TCP_DB_PORT" /tmp/leba_tcp.log
grep -q "event=listener_bound frontend=db_ro mode=tcp bind=.*:$TCP_DB_RO_PORT" /tmp/leba_tcp.log
grep -q "event=listener_bound frontend=db_admin mode=tcp bind=.*:$TCP_DB_ADMIN_PORT" /tmp/leba_tcp.log
grep -q "event=listener_bound frontend=sip_tcp mode=tcp bind=.*:$TCP_SIP_PORT" /tmp/leba_tcp.log
grep -q 'TCP frontend=db' /tmp/leba_tcp.log
grep -q 'TCP frontend=db_ro' /tmp/leba_tcp.log
grep -q 'TCP frontend=db_admin' /tmp/leba_tcp.log
grep -q 'TCP frontend=sip_tcp' /tmp/leba_tcp.log

echo "== udp sip frontend =="
/tmp/leba_udp_echo "$UDP_SIP_ORIGIN1_PORT" usip1 >/tmp/leba_usip1.log 2>&1 &
UDP_SIP1_PID=$!
/tmp/leba_udp_echo "$UDP_SIP_ORIGIN2_PORT" usip2 >/tmp/leba_usip2.log 2>&1 &
UDP_SIP2_PID=$!
sleep 0.2
cat > /tmp/leba_udp_sip.conf <<EOF
frontend stats
  bind $TCP_STATS_PORT
  mode stats
frontend sip_udp
  bind $UDP_SIP_PORT
  mode udp
  default_backend sip
backend sip
  balance sip_call_id
  server sip1 127.0.0.1:$UDP_SIP_ORIGIN1_PORT weight 1 no_check
  server sip2 127.0.0.1:$UDP_SIP_ORIGIN2_PORT weight 1 no_check
EOF
./leba doctor /tmp/leba_udp_sip.conf | grep -q "Result: PASS"
./leba -f /tmp/leba_udp_sip.conf -n 4 >/tmp/leba_udp_sip.log 2>&1 &
UDP_LB_PID=$!
sleep 0.4
udp_msg=$'INVITE sip:bob@example.com SIP/2.0\r\nVia: SIP/2.0/UDP client;branch=z9hG4bK-u\r\nCall-ID: leba-udp-call-42\r\nCSeq: 1 INVITE\r\n\r\n'
udp_body1=$(printf '%s' "$udp_msg" | nc -u -w 1 127.0.0.1 "$UDP_SIP_PORT")
udp_body2=$(printf '%s' "$udp_msg" | nc -u -w 1 127.0.0.1 "$UDP_SIP_PORT")
echo "udp sip body1=$udp_body1"
echo "udp sip body2=$udp_body2"
udp_srv1="${udp_body1%%:*}"
udp_srv2="${udp_body2%%:*}"
[[ "$udp_srv1" == "$udp_srv2" ]]
[[ "$udp_srv1" == usip* ]]
wait "$UDP_LB_PID"
kill "$UDP_SIP1_PID" "$UDP_SIP2_PID" 2>/dev/null || true
wait "$UDP_SIP1_PID" 2>/dev/null || true
wait "$UDP_SIP2_PID" 2>/dev/null || true
grep -q "event=listener_bound frontend=sip_udp mode=udp bind=.*:$UDP_SIP_PORT" /tmp/leba_udp_sip.log

echo "== tls http, h2, and optional h3 frontend =="
"$MAKO" build examples/demo_backend.mko -o /tmp/leba_tls_origin
/tmp/leba_tls_origin "$TLS_ORIGIN_PORT" tlsapi >/tmp/leba_tls_origin.log 2>&1 &
TLS_ORIGIN_PID=$!
sleep 0.2
TLS_PROTOS="http/1.1,h2"
if [[ -n "${MAKO_QUICHE_ROOT:-}" ]] || [[ -d /Users/loreste/mako/runtime/third_party/quiche/target/release ]]; then
  export MAKO_QUICHE_ROOT="${MAKO_QUICHE_ROOT:-/Users/loreste/mako/runtime/third_party/quiche}"
  TLS_PROTOS="http/1.1,h2,h3"
fi
cat > /tmp/leba_tls.conf <<EOF
frontend web
  bind $TLS_HTTP_PORT
  mode http
  tls_cert /Users/loreste/mako/runtime/certs/dev.crt
  tls_key /Users/loreste/mako/runtime/certs/dev.key
  protocols $TLS_PROTOS
  default_backend api
frontend stats
  bind $TLS_STATS_PORT
  mode stats
backend api
  server api1 127.0.0.1:$TLS_ORIGIN_PORT weight 1 no_check
EOF
./leba doctor /tmp/leba_tls.conf | grep -q "Result: PASS"
# Budget for several TLS connections (curl may open parallel H2 streams per conn).
TLS_MAX=16
./leba -f /tmp/leba_tls.conf -n "$TLS_MAX" >/tmp/leba_tls.log 2>&1 &
TLS_LB_PID=$!
sleep 0.4
tls_h1_body=$(curl -sk --http1.1 --max-time 5 "https://127.0.0.1:$TLS_HTTP_PORT/api/tls-h1")
echo "tls h1 body=$tls_h1_body"
[[ "$tls_h1_body" == *'"server":"tlsapi"'* ]]
if curl -V | grep -q HTTP2; then
  tls_h2_body=$(curl -sk --http2 --max-time 5 "https://127.0.0.1:$TLS_HTTP_PORT/api/tls-h2")
  echo "tls h2 body=$tls_h2_body"
  [[ "$tls_h2_body" == *'"server":"tlsapi"'* ]]
  tls_h2_body2=$(curl -sk --http2 --max-time 5 "https://127.0.0.1:$TLS_HTTP_PORT/api/tls-h2b")
  echo "tls h2 body2=$tls_h2_body2"
  [[ "$tls_h2_body2" == *'"server":"tlsapi"'* ]]
fi
if [[ "$TLS_PROTOS" == *h3* ]] && curl -V 2>&1 | grep -qi HTTP3; then
  tls_h3_body=$(curl -sk --http3-only --max-time 5 "https://127.0.0.1:$TLS_HTTP_PORT/api/tls-h3" || true)
  echo "tls h3 body=$tls_h3_body"
  if [[ -n "$tls_h3_body" ]]; then
    [[ "$tls_h3_body" == *'"server":"tlsapi"'* ]]
  fi
elif [[ "$TLS_PROTOS" == *h3* ]]; then
  # System curl may lack HTTP/3; validate with quiche client when available.
  cat > /tmp/leba_h3_smoke_client.mko <<'H3EOF'
fn main() {
    let r = quiche_h3_get("127.0.0.1", PORT, "/api/tls-h3", "localhost", 0)
    print(r)
}
H3EOF
  sed -i.bak "s/PORT/$TLS_HTTP_PORT/" /tmp/leba_h3_smoke_client.mko
  if "$MAKO" build /tmp/leba_h3_smoke_client.mko -o /tmp/leba_h3_smoke_client >/tmp/leba_h3_build.log 2>&1; then
    tls_h3_body=$(/tmp/leba_h3_smoke_client 2>/dev/null || true)
    echo "tls h3 body=$tls_h3_body"
    [[ "$tls_h3_body" == h3:200* ]]
    [[ "$tls_h3_body" == *'"server":"tlsapi"'* ]]
  else
    echo "tls h3 skipped (quiche client build failed)"
  fi
fi
wait "$TLS_LB_PID" 2>/dev/null || true
kill "$TLS_ORIGIN_PID" 2>/dev/null || true
wait "$TLS_ORIGIN_PID" 2>/dev/null || true
grep -q "event=listener_bound frontend=web" /tmp/leba_tls.log

echo "== tcp-only database frontend =="
/tmp/leba_tcp_echo "$TCP_ORIGIN_PORT" db1 >/tmp/leba_tcp_only_db1.log 2>&1 &
TCP_ONLY_ORIGIN_PID=$!
sleep 0.2
cat > /tmp/leba_tcp_only.conf <<EOF
frontend stats
  bind $TCP_STATS_PORT
  mode stats
frontend db
  bind $TCP_DB_PORT
  mode tcp
  default_backend db
backend db
  balance least_conn
  connect_timeout 2s
  server db1 127.0.0.1:$TCP_ORIGIN_PORT weight 1 no_check
EOF
./leba doctor /tmp/leba_tcp_only.conf | grep -q "Result: PASS"
./leba -f /tmp/leba_tcp_only.conf -n 1 >/tmp/leba_tcp_only.log 2>&1 &
TCP_ONLY_LB_PID=$!
sleep 0.4
tcp_only_body=$(printf 'solo' | nc 127.0.0.1 "$TCP_DB_PORT")
echo "tcp-only body=$tcp_only_body"
[[ "$tcp_only_body" == "db1:solo" ]]
wait "$TCP_ONLY_LB_PID"
kill "$TCP_ONLY_ORIGIN_PID" 2>/dev/null || true
wait "$TCP_ONLY_ORIGIN_PID" 2>/dev/null || true
grep -q "event=listener_bound frontend=db mode=tcp bind=.*:$TCP_DB_PORT" /tmp/leba_tcp_only.log
if grep -q "reason=no_http_frontend" /tmp/leba_tcp_only.log; then
  echo "tcp-only runtime rejected missing http frontend"
  exit 1
fi

echo "== live admin + drain =="
for port in 18080 18404 19001 19002 19011 19012 19021; do
  for p in $(lsof -ti :$port 2>/dev/null || true); do kill "$p" 2>/dev/null || true; done
done
sleep 0.2
"$MAKO" build examples/demo_backend.mko -o /tmp/leba_origin
/tmp/leba_origin 19001 api1 >/dev/null 2>&1 &
/tmp/leba_origin 19002 api2 >/dev/null 2>&1 &
/tmp/leba_origin 19011 web1 >/dev/null 2>&1 &
/tmp/leba_origin 19012 web2 >/dev/null 2>&1 &
/tmp/leba_origin 19021 static1 >/dev/null 2>&1 &
sleep 0.35

# production default runs until stopped; -n is only for bounded dev/test runs
./leba -f configs/leba.conf >/tmp/leba_forever.log 2>&1 &
FPID=$!
sleep 0.6
for i in 1 2 3 4 5 6 7; do
  curl -sS http://127.0.0.1:18080/api/forever-$i >/dev/null
done
kill -0 "$FPID"
grep -q 'event=runtime_start mode=forever' /tmp/leba_forever.log
kill "$FPID" 2>/dev/null || true
wait "$FPID" 2>/dev/null || true
sleep 0.2

./leba -f configs/leba.conf -n 90 >/tmp/leba_adv.log 2>&1 &
LPID=$!
cleanup() { kill $LPID 2>/dev/null || true; for port in 19001 19002 19011 19012 19021 18080 18404; do
  for p in $(lsof -ti :$port 2>/dev/null || true); do kill "$p" 2>/dev/null || true; done; done; }
trap cleanup EXIT
sleep 0.7

# admin HTML size
unauth=$(curl -sS -o /dev/null -w "%{http_code}" http://127.0.0.1:18404/stats)
[[ "$unauth" == "401" ]]
unauth_html=$(curl -sS -o /dev/null -w "%{http_code}" http://127.0.0.1:18404/)
[[ "$unauth_html" == "401" ]]
sz=$(curl -sS -u admin:leba http://127.0.0.1:18404/ | wc -c | tr -d ' ')
echo "admin html bytes=$sz"
[[ "$sz" -gt 5000 ]]

# JSON has routes/acls
for attempt in 1 2 3 4 5; do
  curl -sS -u admin:leba http://127.0.0.1:18404/stats >/tmp/leba_stats.json || true
  if python3 -c "import sys,json;d=json.load(open('/tmp/leba_stats.json'));assert 'routes' in d and 'acls' in d and d['version']=='0.4.0'" 2>/tmp/leba_stats_err.txt; then
    break
  fi
  if [[ "$attempt" == "5" ]]; then
    cat /tmp/leba_stats_err.txt
    echo "bad stats body:"
    cat /tmp/leba_stats.json
    exit 1
  fi
  sleep 0.1
done

# CLI can target a NAT/public admin address through env defaults
LEBA_ADMIN_ADDR=http://127.0.0.1:18404/admin LEBA_ADMIN_AUTH=admin:leba ./leba admin servers | grep -q '"servers"'
LEBA_ADMIN_ADDR=127.0.0.1:18404 LEBA_ADMIN_AUTH=admin:leba ./leba admin reload-servers | grep -q '"action":"reload-servers"'
LEBA_ADMIN_ADDR=127.0.0.1:18404 LEBA_ADMIN_AUTH=viewer:leba-view ./leba admin stats | grep -q '"version":"0.4.0"'
viewer_drain=$(curl -sS -u viewer:leba-view -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:18404/admin/drain/api/api1)
[[ "$viewer_drain" == "403" ]]
for attempt in 1 2 3 4 5; do
  LEBA_ADMIN_ADDR=127.0.0.1:18404 LEBA_ADMIN_AUTH=admin:leba ./leba admin stats >/tmp/leba_cli_stats.json || true
  if python3 -c "import json;d=json.load(open('/tmp/leba_cli_stats.json'));assert d['version']=='0.4.0'" 2>/tmp/leba_cli_stats_err.txt; then
    break
  fi
  if [[ "$attempt" == "5" ]]; then
    cat /tmp/leba_cli_stats_err.txt
    echo "bad cli stats body:"
    cat /tmp/leba_cli_stats.json
    exit 1
  fi
  sleep 0.1
done

# Prometheus endpoint is scrapeable
curl -sS -u admin:leba http://127.0.0.1:18404/metrics | grep -q '^leba_requests_total '

# State-changing admin actions require POST.
get_mutation=$(curl -sS -u admin:leba -o /dev/null -w "%{http_code}" http://127.0.0.1:18404/admin/drain/api/api1)
[[ "$get_mutation" == "405" ]]
sleep 0.1
grep -q 'event=admin_audit .*user="viewer".*role="viewer".*status=403.*outcome=forbidden' /tmp/leba_adv.log
grep -q 'event=admin_audit .*user="admin".*role="admin".*status=405.*outcome=handled' /tmp/leba_adv.log

# Oversized raw HTTP requests are rejected before upstream forwarding.
too_large=$(python3 -c 'print("x"*1100000)' | curl -sS -o /dev/null -w "%{http_code}" --data-binary @- http://127.0.0.1:18080/api/too-large)
[[ "$too_large" == "413" ]]

# traceparent is correlated into access logs
TRACE_ID=4bf92f3577b34da6a3ce929d0e0e4736
TRACEPARENT="00-${TRACE_ID}-00f067aa0ba902b7-01"
curl -sS -H "traceparent: ${TRACEPARENT}" http://127.0.0.1:18080/api/trace >/tmp/leba_trace_body.json
grep -q "\"traceparent\":\"${TRACEPARENT}\"" /tmp/leba_trace_body.json
sleep 0.2
grep -q "trace=${TRACE_ID}" /tmp/leba_adv.log

# drain then traffic only to other server
LEBA_ADMIN_ADDR=127.0.0.1:18404 LEBA_ADMIN_AUTH=admin:leba ./leba admin drain api api1 >/dev/null
body=$(curl -sS http://127.0.0.1:18080/api/adv)
echo "after drain: $body"
echo "$body" | grep -q api2

# 403 deny
code=$(curl -sS -o /dev/null -w "%{http_code}" http://127.0.0.1:18080/admin/secret)
[[ "$code" == "403" ]]
sleep 0.1
grep -q 'event=request_rejected reason=acl_denied' /tmp/leba_adv.log

# readyz true
curl -sS http://127.0.0.1:18404/readyz | grep -q '"ready":true'

# drain all api servers → still may serve web; drain both api, hit api path → 502 possible
LEBA_ADMIN_ADDR=127.0.0.1:18404 LEBA_ADMIN_AUTH=admin:leba ./leba admin drain api api2 >/dev/null
code2=$(curl -sS -o /tmp/leba_502.txt -w "%{http_code}" http://127.0.0.1:18080/api/gone || true)
echo "both api drained status=$code2 body=$(cat /tmp/leba_502.txt)"
# accept 502 bad gateway when no eligible upstream
[[ "$code2" == "502" ]] || [[ "$code2" == "503" ]]
sleep 0.1
grep -q 'event=admin_action action=drain backend=api server=api1 status=200' /tmp/leba_adv.log
grep -q 'event=admin_audit .*user="admin".*role="admin".*path="/admin/drain/api/api1".*status=200' /tmp/leba_adv.log
grep -q 'event=request_rejected reason=no_server' /tmp/leba_adv.log

echo "ADVERSARIAL SMOKE OK"
