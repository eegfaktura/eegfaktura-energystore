#!/usr/bin/env bash
# Lines per file against the size limits of AGENTS.md section 13.
# Hand-written Go files only: generated code (*.pb.go, graph/generated/, *_gen.go) is excluded.
# Usage: bash scripts/dev/loc-check.sh [--all]   (default: only files above 300 lines)
set -euo pipefail
cd "$(dirname "$0")/../.."
all=${1:-}
find . -type f -name '*.go' ! -name '*.pb.go' ! -name '*_gen.go' ! -name '*_gen_test.go' \
  ! -path './graph/generated/*' -print0 |
  xargs -0 wc -l | grep -v ' total$' | sort -rn |
  while read -r lines file; do
    if   [ "$lines" -gt 600 ]; then status=blocked
    elif [ "$lines" -gt 450 ]; then status=red
    elif [ "$lines" -gt 300 ]; then status=yellow
    else status=green; fi
    if [ "$status" != green ] || [ "$all" = --all ]; then
      printf '%-8s %5d  %s\n' "$status" "$lines" "${file#./}"
    fi
  done
