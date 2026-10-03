# Agent log

One entry per AI session, newest first. Format: date, task, changes, decisions, verification, open.

## 2026-10-03 — M6 of the test environment (contracts) and close of the goal

**Task.** M6 under the same goal; then the end of the milestone set (M4 stays blocked: production code).

**Changes.** `contract/` (doc.go, parsers, endpoint, GraphQL, request, response and proto contracts,
`testdata/` with README), `mqttclient/contract_test.go`; EXTERNAL_SOURCES (fixtures), m06 result, README.

**Decisions.** Fixtures byte-identical where the licence allows (web, v3, eda-xp: AGPL-3.0); the backend
proto as a hand-written field list (no LICENSE); the mock payload derived from `CrMessage.kt`.

**Verification.** Four throw-away changes were red with a useful message and restored; full suite with
`-race` green (`TZ=UTC`); coverage 68.2 % (45.2 % after M0, 43.6 % before); static, skip and size checks clean.

**Open (whole goal).** ES-24: the production-side items (ES-22, ES-13 with the F1 fix, #11/#47 fixes, M4)
wait for the user; ES-14 (S6), ES-15, ES-16 business answers; the critical security findings and their
private tests are outside the repository (`go test -overlay`); the unpushed commits `07d2457`, `4187df4`,
`b17a3ab` still hold their old text (squash before a push is the user's decision); nothing pushed.

## 2026-10-03 — M5 of the test environment (concurrency and MQTT)

**Task.** M5 under the same goal.

**Changes.** Fake Paho message and client in `internal/testsupport`; `mqttclient/dispatcher_test.go`,
`concurrency_test.go`; two old tests made repeatable (#54); floors; known-errors #8, #19, #33, #34, #41, #54.
Private (outside the repository): the F3, F5, F6, F17 tests and their red results.

**Decisions.** Concurrency tests use channels/latches and the fake's `Notify`; the private F6 and F17 tests
use a short wait and `Eventually` (they are not part of the suite).

**Verification.** Each defect test run red once (F18 13 of 20, F19 20 of 20); `-race -count=20` of
`mqttclient` and `store/ebow` green; full suite with `-race` green; coverage 68.0 %.

**Open.** M6.

## 2026-10-03 — M3 of the test environment (storage, import and report scenarios)

**Task.** M3 under the same goal.

**Changes.** Package `scenario/` with S1 – S5, S7 – S12 and the F28 regression; `services` test filled;
`docs/storage-format/0000-current-format.md` (ES-7); floors raised; known-errors #8, #22 – #26, #29 – #31,
#35, #43 updated (see m03 "Result").

**Decisions.** S6 is not written while ES-14 is open; the period totals are tested without the bucket count.
Today's overwrite rule (#35) is recorded with ES-15 named. None of the M3 findings is a critical security
defect (they are wrong numbers, panics in request handlers recovered by net/http, or test-only code).

**Verification.** Every defect test run red once; full suite with `-race` green; coverage 66.4 %.

**Open.** M5, then M6.

## 2026-10-03 — M2 of the test environment (HTTP layer, tenant matrix)

**Task.** M2 under the same goal (no source change, defect tests skipped, critical security findings private).

**Changes.** `internal/testsupport/fakeoidc` (in-process fake Keycloak, stdlib only), `internal/testsupport/apitest`
(router, tokens, gRPC fakes), `rest/main_test.go`, `matrix_test.go`, `handlers_test.go`, `graphql_test.go`;
floors raised; known-errors #12 mitigated, #17, #28, #36 – #40 reproduced, new #53 (monthly export by mail
panics on every call); m02 result, README, HOWTO, AGENTS.md §10.2. The F1 test is private (outside the
repository, run with `go test -overlay`), its result is recorded there.

**Decisions.** The plan's "no in-process alternative" was wrong: Go's package init order lets a test-support
package start the fake before `middleware`. Real signed tokens instead of the `VerifyTokenClaims` hook.

**Verification.** Every defect test run once without its skip (all red as recorded); the route guard checked
with a removed entry; full suite with `-race` green (one race of my own `TestMain` found and fixed);
coverage 65.2 %.

**Open.** `middleware` coverage target (dead code); M3 next.

## 2026-10-03 — M1 of the test environment (cheap unit tests)

**Task.** M1 of `docs/improve-test-environment` under the same goal (no source change, defect tests skipped).

**Changes.** Test code only (see `m01-unit-tests.md` "Result"); floors raised; known-errors #8, #9, #27, #28,
#42, #45, #46 updated, new #50 (`Reset` with an unknown meter panics), #51 (`RawSourceMeta.Copy` drops
`SourceIdx`), #52 (integer keys of the key codec do not round-trip).

**Decisions.** `"1,5"` is expected as 1.5 in the defect test of #28 (EDA's decimal comma); an unknown or
missing direction must be an error (#27). None of the new findings is security relevant.

**Verification.** Every defect test run once without its skip: all red as recorded in known-errors
(one test of mine hung on the per-tenant lock of `OpenStorageTest` — two stores of one tenant in one
test; fixed with subtests before the commit). Full suite with `-race` green twice; coverage 48.8 %.

**Open.** M2 next (HTTP layer with the fake-OIDC fallback, ES-24).

## 2026-10-03 — M0 of the test environment; critical security errors moved out of the repository

**Task.** The user's goal: implement `docs/improve-test-environment` step by step — document errors, no
fix in the source code, deactivate tests for known errors, commit each step, no push. During M0 the user
asked to move the critical, security-relevant errors out of the public files into
`critical errors/known-critical-errors.md` (outside the repository), the same for billing, with a note in
`known-errors.md`. Later the same day, at the user's request, the private list was split into one file per
repository (`known-critical-errors-energystore.md`, `known-critical-errors-billing.md`).

**Changes.** M0 (`72cdbca`, `c7d79bb`, `21b993f`): `scripts/dev/generate.sh` and the committed stubs;
`internal/testsupport` (+ `tz`); tests under `t.TempDir()`, `TestMain` with `Europe/Vienna`; triage of the
red tests (#4 – #6, #20 skipped, F8 assertion removed, #49 fixed in the test); `test.sh`, `packages.sh`,
`static-check.sh`, `coverage-check.sh` + floors; CI gate. Docs: HOWTO, AGENTS.md §2/§3/§4/§9/§17,
CHANGELOG, EXTERNAL_SOURCES, known-errors, open-points ES-24, m00 result. Security: rows #15, #16, #18,
#20, #21, #32 replaced by a "details not published" note; their details, the concept rows of F1, F3, F5,
F6, F17 and the related M2/M5 test cases are in the private file; AGENTS.md, CHANGELOG, open-points
(ES-13, ES-23), README, concept, m02, m04, m05 and the older AGENT_LOG entry neutralised.

**Decisions.** "No fix in the source code" read strictly: no production `.go` file changed — also not the
build moves the plan allowed (ES-22, #11, #47, ES-13); recorded as ES-24. Critical security rows chosen
by impact: tenant isolation, secrets, and crashes/stalls of the whole service an outside party can
trigger. Defect tests for those rows will be kept outside the repository too.

**Verification.** Baseline and after-M0 figures in `m00-foundation.md` "Result". `static-check.sh` ok;
skip check prints nothing; `loc-check.sh`: no new yellow file.

**Open.** The details are still in the local, unpushed commits `07d2457`, `4187df4`, `b17a3ab` (history
squash before a push is the user's decision); `errors-for-github.md` (untracked, not from this session)
lists them in full. Next: M1.

## 2026-10-02 — Review pass 2 of the test-environment concept (clean context)

**Task.** Second review of the docs (`07d2457`, `4187df4`): re-measure coverage, re-read a sample of
known errors in the code, check that the milestones can be executed, AGENTS.md for Go, open points.

**Changes.** Docs only: coverage total corrected (43.3 %, was mis-added), values are JSON not `msgp`, the
`Dockerfile` does not generate stubs, M0 fixed so it can meet its own acceptance (skip #20 test, F8
assertion removed, `gofmt`/vet as ES-11 mechanical changes, timeout 300 s), M6 depends on M5, new
known errors #47, #48, AGENTS.md Go rules and Quick Reference. Details: README review log, pass 2.

**Verification.** Stubs generated; §10.1 coverage runs, `go test -race ./store/ebow` (three races),
`go test -race ./calculation/` (green, 143 s), `go vet`, `gofmt -l`, the three `go build` lines,
`loc-check.sh`. Tree restored (`git checkout -- protoc/`, masterdata stubs and `test/rawdata/excelsource`
removed; pre-existing ignored leftovers kept). Not committed.

**Open.** ES-13/ES-10/ES-12 each bundle two questions; the `-race` time of `calculation`.

## 2026-10-02 — Review pass 1 of the test-environment concept (clean context)

**Task.** Review the docs of `07d2457` against the source code: line references, counts, CI, versions,
licences, middleware behaviour, callers; consistency and feasibility of the milestones. Docs only.

**Changes.** `known-errors.md` (#1, #2, #3, #5, #7, #8, #10, #12, #13, #18 – #20, #22 – #24, #27, #33 – #35, #40,
#42 – #44, #46 corrected or sharpened; none withdrawn), `test-coverage-concept.md` (§2.1, §2.3, T4, T9, F5,
F27, F28, §5 obstacles incl. a gRPC row, phase 0/2, effort, targets, §10.1), `m00` – `m06`, `README.md`
(order, effort, mapping, review log), `HOWTO-run-tests.md`, `open-points.md` (ES-10, ES-12, ES-13),
`AGENTS.md` (image name, `/query/*` auth, tenant validation, generator versions, hook caveat, log refs).

**Verification.** Stubs generated, `go vet`, all packages except the root, `calculation` alone and
`go test -race ./store/ebow` run once (results as in the README review log); tree restored
(`git checkout -- protoc/`, masterdata stubs and `test/rawdata/excelsource` removed; pre-existing
ignored leftovers kept). Two read-only sub-reviews checked #1 – #23 and #24 – #46 line by line.

**Open.** Most important corrections: `graph` is testable today (#12); `ProtectApi` differs from
`ProtectApp` (400, no `superuser`); a likely cause of the hang (#3). See the README review log for what
was not verified.

## 2026-10-02 — Working agreement, tracking files and the test-environment concept

**Task.** Create a concept for improving the test environment in `docs/improve-test-environment/`, like
eegfaktura-billing's `docs/improve-testing-environment/`, and bring the working agreement and the
tracking files of billing/eegfaktura-v3 into this repository. No code change.

**Changes.** New: `AGENTS.md` (adapted to Go, Badger, MQTT, GraphQL: tenant rule incl. the GraphQL
arguments, storage-format concept instead of Flyway, `-race` and `t.TempDir()` rules, glog `event=`
convention, versions/7-day rule incl. `@latest` tools, size limits without generated code), `CLAUDE.md`,
`known-errors.md` (#1 – #46), `open-points.md` (ES-1 – ES-23), `EXTERNAL_SOURCES.md` (licences from the
modules' own `LICENSE` files), `scripts/dev/loc-check.sh`, `docs/storage-format/README.md` + `TEMPLATE.md`,
`docs/improve-test-environment/` (`test-coverage-concept.md`, `README.md`, `HOWTO-run-tests.md`,
`m00` – `m06`). `CHANGELOG.md`: one line under `[Unreleased]`.

**Inventory and measurement** (HEAD `20e0852`, Go 1.26.0, protoc 29.3 / protoc-gen-go 1.36.11 from
`$HOME`, protobuf code generated for the run and removed afterwards):
- `go test -count=1 -timeout 180s -coverpkg=./... ./...` (root excluded, it has three `main`): 66 test
  functions in 10 packages; red `model` (stale wire format), `mqttclient` (missing
  `../energy-mass-test-data.json`), `test` (writes to `../../../rawdata`); `calculation` hung 180 s in
  Badger `Close` → `stopMemoryFlush`. Re-runs of `calculation`: 4 × green in about 13 s (two with the
  leftover `test/rawdata`, one on a fresh directory, one with `-coverpkg`) — intermittent (#3).
- `go test -race ./store/... ./mqttclient/ ./utils/ ./excel/ ./services/`: `store/ebow` red with three
  data races in `TestOpenMaxObject` (`pool.go:72, 78, 166, 181`) → F5 reproduced; the rest as above.
- Coverage (union of the two profiles): hand-written code 1774 / 4069 statements = **43.6 %** (all
  incl. generated 29.7 %); `excel` 71.5, `store/ebow` 70.5, `model` 64.5, `utils` 59.5, `calculation`
  56.0, `store` 45.4, `mqttclient` 33.0, `store/function` 12.3; `rest`, `middleware`, `graph`,
  `services`, `cmd`, `config` 0 (1001 statements; `middleware`'s `init()` needs Keycloak, #12).
- Routes: 13 `ProtectApp`, 2 `ProtectApi`, GraphQL `/query` with 2 fields (`grep HandleFunc`).
- Callers: eegfaktura-web `energy.service.ts`/`graphql-query.ts` at `c37a0b7`, eegfaktura-v3
  `EnergyStoreClient.kt` at `0b785d2`.
- Code review in two independent read-only passes (API/auth/store; calculation/excel/MQTT/tests);
  F1, F2, F3, F5, F7, F8, F9, F10, F11 re-read against the source.

**Findings worth reading first.** F1 (#16), F3 (#18) and #15 are critical and security relevant —
details not published (moved out of the repository on 2026-10-03). F4 (#19): MQTT messages are acked
before storing and store errors are reported as processed.

**Decisions.** None taken; every question is an open point with a recommended default (ES-1 – ES-23).
Defaults chosen for the concept: Go's built-in coverage with a floors script (no new dependency),
`t.Skip("known-errors #NN")` as the defect-test convention (billing's B-18 in Go form), fakes instead of a
broker, F1 fixed first in its own change. Open-point prefix `ES-` (billing uses `B-`, v3 `V3-`).

**Verification.** `bash scripts/dev/loc-check.sh` lists the 14 existing files above 300 lines (ES-6); the
new docs are Markdown (not counted). `git status` after the work: only the new and changed docs; the
regenerated `protoc/excel*.pb.go` restored with `git checkout`, the generated `masterdata*.pb.go` and the
test run's `test/rawdata/excelsource` removed; the pre-existing ignored leftovers (`excelsource3`,
`te100190`, `store/ebow/te*`) left as found.

**Open.** Every ES point; F1 fix; M0 start needs ES-2, ES-9 – ES-12, ES-22. The production `tenant` claim
shape (ES-20) and the secrets (ES-23) are operator questions.
