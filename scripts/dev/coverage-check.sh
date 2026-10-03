#!/usr/bin/env bash
# Coverage gate (open-points ES-9): statements per package from a cover profile, compared with
# the floors in scripts/dev/coverage-floors.txt. Fails when one package falls below its floor.
# Generated code (*.pb.go, graph/generated/, *_gen.go) and the test helpers (internal/testsupport) are excluded. A profile merged from several
# test binaries lists a block once per binary; a block counts as covered if any run hit it.
#
#   bash scripts/dev/coverage-check.sh cover.out            # check
#   bash scripts/dev/coverage-check.sh cover.out --print    # print the table only
set -euo pipefail
cd "$(dirname "$0")/../.."

profile=${1:-cover.out}
mode=${2:-}
floors=scripts/dev/coverage-floors.txt
module=at.ourproject/energystore

awk -v module="$module/" -v floors="$floors" -v mode="$mode" '
  BEGIN {
    while ((getline line < floors) > 0) {
      if (line ~ /^[[:space:]]*(#|$)/) continue
      split(line, f, /[[:space:]]+/); floor[f[1]] = f[2]
    }
  }
  NR == 1 { next }   # "mode: atomic"
  {
    split($1, a, ":"); file = a[1]
    if (file ~ /\.pb\.go$/ || file ~ /\/graph\/generated\// || file ~ /_gen\.go$/ || file ~ /\/internal\/testsupport\//) next
    block = $1
    stmts[block] = $2
    if ($3 > 0) hit[block] = 1
    pkg = file; sub(/\/[^\/]*$/, "", pkg); sub(module, "", pkg); sub(/^at\.ourproject\/energystore$/, ".", pkg)
    pkgOf[block] = pkg
  }
  END {
    for (b in stmts) { p = pkgOf[b]; total[p] += stmts[b]; if (b in hit) cov[p] += stmts[b]; all += stmts[b]; if (b in hit) allcov += stmts[b] }
    fail = 0
    printf "%-28s %7s %7s %6s\n", "package", "stmts", "covered", "floor"
    for (p in total) {
      pct = 100 * cov[p] / total[p]
      fl = (p in floor) ? floor[p] : "-"
      flag = ""
      if (mode != "--print" && (p in floor) && pct < floor[p]) { flag = "  BELOW FLOOR"; fail = 1 }
      printf "%-28s %7d %6.1f%% %6s%s\n", p, total[p], pct, fl, flag | "sort"
    }
    close("sort")
    printf "%-28s %7d %6.1f%%\n", "total", all, 100 * allcov / all
    for (p in floor) if (!(p in total)) { printf "%s has a floor but no statements in the profile\n", p; if (mode != "--print") fail = 1 }
    exit fail
  }
' "$profile"
