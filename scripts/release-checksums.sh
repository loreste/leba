#!/usr/bin/env bash
# Generate SHA256SUMS for files in dist/ (local release helper).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="${1:-$ROOT/dist}"
cd "$DIST"
rm -f SHA256SUMS
sha256sum * 2>/dev/null | grep -v SHA256SUMS > SHA256SUMS || true
if [[ ! -s SHA256SUMS ]]; then
  echo "no files in $DIST" >&2
  exit 1
fi
echo "Wrote $DIST/SHA256SUMS"
cat SHA256SUMS
