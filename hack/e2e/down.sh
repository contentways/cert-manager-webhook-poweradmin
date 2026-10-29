#!/usr/bin/env bash
# Stops the processes started by hack/e2e/up.sh.
set -euo pipefail

E2E_DIR="${E2E_DIR:-$PWD/_e2e}"
for name in poweradmin pdns; do
  pidfile="$E2E_DIR/$name.pid"
  if [ -f "$pidfile" ]; then
    kill "$(cat "$pidfile")" 2>/dev/null || true
    rm -f "$pidfile"
  fi
done
