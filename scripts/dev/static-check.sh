#!/usr/bin/env bash
# gofmt and go vet over the tested packages. Fails on any finding (the recorded ones, #11 and #47,
# were fixed upstream in platform#181).
set -euo pipefail
cd "$(dirname "$0")/../.."

status=0

fmt_out=$(gofmt -l $(git ls-files '*.go') || true)
if [ -n "$fmt_out" ]; then
  echo "gofmt: files not formatted:"; echo "$fmt_out"; status=1
fi

# shellcheck disable=SC2046
vet_out=$(go vet $(bash scripts/dev/packages.sh) 2>&1 | grep -v '^#' || true)
if [ -n "$vet_out" ]; then
  echo "go vet: findings:"; echo "$vet_out"; status=1
fi

[ $status -eq 0 ] && echo "static-check: ok"
exit $status
