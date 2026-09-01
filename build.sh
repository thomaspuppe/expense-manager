#!/usr/bin/env bash
# Cross-compile a single static Linux binary for the VPS (KTD1, KTD6).
# The pure-Go SQLite driver means CGO_ENABLED=0 yields a fully self-contained
# artifact you can scp and run — no C toolchain, no shared libs.
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w" -o dist/expensemanager .

echo "built dist/expensemanager ($(du -h dist/expensemanager | cut -f1))"
