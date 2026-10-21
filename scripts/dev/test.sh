#!/usr/bin/env bash
# The whole test suite in one command (M0): race detector, no cache, per-package timeout,
# coverage profile over all hand-written packages.
#
#   bash scripts/dev/test.sh                 # all packages, writes cover.out
#   bash scripts/dev/test.sh -run TestName   # extra arguments go to `go test`
#
# The root package (server.go, estore.go, initQoV.go: three func main) is left out: it does not
# compile as one package (known-errors #1). Moving the two extra entry points to cmd/ is
# a source change and was not done.
# Timeout 600 s: calculation alone takes 220-270 s under -race in CI (upstream CI runs of platform#181).
set -euo pipefail
cd "$(dirname "$0")/../.."

pkgs=$(bash scripts/dev/packages.sh)
coverpkg=$(echo "$pkgs" | paste -sd, -)

# shellcheck disable=SC2086
go test -race -count=1 -timeout 600s -coverpkg="$coverpkg" -coverprofile=cover.out "$@" $pkgs
