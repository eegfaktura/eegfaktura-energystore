# M3 — Storage, import and report scenarios S1 – S12

**Concept:** phase 3 · **Status:** done 2026-10-03 (S6 waits for ES-14) · **Production code:** none
**Depends on:** M0 (builders, `t.TempDir()`, time zone); ES-14 for S6, ES-15 for S7's expectations;
ES-7 (one page on today's storage format, written here). **Effort:** 3 – 4 days.

## Goal

The numbers the communities see — slots, sums, allocation, quality, the export workbook — are pinned by
scenarios with independently computed expectations, including both DST days.

## Design

- One package `scenario` (or `_test.go` files next to `store`), one file per area, `sNN_` test names so
  "12 scenarios" can be counted with grep.
- Each scenario: a fresh store in `t.TempDir()`; data imported **through the production paths** — the
  MQTT importer (`TenantEnergyImporter.Import` with decoded CR_MSG builder messages; `Execute` would
  publish through a nil Paho client — the fake client is M5's), and for S3/S9 the Excel importer with a fixture generated
  by excelize in the test; then the reads callers use: `QueryRawData`, the report, summary, intra-day and
  load-curve functions, and the export workbook (read back with excelize).
- Expected values from an oracle table per scenario, computed by hand in the scenario file (slot count,
  sums per meter and day, allocation by the static key) — never the code's own output copied.
- Floats with tolerance; time in `Europe/Vienna`.
- Write `docs/storage-format/0000-current-format.md` (ES-7) from what the scenarios touch: directories,
  row ids, `cpmeta/0`, the line layout, QoV arrays.

## Scenarios

| # | Scenario | Checks | Findings |
|---|---|---|---|
| S1 | 1 consumer, 1 producer, 2026-06-01 | 96 slots, daily sums, allocation, report JSON, export sheet | base |
| S2 | spring day 2026-03-29 | 92 slots, no fill rows, no L0 in the summary | **F7** |
| S3 | autumn day 2026-10-25 (MQTT and Excel, with TF columns) | 100 slots, both 02:xx hours kept, TF not double-counted | **F10**, **F14** |
| S4 | one hour missing on a normal day | 4 fill rows at the right ids, flagged L0 | **F7** |
| S5 | 31 Dec → 1 Jan, 2028-02-29, month end | period ranges and monthly report | — |
| S6 | quarterly and half-year report | bucket count and boundaries | **F8** (after ES-14) |
| S7 | L1/L2/L3 mixed; a late L2 for an L1 slot | QoV in report, load curve, export | **F15**, **F20** (ES-15) |
| S8 | export requested for a meter without data | an error or an empty column, no panic | **F9** |
| S9 | Excel: unknown direction, `"1,5"`, missing Metercode header | an error, no panic, no silent zero | **F12**, **F13** |
| S10 | raw-data delete of a range, then report and raw | deleted values gone, neighbours intact | delete path |
| S11 | one stored row corrupted on disk | the read returns an error, not a short report | **F16** |
| S12 | intra-day and load curve over S1's day | hour keys 0–23 match the slots | **F11**, **F15** |

Plus a regression row for F28 (aggregate with a non-contiguous `SourceIdx` after a meter is removed),
marked as suspicion until it fails.

## Tasks

- [x] Oracle tables and builders for S1 – S12; Excel fixtures generated in the test
- [x] Scenarios through the production import paths; reads as the callers use them
- [x] Defect scenarios skipped with their numbers, each run red once
- [x] `docs/storage-format/0000-current-format.md` (ES-7)
- [x] `services.TestGetLastEnergyEntry` (empty table, #8) filled with a store from the builders
- [x] Floors raised; `known-errors.md` (reproduced or not per finding), `AGENT_LOG.md`

## Acceptance criteria

- `grep -rhoE 'func TestS[0-9]{2}_' --include=*_test.go . | sort -u | wc -l` gives 12 (11 while ES-14 is open — no empty placeholder test).
- `store` ≥ 70 %, `calculation` ≥ 70 %, `excel` ≥ 80 % statements (targets).
- The scenario suite runs in under 60 s and leaves nothing outside `t.TempDir()`.
- `grep -rn 'TODO: Add test cases' --include=*_test.go .` prints nothing.
- Each finding of the table is either reproduced (skipped defect test, red result in `AGENT_LOG.md`) or recorded as not reproduced.

## Risks

- The oracle disagrees with the code where the business rule is unclear (ES-14/15/16) → recorded as an
  open point, the scenario asserts only what is certain.
- Import through MQTT needs the importer's Viper configuration → set per test, reset in `t.Cleanup`.

## Result (2026-10-03)

- Package `scenario/` (test files only, `doc.go`): `helpers_test.go` (oracle series, MQTT and Excel import
  through the production paths, raw reads), `s01_s05_test.go`, `s01_export_test.go`,
  `s03_s09_excel_test.go`, `s07_s12_test.go`. 11 scenario numbers (S6 waits for ES-14, no placeholder);
  the suite runs in about 2 s.
- Reproduced and skipped (each run red once): #22 (spring phantom slots at 04:00 – 04:45; gap fill
  2 h late), #24, #25, #26, #27, #28 (three Excel cases), #29, #30 (load curve and summary), #31, #43
  (F28, was a suspicion). Recorded as today's behaviour with ES-15 named: #35.
- Green and pinned: values, slots and sums of S1 in raw data, report, summary and the export workbook;
  92 stored slots on the spring day; the autumn total; a missing hour is not stored; year end, leap
  day and month reports; quarter, half-year and year totals (bucket count left to ES-14); quality per
  slot; delete of a range (end included); TF replaces the base value; intra-day and load-curve sums.
- `services.TestGetLastEnergyEntry` filled; `docs/storage-format/0000-current-format.md` written (ES-7).
- Coverage: `store` 77.8 %, `calculation` 72.8 % (targets met), `excel` 76.9 % (target 80 %: the rest
  is the legacy `ImportExcelEnergyFile`, test-only, and export branches for odd data). Total 66.4 %.
