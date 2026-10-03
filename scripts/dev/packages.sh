#!/usr/bin/env bash
# Prints the import paths of all packages that are tested, vetted and measured: every package
# except the root, which holds three func main (known-errors #1, open-points ES-24).
set -euo pipefail
cd "$(dirname "$0")/../.."
go list ./... | grep -v '^at.ourproject/energystore$'
