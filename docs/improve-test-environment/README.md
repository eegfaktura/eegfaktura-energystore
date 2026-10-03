# Test-environment milestones

Milestones of [test-coverage-concept.md](test-coverage-concept.md), one file each. The numbers equal
the phases of the concept. Modelled on eegfaktura-billing's `docs/improve-testing-environment/`.
Order: M0 → M1 → M2 → M3 → M5 → M6 (M1, M2 and M3 only need M0 and may run in parallel; M5 needs
M3's import helpers; M6 needs M2's router helper and M5's fake MQTT client); M4 is **blocked and optional** (ES-18). Effort in total
(M0 – M3, M5, M6): about 11.5 – 15.5 working days AI-assisted, maintainer review extra; M4 adds 4.5 – 6.5.

**Before any milestone:** the critical security defect F1 (`known-errors.md` #16, details not published) should be fixed
in its own change (ES-13) — it does not need the test environment.

| Milestone | Title | Production code changed? | Needs | Status |
|---|---|---|---|---|
| [M0](m00-foundation.md) | Foundation: build, hygiene, triage, CI, coverage | build layout (ES-22), generated code (ES-2), `gofmt`/vet fixes (ES-11), CI | ES-2, ES-3, ES-9 – ES-12, ES-22 | **done 2026-10-03** (without the production moves, ES-24) |
| [M1](m01-unit-tests.md) | Cheap unit tests | no | M0 | **done 2026-10-03** |
| [M2](m02-http-layer.md) | HTTP layer, GraphQL, tenant matrix (17 entry points) | `InitKeycloak()` only (ES-13); none with the fallback script | M0, ES-13, ES-20 | **done 2026-10-03** (in-process fake Keycloak, no production change) |
| [M3](m03-storage-scenarios.md) | Storage, import and report scenarios S1 – S12 | no | M0; ES-7; S6 waits for ES-14, S7 for ES-15 | **done 2026-10-03** (11 of 12, S6 waits for ES-14) |
| [M4](m04-testability-refactoring.md) | Refactoring for testability | yes | M0 – M3, M5; ES-17, ES-18 | **blocked, optional** — not started: production code, excluded by ES-24 |
| [M5](m05-concurrency-and-mqtt.md) | Concurrency and MQTT robustness | no | M0, M3 helpers; ES-12 | **done 2026-10-03** |
| [M6](m06-contracts.md) | Contracts with web, v3, eda-xp, backend | no | M0, M2 router helper, M5 fake MQTT client; ES-21 | **done 2026-10-03** |

## Where each item of the concept lives

| Concept item | Milestone | Note |
|---|---|---|
| F1 (#16) | own change first (ES-13); test in M2 | critical |
| F2, F21 – F25 (#17, #36 – #40) | M2 | F25 partly not testable (CORS, introspection: recorded) |
| F3 – F6, F17 – F19, F26 (#18 – #21, #32 – #34, #41) | M5 | F5 already visible with `-race` (M0's CI) |
| F7 – F11, F13 – F16, F20, F28 (#22 – #26, #28 – #31, #35, #43) | M3 | F8 after ES-14, F15/F20 after ES-15 |
| F12, F15, F27, F30, F31 (#27, #30, #42, #45, #46) | M1 | F12 also in M3 (S9) |
| F29 (#44) | none until ES-16 | business question |
| T1, T2, T4, T5, T6, T11, T12 | M0 | |
| T3, T7 | M0 (helpers), then every milestone | |
| T8, T9, T10 | M4 / M2 / M3 + M5 | |
| S1 – S12 | M3 | S6 after ES-14, S7 after ES-15 |
| #1 – #13 (environment) | M0, except #12 (M2), #10 partly (ES-3), #13 Snyk/Dependabot (ES-4); #8 filled in M1 (`store`, `store/function`) and M3 (`services`) | |
| #14, #15 | operator questions ES-20, ES-23 | not test work |
| #47 (`gofmt`), #48 (request context) | M0 (#47); #48 with the next middleware change | |

**State 2026-10-03:** the user's goal allows no change to production code (`open-points.md` ES-24):
the enabling moves (ES-22, ES-13, the #11/#47 fixes) and M4 are not done; M2 uses the fallback.
Critical security findings are kept outside the repository (`known-errors.md` note).

## Decisions needed

All are in `open-points.md`. Status 2026-10-02: none decided yet; the assumptions are the defaults.

| Id | Question | Needed before | Recommended default |
|---|---|---|---|
| ES-13 | Fix F1 first; `InitKeycloak()` instead of `init()` | now / M2 | yes / yes |
| ES-2 | Commit generated protobuf code, check it in CI | M0 | yes |
| ES-22 | Move `estore.go`, `initQoV.go` to `cmd/` | M0 | yes |
| ES-9 | Built-in coverage + floors script | M0 | yes |
| ES-10 | `t.Skip("known-errors #NN")` for defect tests; delete the assertion-free tests | M0 | yes / yes |
| ES-11 | CI `go test -race ./...` gating the image; `gofmt`/vet checks | M0 | yes |
| ES-12 | `govulncheck` yes; no MQTT broker library | M0 / M5 | yes / no |
| ES-3 | Pin tools and new actions | M0 | yes |
| ES-7 | Describe today's storage format once | M3 | yes |
| ES-14 | Quarter/half-year bucket rule | M3 S6 | — (business) |
| ES-15 | QoV meaning and overwrite rule | M3 S7 | — (business) |
| ES-16 | Rounding consumers vs producers | — | no test pins it |
| ES-17 | Manual MQTT ack after store | M4 | proposed |
| ES-18 | Go for M4 | M4 | not now |
| ES-19 | Invest in v1 tests vs v2 cutover | all | v1 is production |
| ES-20, ES-23 | Production claim shape; #15 (details not published) | M2 (nested case) | operator |
| ES-21 | Fixture source and copy rule for M6 | M6 | as billing B-16 |

## Rules that apply to every milestone

- **No production change in M0 – M3, M5, M6** except the named enabling moves (ES-2, ES-22 and the
  mechanical `gofmt`/vet fixes of ES-11 in M0; ES-13 in M2), CI and test code. A defect found by a test is recorded in `known-errors.md` and fixed in
  its own change (AGENTS.md §10.1). A test that would bless wrong behaviour is not written; it is added
  as a defect test with `t.Skip("known-errors #NN")` as the first statement, asserting the correct
  behaviour. Check: `grep -rnE 't\.Skip(f|Now)?\(' --include=*_test.go . | grep -v 'known-errors #[0-9]'` prints
  nothing. Each defect test is run once without the skip and its red result recorded before the commit.
- **Order inside a milestone:** build everything first, write the tests once at the end of the step,
  run the full suite once at the end (AGENTS.md §10.3). While iterating: one package or `-run`.
- **Determinism:** Badger only in `t.TempDir()`; `Europe/Vienna`; no `time.Now()` in assertions; no
  sleeps; no network, Keycloak or broker; floats with tolerance.
- **Race detector:** the full suite with `-race` once per milestone; a race in production code is a
  `known-errors.md` row and a skipped test, never a test-side workaround.
- **Dependencies:** none expected; any tool at an exact version, ≥ 7 days old, licence read from its own
  `LICENSE`, row in `EXTERNAL_SOURCES.md` in the same change (AGENTS.md §12).
- **Size limits:** no new file above 300 lines if avoidable, never above 450 (AGENTS.md §13); check with
  `bash scripts/dev/loc-check.sh`. Split test files by area.
- **Measure clean:** `go test -count=1` (no cache) for every figure in a milestone report; coverage from
  the profile of the full clean run.
- **No benchmarks or mass imports** (`ExportBench_test.go`, `TestMassImport` with real volumes) unless the user says so.
- **Tracking:** every milestone updates `AGENT_LOG.md`; defects go to `known-errors.md`; questions to
  `open-points.md`. Each milestone is its own change with its own review and is committed when green.

## Review log

Pass 1: 2026-10-02 (author's self-check before the first review)
- Every finding F1 – F31 was traced in the code at `20e0852` by one of two independent review passes;
  F1, F2, F3, F5, F7, F8, F9, F10, F11 were re-read by the author against the source; v3-sourced rows
  (#14, #17, #18, #25, #26, #36) re-located (line numbers updated where the code moved: `restServer.go:188-192`,
  `importFunctions.go:151-154`, `rowdata.go:113`); v3 #16 is partly fixed by 20e0852 (`poolKey`), the
  remaining part is F21.
- Confirmed by a run: F5 (three data races, `go test -race ./store/ebow`), the red packages and the hang
  (#3 – #6). The hang is intermittent (1 of 5 runs), not a permanent hang as the CI comment says.
- `store/ebow` was green in every run here, although the CI comment and state-of-testing call it red or
  flaky; under `-race` it fails (F5) — that is the likely reason.
- Not verified: #15 (details not published); the production
  `tenant` claim shape (#14); F18, F19, F27, F28 (suspicions).

Review pass 1 (clean context): 2026-10-02
- Checked against the code at `07d2457`: every file:line reference and behaviour in `known-errors.md`
  #1 – #46 (= F1 – F31); route counts (13 `ProtectApp` `restServer.go:28-40`, 2 `ProtectApi`
  `energy.go:25-26`, 2 GraphQL fields); 66 test functions in 10 packages (+3 benchmarks); `docker-image.yml`
  (triggers, test step `:102-103`, floating actions); `Dockerfile`; `go.mod` direct versions (all 18 match)
  and nine licences in the module cache; the callers (web `energy.service.ts`/`base.service.ts`, v3
  `EnergyStoreClient.kt`); MQTT topics (`config.yaml`, `mqttClient.go:117`); v3 known-errors #14 – #19, #51, V3-53.
- Re-run (stubs generated, tree restored): `go vet` (only the two findings of #11 plus `main redeclared`),
  all packages except the root (`model`, `mqttclient`, `test` red as stated), `calculation` alone green
  in 13 s, `go test -race ./store/ebow` red with three races at `pool.go:72, 78, 166, 181`; regenerating
  rewrites `excel*.pb.go` (72 + / 102 −).
- Wrong and fixed: `graph` does not import `middleware` (#12, concept §2.1, T9) — only `rest` and the root
  are blocked; `init()` runs **two** discoveries (`:50-121`, not `:50-117`); #8 has six skeletons, three of
  them empty incl. `services.TestGetLastEnergyEntry` (not "five empty tables"; ES-10, M0, M1, M3 adjusted);
  the reproduced F5 races are on `DbObject.Db`, the map race is read only; #43/F28 mechanism (an
  out-of-range panic, not "longer lines"); line numbers in #2, #18, #22, #24, #27, #33, #34, #35, #40, #44, #46;
  #42/F27 upgraded from suspicion to read (the dropped block was traced); `excel*.pb.go` come from older
  generators (#1, AGENTS §9).
- m02: the status matrix now has one column per guard — `ProtectApi` answers 400 for a wrong scheme,
  403 for a failed grant, reads only `X-Tenant` and has no `superuser` bypass; `GQLProtect` 401 for a
  foreign tenant. The fallback is specified (fake OIDC server with discovery, JWKS, token endpoint;
  absolute `KEYCLOAK_CONFIG`); "set the API client's URL" was not possible (`kcClientAPI` is built in
  `init()`). `bufconn` dropped: `grpc.Dial` takes a Viper address inside the function → loopback gRPC
  server. The `../x` `ecId` case replaced (mux redirects `..` paths); invalid `ecId` gives 400 or 500, not 4xx.
- m05: the reply check does not wait for M4 — `MQTTStreamer.client` is Paho's `mqtt.Client` interface,
  so an in-package fake records `cr_msg_history`. m03 imports through `Import` (`Execute` would publish
  through a nil client).
- New suspicion for the hang (#3): `excel` and `calculation` share `test/rawdata/excelsource*` and run
  in parallel under `go test ./...`; the hang was seen only in that run. M0 and the HOWTO (`-p 1`) name it.
- Consistency: M4 effort 4.5 – 6.5 days (the sum of its steps; was 5 – 7); README order (M6 needs M2's
  router helper); DST scenarios target 2 (S2, S3); S1 – S12 and the #8 skeletons added to the mapping
  table; AGENTS.md image name, tenant validation, log references.
- Not verified: the coverage figures (not re-measured); eda-xp's payload fields (Scala, not read beyond
  the topic mapping); `state-of-testing.md`/`state-of-patches.md` quotes; the production volume; whether
  #15 (details not published).
- Unresolved: whether `ProtectApi`'s 400 and missing `superuser` bypass are intended (recorded in m02, not
  as a defect); the fallback script makes plain `go test ./rest/` panic — ES-13 is the clean way.

Review pass 2 (clean context): 2026-10-02
- Re-measured §2.2 with the commands of concept §10.1 (stubs generated, tree restored): every package
  figure identical; the hand-written total was mis-added (4 069 → **4 096** statements, 43.6 → **43.3 %**),
  a row for the small packages (`mocks`, `codec/key`, `codec/json`, `graph/model`, `test`) added.
  `go test -race ./store/ebow` again three races; `go vet` only #11 plus `main redeclared`.
- New measurement: `go test -race ./calculation/` takes **143 s** (green). M0's 120 s timeout would have
  failed it: timeout 300 s in M0, ES-11, the HOWTO and #3.
- Behaviour re-read in the code (not only the line): #16, #17, #21, #25, #26, #28, #29, #30, #31, #32,
  #36, #37, #38, #39, #41, #45 — all confirmed. Sharpened: #26 (00:00 lands in slot 23), #30 (the
  intra-day report has the same QoV start value).
- Wrong and fixed: stored values are **JSON** (`codec/json`, default of `ebow.Open`), not `msgp` — no
  caller sets a codec; the `msgp` codec is unused (AGENTS.md §2/§8, storage-format README, ES-7, M1,
  `EXTERNAL_SOURCES.md`). The `Dockerfile` does **not** generate the stubs (#1, ES-2, AGENTS §17, HOWTO).
- Plan fixes: M0 would have been red under its own acceptance — `TestOpenMaxObject` fails under `-race`
  (now skipped with #20); the F8 assertion is removed, not turned into a defect test (the correct count
  needs ES-14); the #11 vet fixes and a format-only commit contradicted "no change outside `_test.go`"
  (now named as mechanical changes under ES-11, with `gofmt`/vet in CI); `-count=20` for `calculation`
  without `-race`. M6 also needs M5's fake MQTT client. Skip check also catches `t.Skipf`/`t.SkipNow`;
  M3's scenario count is a runnable command; M2's fallback names `go-jose` (already in `go.sum`).
- AGENTS.md: request context rule, `recover` at worker boundaries, no `t.Parallel` with Viper/pool
  globals, `gofmt`/vet commands; Quick Reference marks what works only after M0. Each other command
  checked: the three `go build` lines work once stubs exist; `go vet ./...` fails on the root (#1).
- New rows: #47 (`gofmt` not clean in `store/ebow/pool*.go`, CI checks neither format nor vet), #48
  (token verification with `context.Background()`).
- Effort sums checked: 11.5 – 15.5 days (M0 – M3, M5, M6) and 4.5 – 6.5 (M4) are right. Every ES point a
  milestone names exists and has a default; no duplicates; README, concept and milestone headers agree.
- Not verified: the test-function counts of §2.1 (taken from pass 1); eda-xp's payload; the production
  volume (1.2 TB, now marked as a 2026-09 figure); #15.
- Unresolved: ES-13 bundles two decisions (F1 fix, `InitKeycloak`), as do ES-10 and ES-12 — left as is
  to keep the references stable; the `-race` run time of `calculation` may need a smaller fixture in M0.
