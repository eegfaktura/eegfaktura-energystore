# M1 — Cheap unit tests

**Concept:** phase 1 · **Status:** done 2026-10-03 · **Production code:** none
**Depends on:** M0 (test support, time zone, CI). **Effort:** 1.5 – 2 days.

## Goal

Fast tests (no network, milliseconds; a real Badger only for `writeMeta`) for the helpers every number passes through, and
visible defect tests for the findings that live in them.

## Scope

| Target | Tests | Findings |
|---|---|---|
| `utils/timeUtils.go` | row id ↔ time in `Europe/Vienna`: 2026-03-29 01:45 → 03:00, 2026-10-25 02:xx twice, 31 Dec → 1 Jan, 2028-02-29; `ConvertUnixTimeToRowId` with assertions (today none) | T2, #8 |
| `utils/counterpoint.go` | `DetermineDirection` with short ids | F31 |
| `mqttclient.SplitEnergyByDay` | blocks over 28.–31.03. and 24.–27.10., end at and not at midnight: no value lost, blocks on local days | F27 |
| `mqttclient.decodeMessage` | valid CR_MSG (builder), bad base64, truncated gzip, invalid JSON → nil, no panic | F4 (input side) |
| `store/aggregate_function.go` | `calcQoV` truth table (fills the empty `Test_calcQoV`), `parseArgument` incl. `""`; real cases for the three one-case skeletons `TestAggregate_Handle*` | F15, F31, #8 |
| `store/function/reset` | the empty table filled with real cases | #8 |
| `model/QuotaMatrix.go`, `sourcemodel.go` | unknown name, zero quota, more than three producers; `MakeRawSourceLine` sizes | F31 |
| `excel` parsing | header detection, `returnFloat("1,5")`, missing and unknown direction row | F12, F13 |
| `store/ebow/codec` | JSON and key round trips (`codec/key` 33 % today); `msgp` is unused — no test, removal noted for M4 | — |
| `store/ebow.writeMeta` | a failing `Set` (store opened read-only or closed, in `t.TempDir()`) is returned, not lost | F30 (needs a small real Badger) |

Defect tests assert the **correct** behaviour and are `t.Skip("known-errors #NN")` (ES-10); each is run
once without the skip and its red result recorded in `AGENT_LOG.md` before the commit.

Where the correct behaviour is a business question (QoV meaning, ES-15), the test records today's
behaviour with a comment naming the open point — it is not a defect test and not a blessing.

## Out of scope

Anything that needs a store (M3), HTTP (M2), concurrency (M5). Extracting inline closures into named
functions (M4).

## Tasks

- [x] Tests per row above, table-driven, `InDelta` for floats
- [x] Defect tests skipped with their number, each run red once
- [x] Floors in `coverage-floors.txt` raised to the new clean values
- [x] `known-errors.md` (reproduced / not reproduced per finding), `AGENT_LOG.md`

## Acceptance criteria

- The new M1 tests (`go test -race -count=1 -run '<M1 test names>'` over `./utils/ ./model/ ./store/ ./store/function/ ./store/ebow/... ./excel/ ./mqttclient/`) are green in under 10 s; the packages as a whole are green.
- `grep -rn 'TODO: Add test cases' --include=*_test.go store/ utils/ model/` prints nothing (`services` follows in M3); no new test without an assertion.
- `utils` ≥ 85 %, `store/function` ≥ 70 %, `codec/key` and `codec/json` ≥ 80 % statements (targets, confirmed by measurement).
- Every finding of the table has a test or a recorded reason why not.

## Risks

- `SplitEnergyByDay` turns out correct for every realistic `End` → F27 closed as "not a defect" with the test kept.
- Floats: sums over 96 values differ in the last digits by order → tolerance, never exact equality.

## Result (2026-10-03)

- New test files: `utils/rowid_test.go`, `mqttclient/split_test.go`, `model/edge_cases_test.go`,
  `excel/parse_test.go`, `store/ebow/codec/key/key_test.go`, `store/ebow/codec/json/json_test.go`,
  `store/ebow/writemeta_test.go`; rewritten: the three `TestAggregate_Handle*`, `Test_calcQoV`,
  `store/function/reset_test.go`, `utils/generator_test.go`.
- Defect tests (skipped, each run red once): #27, #28, #42 (two), #45, #46 (four), #50, #51, #52 (the last three
  new). `calcQoV` is pinned as today's rule with ES-15 named (not a blessing).
- F27 is a defect only when the blocks are checked: values are lost only when `End` is not at local midnight.
- Full suite with `-race` green (`TZ=UTC` and `TZ=Europe/Vienna`); coverage 45.2 → **48.8 %**: `utils` 88.9 %,
  `store/function` 78.5 %, `codec/key` 89.7 %, `codec/json` 100 % (targets met); floors raised.
- `excel` parsing of a whole sheet (header detection, Metercode header missing) is left to M3 S9, where
  the fixture is generated with excelize.
