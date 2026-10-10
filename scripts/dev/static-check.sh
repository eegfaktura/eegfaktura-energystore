#!/usr/bin/env bash
# gofmt and go vet over the tested packages. Fails on any finding that is not one of the known,
# recorded ones below; those are production code and stay until their own fix (no source change
# in the test-environment milestones, open-points ES-24).
#   known-errors #47: store/ebow/pool.go is not gofmt-clean (comment alignment)
#   known-errors #11: excel/ExcelSource.go:558 unreachable code,
#                     calculation/energy.go:85 unkeyed fields in a store.Engine literal
set -euo pipefail
cd "$(dirname "$0")/../.."

status=0

known_fmt='^store/ebow/pool\.go$'
fmt_out=$(gofmt -l $(git ls-files '*.go') | grep -vE "$known_fmt" || true)
if [ -n "$fmt_out" ]; then
  echo "gofmt: files not formatted:"; echo "$fmt_out"; status=1
fi

known_vet='excel/ExcelSource\.go:558:[0-9]+: unreachable code|calculation/energy\.go:85:[0-9]+: at\.ourproject/energystore/store\.Engine struct literal uses unkeyed fields'
# shellcheck disable=SC2046
vet_out=$(go vet $(bash scripts/dev/packages.sh) 2>&1 | grep -v '^#' | grep -vE "$known_vet" || true)
if [ -n "$vet_out" ]; then
  echo "go vet: new findings:"; echo "$vet_out"; status=1
fi

[ $status -eq 0 ] && echo "static-check: ok (known findings #11, #47 allowed)"
exit $status
