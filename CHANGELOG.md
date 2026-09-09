# Changelog

All notable changes to **eegfaktura-energystore (Go measurement/energy data store)** are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and
versioning follows the deployment release tags. Detailed diffs stay in the `git log`;
this changelog highlights the changes relevant for overview and operations.

## [Unreleased]

## [1.2.1] – 2026-09-09

### Changed
- The energy export writes its data rows with an explicit neutral row style, which cuts export
  time by 22% on a large community and 29% on a medium one (measured; see the benchmark added
  alongside). The output is unchanged.

  Why this helps: excelize resolves a style for every cell that carries none, and that
  resolution walks all column definitions of the sheet. This sheet declares widths for columns
  2..1000, and `flatCols` expands such a range into one entry per column — so each of roughly
  2.5 million cells scanned up to a thousand entries. None of those entries carries a style
  (we only set widths), so the scan always returned zero: full cost, no effect. Passing a
  non-zero `RowOpts.StyleID` makes `prepareCellStyle` return immediately, and per-cell styles
  still win afterwards, so nothing about the rendered sheet changes.

  A trap worth recording: `NewStyle(&excelize.Style{})` returns **0**, which is
  indistinguishable from "no style" and leaves the fast path unused. The style has to be a real
  one — font size 11, Excel's default, is visually neutral.

  This is mitigation, not a cure. The export is still far slower than before the excelize
  2.8.1 -> 2.11.0 upgrade in 1.2.0 (large community: 22.3s -> 17.5s, against 6.0s on 2.8.1).
  The structural fix is to decouple the export from the HTTP request — see
  `konzept-async-energy-export.md` in the eegfaktura repo.

### Added
- `excel/ExportBench_test.go` — a benchmark over the real export path (runner, summary and
  energy sheets, mocked storage) at three community sizes. It is what established the excelize
  regression and bisected it to 2.9.0/2.9.1, and it makes any future change to this path
  measurable rather than arguable.

### Security
- `google.golang.org/grpc` 1.82.1 -> 1.83.1, closing CVE-2026-84304 (HIGH): heap memory
  exhaustion through HTTP/2 DATA frame fragmentation. This is the same advisory that
  `eegfaktura-backend` closed a day earlier — energystore carried it too, since both speak
  gRPC to each other. The server is cluster-internal rather than exposed at the ingress, which
  limits who can reach it, but does not remove the exposure. (#32)

## [1.2.0] – 2026-09-07

### Security
- `github.com/xuri/excelize/v2` 2.8.1 → 2.11.0, closing CVE-2026-54063 (CVSS 7.5). Three
  minor versions in one step, because this service had fallen further behind than the others
  — and unlike the x/crypto advisories this one is on a reachable path: excelize parses the
  offline EDA spreadsheets and writes the energy exports. The `excel` package tests cover
  both directions and pass.
- `google.golang.org/grpc` 1.79.3 → 1.82.1 (GHSA-hrxh-6v49-42gf), which also pulled
  `protobuf` 1.36.10 → 1.36.11.
- `golang.org/x/crypto` 0.46.0 → 0.52.0, closing seven open advisories — CVE-2026-46595
  (CVSS 10.0) plus six rated 9.1. All of them are in `x/crypto/ssh`, which this service does
  not import (the module is an indirect dependency and there is no SSH server here), so the
  vulnerable code was never reachable — but leaving a 10.0 open is not defensible either.
  Requires Go 1.25: `x/crypto` 0.52.0 declares `go 1.25.0`, so the Dockerfile and the CI Go
  version move from 1.24 to 1.25. That version floor is why Dependabot's own bump (#22) kept
  failing at `go mod download && go mod verify` — it raised the dependency without raising
  the toolchain. Pulled along by the resolution: `x/net` 0.48.0 → 0.54.0 (direct) and the
  usual `x/sys`/`x/text`/`x/tools`/`x/mod`/`x/sync` indirects.

### Fixed
- `DateToString` rendered the seconds with `%.4d` — the year's verb, applied one argument too
  far — so every period timestamp it wrote looked like `30.12.2023 15:00:0000`. The value was
  still read correctly because the parser was lenient, but it leaked into the XLSX summary
  sheet ("Zeitraum von …:0000") and into the `lastRecordDate` REST/GraphQL response, and it
  silently ruled out moving the parser to `time.Parse`. Seconds are now two digits. Records
  already on disk keep the old shape and stay readable (see below).
- Reading period timestamps is now tolerant by intent rather than by accident: the canonical
  form, the legacy four-digit-seconds form, and EDA's offline exports without seconds
  (`31.07.2026 23:45`) are all accepted, everything else fails. Previously this rested on
  `fmt.Sscanf` happening to be forgiving — a `DateToString`/`StringToTime` round-trip test
  now pins it, which is the assertion that was missing all along.
- `updateMeta` merged instead of overwriting: a month message is split into day blocks that
  each read `cpmeta/0` once and write it back at the end, so a block could persist its own
  stale period over the wider one another block had just written. The values themselves were
  complete; the dashboard cut off at the older end date. The widening is now re-applied
  against the record as it is on disk. Reported externally in #28, which serializes the day
  blocks — this fixes the underlying lost update, so the metadata is safe even without that
  serialization.
- Offline EDA XLSX import now normalizes capitalization and spacing variants in headers and
  accepts date lines with optional seconds.
- Daily MQTT energy blocks are now stored sequentially, protecting shared `cpmeta/0` updates
  and `SourceIdx` allocation for previously unknown metering points from concurrent writes.

### Changed
- CI runs the Go tests of the reliably green packages (`utils`, `store`, `store/function`,
  `excel`) before the image build. Until now the pipeline only built the Docker image, so a
  green check meant "it compiles" — nothing more. A regression in the period-metadata logic
  (external PR #26) passed a green check although an existing test (`Test_updateMetaCP`)
  caught it locally. Deliberately not `go test ./...`: on `main` the packages `calculation`
  (hangs into the timeout), `model`, `mqttclient` and `store/ebow` are already failing
  (Badger disk usage, wire-format fixtures) — the list is meant as a lower bound and should
  grow once those are fixed.
- Repository hygiene: generated Badger test data (`test/rawdata/`, `store/ebow/te999999/`) was
  accidentally committed together with the CI change and is removed again; both paths are now
  gitignored so a `git add -A` after a test run cannot pick them up.
- MQTT import logs whether an invalid CR_MSG transport payload failed during
  Base64 decoding or gzip decompression instead of reporting only empty data.

## [1.1.0] – 2026-07-11

### Added
- Ops endpoint `POST /eeg/v2/{ecid}/rawdata/delete` to remove the raw energy data of a **single
  metering point** within a time range (maintenance for mis-assigned/duplicate data). Because one
  BadgerDB row (15-min timestamp) packs all metering points of the EC into shared arrays, deletion
  zeros only the target metering point's slot block (Consumers/Producers + QoV, resolved via the
  same `GetMetaInfo`/`cpmeta/0` mapping the read path uses) — co-located metering points in the
  same row stay untouched (core-correctness test in `store/deleteRawData_test.go`). Same iteration
  for `dryRun` (preview: affected timesteps + summed kWh, no write) and execute; batched and
  idempotent (re-zeroing is a no-op). Behind `ProtectApp` **and** an explicit `superuser` realm-role
  check in the handler — because energystore is reachable directly by user-facing clients (the web
  app calls `/eeg/v2/...` with user tokens), a tenant-scoped check alone would let any EEG-admin
  delete their own tenant's data; only superusers may delete. Each execute writes one structured log line
  (operator/tenant/ec/zp/range/timesteps). Deletion is irreversible (value 0 / QoV 0); a later EDA
  re-import repopulates the slots.

## [1.0.3] – 2026-07-06

### Changed
- CI: Preview-Deployments (ADR-0007) — Push auf `preview/**` baut+deployt on-demand in die Dev-Zone (sha-pinned, kein `:latest`), Auto-Reset bei Branch-Delete.

### Fixed
- Rawdata-delete (`POST /eeg/v2/{ecid}/rawdata/delete`) skipped the last timesteps
  of the selected range ("end not deleted"). Row-ids encode wall-clock time in the
  fold timezone (the image bakes `TZ=Europe/Berlin`), but the delete parsed them with
  `time.UTC`, shifting every timestamp by the +1h/+2h offset — so timesteps after
  local 23:00 on the last day fell past the range end and were skipped (dry-run
  undercounted identically). Now parses row-ids in `time.Local`, matching the import,
  report and Excel paths and the absolute `from`/`to` instants sent by the client.
  Regression test added.
- Raw-data query returned each timestamp multiple times for a re-registered
  metering point. When a metering point was deregistered from one member and
  re-registered under another, the old participant row remained with an
  overlapping active window, so the caller's `cps` list contained that metering
  point more than once for queries overlapping that window. The store holds
  exactly one data series per metering-point name, so each duplicate `cps` entry
  produced an identical repeated series ("each timestamp 4×"; support cases
  RC101586, RC105720). `QueryRawData` now de-duplicates the target list by
  metering-point name before querying — covering both raw endpoints
  (`/query/rawdata` and `/eeg/v2/{ecid}/raw`) and preventing the `Aggregate`
  function from double-counting. Single-metering-point queries and the Excel
  export were unaffected and remain unchanged.

## [1.0.2] – 2026-06-30

### Changed
- Hardening: close idle per-tenant Badger DBs after 15s instead of 60s; raise the keycloak token HTTP client timeout from 1s to 10s. (#16)

## [1.0.1] – 2026-06-30

### Fixed
- OOM / node-level SystemOOM under broad multi-tenant load: cap the per-tenant Badger block cache at 64 MB and the index cache at 16 MB (was the 256 MB default). (#15)

## [1.0.0] – 2026-06-28

First production release built entirely from public source.

### Changed
- CI: push to the registry's development tier with an auto-rollout bridge
  (dispatch-deploy, ADR-0005). (#7)
- Added AGPL-3.0 license; README with service overview and tech stack. (#2, #8)
