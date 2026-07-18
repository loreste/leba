#!/bin/sh
# keepalived track script: 0 = healthy
# Prefer public ready probe; fall back to stats livez if configured locally.
set -eu
if curl -fsS --max-time 1 http://127.0.0.1:80/readyz >/dev/null 2>&1; then
  exit 0
fi
if curl -fsS --max-time 1 http://127.0.0.1:8404/readyz >/dev/null 2>&1; then
  exit 0
fi
exit 1
