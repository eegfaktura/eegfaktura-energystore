# Concept: improving the test environment and coverage of the energystore

**Date:** 2026-10-02 · **Branch:** `improve-test-environment` · **Kind:** concept, no code change

This document describes how the test environment of eegfaktura-energystore (v1) is repaired and its
coverage raised systematically. It is modelled on eegfaktura-billing's
`docs/improve-testing-environment/` and changes **no code**. Defects found during the analysis are
listed in section 4 and **not fixed**; they are in `known-errors.md` (#16 – #46, F*n* = #(*n* + 15)).

**Decisions:** none taken yet. Every question is in `open-points.md` (ES-1 … ES-23); the README of this
folder says which milestone needs which. One point should not wait for the milestones: the critical
security defect F1 (#16, details not published) — ES-13 proposes to fix it at once in its own change.

## 1. Goals

1. The repository builds and **all** tests run from a fresh checkout with one command, locally and in
   CI, deterministically (no hang, no leftovers, no dependence on the host's time zone).
2. Every computed number — allocation, quarter-hour slots, daily/period sums, quality flags, the Excel
   export — is covered by tests that turn red as soon as it changes, **including the DST days**.
3. Tenant isolation is tested for **every** REST route and both GraphQL fields.
4. The MQTT ingest is tested for malformed, duplicate, late and failing messages without a broker.
5. Tests run with the race detector on every change in CI; coverage is measured and must not drop.
6. Known defects are first made visible by a test, then fixed in a separate change (AGENTS.md §10.1).

## 2. Starting point (measured 2026-10-02)

Go 1.26.0 locally (`go.mod` 1.25.0), protobuf code generated first (`known-errors.md` #1), one run of
`go test -count=1 -timeout 180s -coverpkg=./... ./...` over all packages except the root, then the
calculation package once more alone (it had hung). Commands: section 10.1.

### 2.1 Tests

| Package | Test funcs | Result | Remark |
|---|---:|---|---|
| `excel` | 12 (+ 3 benchmarks) | green, 6.4 s | xlsx fixtures; writes Badger data into `test/rawdata` |
| `store` | 15 | green | four GoLand skeletons: `Test_calcQoV` empty, three with one `NoError` case (#8) |
| `store/ebow` | 9 | green; **red under `-race`** (3 data races) | `TestOpenMaxObject` (#20); Badger in the source tree |
| `utils` | 12 | green | `time.Local`; one without an assertion |
| `store/function`, `services` | 1 + 1 | green | both are empty tables (`TestReset`, `TestGetLastEnergyEntry`, #8) |
| `calculation` | 1 (4 subtests) | **hung 180 s in 1 of 5 runs**, else green in 13 s | Badger `Close` (#3); pins a defect (F8) |
| `model` | 9 | **red** | `mqtt_test.go` stale wire format (#4) |
| `mqttclient` | 5 | **red** | `TestMassImport` needs a missing file (#5); `dispatcher_test` empty |
| `test` | 1 | **red** | writes to `../../../rawdata` (#6) |
| `rest`, `graph`, `middleware`, `cmd`, `config`, root | 0 | no tests | `rest` and the root cannot run a test without Keycloak (`middleware`'s `init()`, #12); `graph` does not import `middleware` and is testable today; root has three `main` (#1) |
| **Total** | **66** (+3) in 10 packages | 3 packages red (4 under `-race`), 1 flaky hang | |

CI runs only `utils`, `store`, `store/function`, `excel` (#2).

### 2.2 Coverage

Statement coverage with Go's built-in tool (`-coverpkg=./...`, union of both runs; covered by any test).
Re-measured in review pass 2 with §10.1: every package figure identical; the hand-written total was
mis-added (4 069 instead of 4 096 statements) and is corrected below.

| Package | Statements | Covered |
|---|---:|---:|
| `excel` | 567 / 793 | 71.5 % |
| `store/ebow` | 385 / 546 | 70.5 % |
| `model` | 131 / 203 | 64.5 % |
| `utils` | 113 / 190 | 59.5 % |
| `calculation` | 144 / 257 | 56.0 % |
| `store` | 317 / 698 | 45.4 % |
| `mqttclient` | 69 / 209 | 33.0 % |
| `store/function` | 8 / 65 | 12.3 % |
| `rest`, `middleware`, `graph`, `services`, `cmd`, `config` | 0 / 1 001 | **0 %** |
| `store/ebow/codec/msgp` (unused codec) | 0 / 41 | 0 % |
| `mocks`, `codec/key`, `codec/json`, `graph/model`, `test` | 40 / 93 | 43.0 % |
| **Hand-written code** | **1 774 / 4 096** | **43.3 %** |
| All incl. generated (`graph/generated`, `protoc`) | 1 791 / 6 038 | 29.7 % |

The root package (`server.go`, …) is not measured (#1). **Coverage is a signpost, not a quality
measure:** the well-covered `excel` and `calculation` print more than they assert, and the
calculation test pins a defect (F8). Everything a caller or an attacker reaches first — HTTP, tokens,
tenants, GraphQL — has **no test at all**.

### 2.3 CI

`docker-image.yml` runs on pushes to `master`, `main`, `preview/**`, `env/**` and `v*` tags and on pull requests
to `master`/`main`: protoc (unpinned, #10), the four-package test step, then the image build (pushed only on
push events) and the deploy dispatch. No `-race`, no coverage, no dependency scan (`snyk.yml`
is SAST with `continue-on-error`, #13).

## 3. Weaknesses of the existing tests

| # | Weakness | Consequence |
|---|---|---|
| T1 | Badger data written into the source tree (`../test/rawdata`, `store/ebow/te*`) with wrong or commented clean-up (#7) | runs depend on earlier runs; leftovers in the working tree |
| T2 | No fixed time zone; dates in `time.Local`; epochs valid only in CET (#9) | a UTC runner computes other row ids; DST is never tested |
| T3 | Tests that print instead of asserting (`fmt.Printf`, `println` in calculation, excel, mqttclient) | count as coverage, protect nothing |
| T4 | Vacuous tests: six GoLand skeletons (three empty tables), empty `dispatcher_test`, no-assert tests (#8) | CI's green packages assert less than they seem to |
| T5 | Stale or missing fixtures: `mqtt_test` wire format (#4), `energy-mass-test-data.json` (#5), `../../../rawdata` (#6) | three packages permanently red, so CI excludes them |
| T6 | A test pins a defect: 26 half-year buckets, the 27-bucket assertions commented out (F8) | the bug is "protected" |
| T7 | Exact float equality on kWh sums | breaks on a harmless reordering of additions |
| T8 | `calculation` only reachable with a real Badger: it takes `*ebow.BowStorage`, not the `IBowStorage` interface the mock implements | slow, disk-bound tests; the hang (#3) |
| T9 | `rest`, `middleware` and the root cannot run a test without Keycloak (#12); `/query` with `GQLProtect` exists only in `server.go` | 0 % on the whole HTTP layer |
| T10 | No test for MQTT failure paths, DST, quarterly/yearly reports, iterator errors, concurrent access | the high findings F3–F7 went unnoticed |
| T11 | The root package does not compile as a package (#1) | `go test ./...` fails before it starts |
| T12 | Commented-out tests (`datasetCalc_test.go`, the 27-bucket asserts, `TestMain` in `ExcelSource_test.go`) | gaps are known but not tracked |

## 4. Defects found (recorded only, not fixed)

"Read" = traced in the code; "suspicion" = plausible but not proven by a run; "reproduced" = shown by a
run. Details and line numbers: `known-errors.md`.

| # | KE | Severity | Finding | Status | Test that shows it |
|---|---|---|---|---|---|
| F1 | #16 | **critical** | security relevant — details not published | — | M2 |
| F2 | #17 | high | `lastRecordDate` writes 404 then 200 (no `return`) | read (v3 #15) | M2 |
| F3 | #18 | high | security relevant — details not published | — | M5 |
| F4 | #19 | high | auto-ack before store; store errors → success and `cr_msg_history` | read | M5: failing store → no history, error |
| F5 | #20 | high | security relevant — details not published | — | M5 |
| F6 | #21 | high | security relevant — details not published | — | M5 |
| F7 | #22 | high | gap fill mixes UTC/local: shifted fill rows, 4 phantom L0 slots on the spring day | read | M3: raw/export over 2026-03-29 → 92 unique slots |
| F8 | #23 | high | quarter/half-year buckets merge the first two weeks; mod 52 vs 53 | read; pinned by a test | M3 after ES-14 |
| F9 | #24 | high | Excel export panics for a meter without data | read | M3 |
| F10 | #25 | medium | autumn fold: 96 keys instead of 100 | read (v3 #18) | M3: 2026-10-25 → 100 slots |
| F11 | #26 | medium | intra-day hours shifted by one | read (v3 #17) | M3 |
| F12 | #27 | medium | Excel import panics on an unknown direction | read | M1/M3 fixture |
| F13 | #28 | medium | Excel import: QoV always 1, bad cells → 0, missing header → success | read | M3 fixtures |
| F14 | #29 | medium | Excel import double-counts TF values in the autumn hour | read | M3 fixture |
| F15 | #30 | medium | load-curve QoV never 1; reset to 0 | read | M1 truth table (ES-15) |
| F16 | #31 | medium | iterator errors ignored; iterators/txns and Badger leak on errors | read | M3 with a corrupt row |
| F17 | #32 | medium | security relevant — details not published | — | M5 |
| F18 | #33 | medium | lost update on `cpmeta/0` under concurrent imports | suspicion | M5 |
| F19 | #34 | medium | raw-data delete outside one transaction | suspicion | M5 |
| F20 | #35 | medium | late message of worse quality overwrites | read | M3 after ES-15 |
| F21 | #36 | medium | any valid `ecId` creates a directory; pool never pruned | read | M2/M3 |
| F22 | #37 | low | GET reports: bad `start` → 1970; no range check | read | M2 |
| F23 | #38 | low | `/query/rawdata`: empty targets, tenant-wide meters, body logged | read | M2 |
| F24 | #39 | low | Basic auth: wrong base64 alphabet, `:` in passwords, grant per request | read | M2 |
| F25 | #40 | low | error text to client, `InsecureSkipVerify`, CORS `*`+credentials, introspection, payload logging | read | M2 (where testable) |
| F26 | #41 | low | `wg.Add` inside the goroutine; MQTT not disconnected on close | read | M5 (`-race`) |
| F27 | #42 | low | `SplitEnergyByDay` steps by 24 h across DST; drops the last block when `End` is not at local midnight | read | M1 |
| F28 | #43 | low | aggregate cache sized by count, written by line length: a `SourceIdx` gap panics | suspicion | M3 |
| F29 | #44 | low | rounding consumers vs producers differs | read | none until ES-16 |
| F30 | #45 | low | `writeMeta` loses the `Set` error | read | M1 |
| F31 | #46 | low | dead or wrong helper code (legacy import, `QuotaMatrix`, `parseArgument("")`, …) | read | M1 for what stays; the rest removed in M4 |

## 5. Obstacles to testing

| Obstacle | Where | Workaround without code change | Solution with code change |
|---|---|---|---|
| `init()` runs two OIDC discoveries at package load and panics on failure; `TestMain` runs after it | `middleware/authentication.go:50-121` | a fake OIDC server on a loopback port started by the test script **before** `go test` (discovery document whose `issuer` equals its URL, JWKS, token endpoint), an absolute `KEYCLOAK_CONFIG` with `api` and `app` entries pointing at it; tokens through the `VerifyTokenClaims` hook set in `TestMain` | explicit `InitKeycloak()` from `main` (ES-13, M2) |
| Three `main` in the root package | `server.go`, `estore.go`, `initQoV.go` | exclude `.` from `./...` | move two to `cmd/…` (ES-22, M0) |
| Generated code not committed | `protoc/` | generate in the test script | commit it, CI checks it (ES-2, M0) |
| Global configuration through Viper (`persistence.path`, `mqtt.*`) | `store/ebow`, `mqttclient` | `viper.Set` per test with `t.TempDir()`, reset in `t.Cleanup`; no `t.Parallel` in those packages | pass the base path in (M4) |
| Global pool and lock state (`turns`, the pool singleton) | `store/ebow` | open/close per test, `-count=1` | injectable opener (M4) |
| `calculation` needs the concrete Badger type | `calculation/*.go` | real Badger in `t.TempDir()` | functions on `IBowStorage` (M4), then the existing mock works |
| Time not injectable (`time.Now()` in intraday, engine) | `store/intraday_function.go`, `engine_test.go` | fixed dates in the past, `Europe/Vienna` set in `TestMain` | a clock parameter (M4) |
| Bucket and split rules are inline closures | `EEGCalculationV2.go`, `SplitEnergyByDay` | test through the public function with a small data set | named pure functions (M4) |
| Importer and dispatcher take the concrete `*MQTTStreamer` | `mqttclient/mqttClient.go` | its `client` field is Paho's `mqtt.Client` interface: an in-package test builds `&MQTTStreamer{client: fake}` and records `AddRoute`/`Publish` (`cr_msg_history`); fake `mqtt.Message`s into dispatcher and importer | an interface for publish (M4) |
| gRPC clients dial a Viper address inside the function (`grpc.Dial`) | `services/apiService.go:17` (`/query/rawdata` without meters), `utils/admin.go:16` (export mail) | a gRPC test server on `127.0.0.1:0`, `viper.Set` the address; `bufconn` does not work (no dialer can be passed in) | inject the connection (M4) |

## 6. Measures in phases

Each phase is its own change with its own review. Phases 0 – 3, 5, 6 change no production logic; the
only production edits are enabling moves, each a decision: in M0 the root split (ES-22), committed
generated code (ES-2) and the mechanical `gofmt`/vet fixes (ES-11); in M2 `InitKeycloak()` (ES-13).

### Phase 0 — Foundation (make it run, everywhere)

- Reproducible build: `scripts/dev/generate.sh` with pinned `protoc` and plugin versions; generated code
  committed and checked in CI (ES-2); the root package split (ES-22); a format-only commit and the two
  vet fixes (#11, #47) so `gofmt -l .` and `go vet ./...` print nothing (ES-11).
- Test hygiene: every Badger path under `t.TempDir()` (#7), the global pool closed in `t.Cleanup`
  (`ebow.ClosePool()`); `TestMain` per package sets `time.Local = Europe/Vienna`, a guard test fails
  otherwise (#9); per-package `-timeout` and a goroutine dump on timeout (#3); the hang diagnosed once
  (first suspect: `excel` and `calculation` share `test/rawdata/excelsource*` and run in parallel).
- Triage: `model/mqtt_test.go` updated to today's wire format (#4); `TestMassImport` gets a small
  generated fixture or `t.Skip("known-errors #5")`; `store/ebow.TestOpenMaxObject` (red under `-race`)
  skipped with `known-errors #20`; the 26-bucket assertion (F8) removed, not moved (the correct count
  waits for ES-14); `rawstore_test.go` and `datasetCalc_test.go` deleted (ES-10, #6); the six
  skeletons (#8) stay until M1/M3 fill them.
- Shared test support package `internal/testsupport`: temp store, raw-line and `cpmeta` builders, a
  CR_MSG builder (base64 + gzip like eda-xp), DST date constants, float tolerance helper.
- CI: one job `go test -race -count=1 -timeout 300s ./...` (`calculation` alone takes 143 s under
  `-race`) plus `gofmt -l` and `go vet`, on pull requests and pushes, gating the image (ES-11); coverage
  profile, `scripts/dev/coverage-check.sh` with per-package floors that may only rise (ES-9), HTML report
  as artefact; `govulncheck` reporting (ES-12); new actions pinned by SHA (ES-3).

### Phase 1 — Cheap unit tests (no network; Badger only for `writeMeta`)

| Target | What is checked |
|---|---|
| `utils/timeUtils` | row id ↔ time in `Europe/Vienna`, both DST days, year end, leap day |
| `mqttclient.SplitEnergyByDay`, `decodeMessage` | day blocks across DST (F27); base64/gzip errors |
| `store/aggregate_function` | `calcQoV` truth table (F15), `parseArgument` incl. `""` (F31) |
| `model` | `QuotaMatrix` edge cases, `MakeRawSourceLine` sizes (F31), the MQTT JSON contract (from M0) |
| `excel` header and cell parsing | direction row (F12), `returnFloat` with `"1,5"` (F13) |
| codecs | JSON and key round trips (`msgp` is unused: test it only if it stays, else remove in M4) |
| `store/ebow.writeMeta` | error propagation (F30) |

### Phase 2 — HTTP layer and tenant matrix

`httptest` against the real router (`rest.NewRestServer()` plus `/query` wired as in `server.go`) with the
real middleware; once `init()` has passed (ES-13, or the fake OIDC server of §5), tokens go through
`VerifyTokenClaims` (claims built in the test, no signature needed); Badger in `t.TempDir()`. Matrix for **every** entry point — 13 `ProtectApp` routes, 2 `ProtectApi` routes,
2 GraphQL fields — where the case applies:

| Case | Expectation |
|---|---|
| no token / malformed `Authorization` | 403 (today's contract, pinned); `ProtectApi` with a non-Basic scheme: 400 |
| invalid token / failed password grant | 401 (`ProtectApp`, `GQLProtect`); 403 (`ProtectApi`) |
| own tenant | 200 |
| foreign tenant in the header | 403 (`ProtectApp`, `ProtectApi`), 401 (`GQLProtect` today — differs, recorded) |
| missing tenant header | 403 |
| `superuser` with any tenant | 200 (`ProtectApp`, `GQLProtect`); `ProtectApi` has no `superuser` bypass: 403 |
| GraphQL tenant isolation (#16) | refused — **F1**, details not published |
| invalid `ecId` (`a-b`, `a.b`, 40 characters) | an error status (today 400 or 500 by route), no directory created |
| unknown but valid `ecId` | today a directory is created (F21) |
| invalid body / query | 400 (F22) |
| `rawdata/delete` without `superuser` | 403 |

Plus F2, F23, F24 and the error bodies. `ProtectApi` needs a token endpoint for the password grant. Today
the API client and its token URL come from the discovery in `init()` (unexported `kcClientAPI`), so the
endpoint must be served by the fake OIDC server of §5; with ES-13, `InitKeycloak` can point it at an
`httptest.Server`.

### Phase 3 — Storage, import and report scenarios (real Badger, `t.TempDir()`)

One scenario matrix; each scenario imports a small data set (MQTT path and Excel path) and checks the
raw rows, `cpmeta/0`, the report JSON and the export workbook (read back with excelize):

| # | Scenario | Covers |
|---|---|---|
| S1 | one consumer, one producer, one normal day | base case: 96 slots, sums, allocation |
| S2 | spring-forward day 2026-03-29 | 92 slots, no phantom gap (**F7**) |
| S3 | fall-back day 2026-10-25 | 100 slots (**F10**), Excel TF values (**F14**) |
| S4 | gap of one hour in the data | fill rows at the right ids (**F7**) |
| S5 | month and year boundary, leap day 2028-02-29 | period ranges |
| S6 | quarterly and half-year report | buckets (**F8**, after ES-14) |
| S7 | quality L1/L2/L3 mixed, late message of worse quality | QoV in reports and export (**F15**, **F20**) |
| S8 | metering point requested without data | export (**F9**) |
| S9 | Excel import with bad direction, text cells, missing header | **F12**, **F13** |
| S10 | raw-data delete of a range, then a report | delete path |
| S11 | corrupt stored row | error instead of a truncated report (**F16**) |
| S12 | intra-day and load-curve report over one day | **F11**, **F15** |

Expected values come from a hand-computed table per scenario (an independent oracle, not the code's
own output). Optionally the same data sets from eegfaktura-v3's `energy-mock` (ES-21).

### Phase 4 — Refactoring for testability (production code, own decision)

**Status: blocked and optional until M0 – M3 are done (ES-18).**
Clock parameter; storage opener and base path injected; `calculation` on `IBowStorage`; bucket and day
split as named pure functions; importer returns errors, manual MQTT ack (ES-17); pool rewritten (F5, F6); `restServer.go` split per route group (ES-6); dead code removed (F31).
Effort: section 7.

### Phase 5 — Concurrency and MQTT robustness

All under `-race`: the defect tests of F3, F5, F6 and F17 (scenarios not published); importer with a
failing store (F4); start/stop under load (F26); two concurrent imports of new metering points
(F18); delete and import of the same day (F19). Fake `mqtt.Message`s, no broker (ES-12).

### Phase 6 — Contracts with the neighbours

- **HTTP callers:** the requests eegfaktura-web (`energy.service.ts`, `graphql-query.ts`) and eegfaktura-v3
  (`EnergyStoreClient.kt`, `EnergyStoreWire.kt`) send, as fixtures with the source commit id; one test per
  call that the route exists and the body is accepted; response snapshots of the fields they read (ES-21).
- **MQTT input:** a CR_MSG as eda-xp and v3's `energy-mock` publish it (topic, base64/gzip, JSON) →
  imported correctly; the `cr_msg_history` reply.
- **gRPC:** `protoc/masterdata.proto` compared with the backend's copy.
- Reusable for energystore-v2: the same fixtures against v2 would show the cutover gaps (ES-19).

## 7. Effort

AI-assisted work in this repository; **review and decisions by the maintainer are extra**.

| Milestone | Content | Effort |
|---|---|---|
| M0 | build, hygiene, triage, test support, CI, coverage gate | 2 – 3 days |
| M1 | unit tests | 1.5 – 2 days |
| M2 | HTTP matrix (17 entry points), `InitKeycloak` move | 2 – 2.5 days |
| M3 | scenarios S1 – S12 with oracle tables and fixtures | 3 – 4 days |
| M5 | concurrency and MQTT | 1.5 – 2 days |
| M6 | contracts | 1.5 – 2 days |
| **M0 – M3, M5, M6** | | **11.5 – 15.5 days** |
| M4 | refactoring (blocked, optional) | 4.5 – 6.5 days (sum of its steps 4a – 4f) |

Risks: the hang (#3) may need a Badger option or upgrade (a decision); `-race` makes `calculation` slow
(143 s), so M0 may need a smaller fixture for it; business answers ES-14/15/16
gate parts of M3; fixing F1 first (ES-13) is outside this effort (half a day).

### 7.1 What cannot be done without phase 4

| Item | Without M4 | Consequence |
|---|---|---|
| Fast calculation tests | only with a real Badger per test | seconds instead of milliseconds |
| Bucket and day-split rules alone | through the public functions | coarse failure messages |
| MQTT ack semantics (F4) | testable only as "history published on failure" | the ack itself stays untested |
| Pool fixes (F5, F6) | tests exist and are skipped | the races stay in production |
| Time-dependent intraday/engine code | fixed past dates | "today" paths untested |

## 8. Targets and metrics

| Metric | today | after M0 | after M2 | after M3 | after M5/M6 |
|---|---:|---:|---:|---:|---:|
| Packages run in CI | 4 | all | all | all | all |
| Red / hanging packages | 3 (4 with `-race`) / 1 | 0 / 0 | 0 / 0 | 0 / 0 | 0 / 0 |
| Hand-written statements | 43.3 % | ≥ 45 % | ≥ 60 % | ≥ 70 % | ≥ 75 % |
| `rest` + `middleware` + `graph` | 0 % | 0 % | ≥ 80 % | ≥ 80 % | ≥ 80 % |
| Entry points with the tenant matrix | 0 of 17 | 0 | 17 | 17 | 17 |
| DST scenarios (S2, S3) | 0 | 0 | 0 | 2 | 2 |
| Tests under `-race` in CI | no | yes | yes | yes | yes |

The percentages are estimates — **targets to be confirmed by measurement** at the end of each milestone;
a target is lowered with a reason, never met by excluding code. A test without an assertion does not
count (T3, T4). Defect tests are `t.Skip("known-errors #NN")` until the fix (ES-10).

## 9. New tools and licences

The service is AGPL-3.0 (AGENTS.md §12). Proposed, all build/test only, nothing in the image:

| Tool | Licence | Decision |
|---|---|---|
| Go coverage (`go test -cover`, `go tool cover`) | BSD-3-Clause, part of Go | ES-9 (no new source) |
| `golang.org/x/vuln/cmd/govulncheck` | BSD-3-Clause | ES-12 |
| `mochi-mqtt/server` (embedded broker), only if fakes do not suffice | MIT | ES-12 (assumption: not needed) |

Exact versions are fixed (≥ 7 days old) and added to `EXTERNAL_SOURCES.md` in the change that uses them.

## 10. Appendix

### 10.1 Reproduce the measurement

```bash
export PATH="$HOME/go/bin:$HOME/.local/opt/protoc/bin:$PATH"   # protoc 29.3, protoc-gen-go 1.36.11, -go-grpc 1.6.2
protoc --experimental_allow_proto3_optional=true --proto_path=. --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative ./protoc/*.proto
PKGS=$(go list ./... | grep -v '^at.ourproject/energystore$')
go test -count=1 -timeout 120s -skip TestCalculateBiAnnualParticipantReport \
  -coverpkg=./... -coverprofile=cover.out $PKGS
go test -count=1 -timeout 90s -coverpkg=./... -coverprofile=cover-calc.out ./calculation/
go test -count=1 -race ./store/ebow/                                # the three races of F5
git checkout -- protoc/ && rm protoc/masterdata*.pb.go              # leave the tree as it was
rm -rf test/rawdata/excelsource                                     # the calculation test's leftover (#7)
```

Union of the two profiles per block, generated code (`graph/generated`, `protoc`) counted separately.

### 10.2 Sources

Code review of all packages on 2026-10-02 (two independent passes, each finding checked against the
code); the test runs above; `eegfaktura-analyze-it/state-of-testing.md` and `state-of-patches.md`
(2026-09-01); eegfaktura-v3 `known-errors.md` #14 – #19, #51; billing's concept as the template.
