# AGENTS.md

Authoritative working agreement for every AI agent and every developer in this repository.
`CLAUDE.md` only points here. Keep this file as the single source.

## 1. Purpose

- Prefer verified repository facts over aspirational architecture.
- If the codebase and a requested target architecture differ, call out the mismatch and avoid silent large-scale rewrites.
- Treat architectural migrations (Go major version, storage engine, auth model, the move to energystore-v2) as explicit tasks.
- Every task ends with: code, tests, docs and the tracking files (section 14) updated.

## 2. Verified Baseline

This repository is **eegfaktura-energystore** (v1, image `ghcr.io/vfeeg-development/eegfaktura-energystore`;
the `Makefile` still uses the old name `energy-store`), the energy-data service of the
eegfaktura platform: it receives the 15-minute consumption and production values of the renewable
energy communities (EEG) over MQTT from the EDA pipeline (eda-xp), stores them per community in
BadgerDB and serves reports, raw data, load curves and Excel exports. It is called by the customer
web (`eegfaktura-web`) and by eegfaktura-v3, always with the user's Keycloak token; `/query/*` takes Basic
credentials instead (its callers are not recorded here).

- Go **1.25** (`go.mod`: `go 1.25.0`; `Dockerfile`: `golang:1.25`). Module path `at.ourproject/energystore`.
- Storage: BadgerDB v4 through the own wrapper `store/ebow` (buckets; values JSON-encoded by `codec/json`,
  the default of `ebow.Open` — the `msgp` codec exists but is unused), one database per
  community under `persistence.path/<tenant>/<ecId>` (section 8). No SQL database.
- APIs: REST with gorilla/mux (`rest/`), GraphQL with gqlgen (`graph/`, `/query`), MQTT ingest with
  Eclipse Paho (`mqttclient/`), a gRPC client to the backend's master data (`services/apiService.go`).
- Authentication: Keycloak bearer token, verified with go-oidc against the realm (`middleware/`); the
  community is the `tenant` (or `X-Tenant`) header, which must be in the token's `tenant` claim unless
  the token has the realm role `superuser` (section 6). `/query/*` uses Basic credentials and a
  Keycloak password grant (`middleware/api_authentication.go`; only `X-Tenant`, no `superuser` bypass).
- Configuration: Viper (`config/`, `config.yaml`), Keycloak clients from `keycloak.json`
  (`KEYCLOAK_CONFIG`). Logging: glog.
- Entry points: `server.go` (the service), `estore.go` (CLI on `cmd/`, cobra) and `initQoV.go` — three
  files with `func main` in one root package, built one file at a time (`known-errors.md` #1).
- Generated code: protobuf/gRPC stubs in `protoc/` (committed, `scripts/dev/generate.sh`), gqlgen code in `graph/generated/`.
- Deployment: one Docker image (`Dockerfile`, non-root UID 1000, `TZ=Europe/Berlin`); CI
  `docker-image.yml` checks the generated code, runs `gofmt`/`go vet`, the whole suite with `-race`
  and the coverage floors, and only then builds and pushes the image.
- Licence: AGPL-3.0 (`LICENSE`).
- A successor exists: `eegfaktura-energystore-v2` (PostgreSQL). This repository stays the production
  service until a cutover is decided (`open-points.md` ES-19).

## 3. Project Layout

- `server.go` wiring (router, GraphQL, CORS, MQTT dispatcher); `config/` configuration
- `rest/` REST handlers; `graph/` GraphQL schema and resolvers; `middleware/` token and tenant checks
- `mqttclient/` broker connection, per-tenant dispatcher, energy importer
- `store/` queries, imports, reports, raw-data delete; `store/ebow/` the Badger wrapper and the pool;
  `store/function/` aggregation functions
- `calculation/` allocation and period reports; `excel/` Excel import and export; `model/` data types;
  `utils/` time and row-id helpers; `services/` energy service and the gRPC master-data client
- `cmd/` the `estore` CLI; `mocks/` hand-written storage mock; `test/` fixtures (xlsx, json)
- `internal/testsupport/` test helpers (temp stores, builders, CR_MSG encoding, `tz` for `Europe/Vienna`); `scripts/dev/` generate, test, coverage and static checks
- Tests next to the code (`*_test.go`); test data in `test/` or the package's `testdata/`
- Documentation: `docs/`; storage-format concepts in `docs/storage-format/`
- Tracking files (section 14): `AGENT_LOG.md`, `known-errors.md`, `open-points.md`, `EXTERNAL_SOURCES.md`

Keep new code close to the feature it belongs to. Do not create parallel layers beside the existing packages.

## 4. Go Standards

- `gofmt -l .` and `go vet` print nothing for the files you touch (`scripts/dev/static-check.sh` in CI allows only the recorded `known-errors.md` #11, #47). Errors are returned and wrapped (`fmt.Errorf("…: %w", err)`), never dropped with `_` unless the reason is commented.
- Handlers (`rest/`, `graph/`) are thin: decode, validate, call `store`/`services`/`calculation`, encode. Every error path ends with `return`.
- No package `init()` with side effects (network, files, panics) in new code; dependencies are passed in (section 10 explains why: `middleware`'s `init()` is `known-errors.md` #12).
- Pass the request's `context.Context` (`r.Context()`) down to everything that does I/O; no `context.Background()` in a request path (#48).
- A goroutine that handles external input (MQTT, uploads) must not take the process down: return errors, `recover` at the worker boundary (#18).
- Shared state is guarded by **one** mutex per structure; goroutines are started with `wg.Add` before `go`. Every new concurrent path is tested with `-race`.
- Time: row ids are local wall-clock time (`Europe/Vienna` rules; the image sets `Europe/Berlin`, same rules). Convert with an explicit `*time.Location`, never mix `time.UTC` and `time.Local` in one calculation. Days have 92, 96 or 100 quarter hours.
- Energy values are `float64` today; any comparison in a test uses a tolerance, and a change of the rounding is a business decision (`open-points.md`).
- Badger: every iterator and transaction is closed on every path, including error paths.
- Dependencies: reuse what `go.mod` already has. Adding a module is an external source (section 12).

## 5. Reports and exports

- The Excel export and the report JSON are what communities and v3 read: a change to a column, a sheet, a
  value or a JSON field is a contract change (section 7) and needs a test that reads the output.
- A change that alters a reported number (allocation, rounding, quality flag) is a user decision: note it
  in `open-points.md` and in `CHANGELOG.md`.

## 6. Security Standards

- Every route needs a valid bearer token (`ProtectApp`, `GQLProtect`) or Basic credentials (`ProtectApi`); deny by default.
- **Tenant isolation is the first rule.** The tenant header must be in the token's `tenant` claim (or the token has `superuser`), and every handler and resolver uses **that** tenant — never a tenant taken from the path, the body or GraphQL arguments. A route without that is a defect.
  **State on `main` (2026-10-03): a critical tenant-isolation defect is open (`known-errors.md` #16, details not published).**
- Tenant and `ecId` are validated (`^[A-Za-z0-9]+$`, tenant at most 8 characters, `store/ebow/rowdata.go:113-124`) before they reach the file system; the directory is `<tenant>/<ecId>` (tenant in lower case).
- Destructive operations (`rawdata/delete`) need the realm role `superuser`.
- Never log tokens, passwords, full request bodies, MQTT payloads or metering-point data at levels enabled in production; sanitize logged request values (no line breaks, bounded length).
- Secrets never in the repository (`known-errors.md` #15).

When editing auth or security-sensitive code: report critical security issues before writing tests that would normalize insecure behaviour.

## 7. API Conventions

- REST under `/eeg/...` (v1) and `/eeg/v2/{ecid}/...`; JSON camelCase; the tenant is the header, the community is `{ecid}`.
- GraphQL `/query`: `lastEnergyDate`, `singleUpload`.
- The callers are `eegfaktura-web` (`src/service/energy.service.ts`, `graphql-query.ts`) and eegfaktura-v3
  (`integration/energystore/EnergyStoreClient.kt`); the MQTT input comes from eda-xp (and v3's
  `energy-mock` in test stacks). A change to a path, a DTO, a GraphQL field or the MQTT payload is a
  contract change — say so in `CHANGELOG.md` and tell the callers.
- An error response carries a status and stops: no second write after `respondWithError`.

## 8. Storage format and the format concept

- The persisted format is: the directory layout `<tenant>/<ecId>`, the row ids (`CP/yyyy/MM/dd/hh/mm/ss`,
  local time), the meta record `cpmeta/0` (metering points with `SourceIdx`, direction, period), the
  value layout of a raw line (consumer triples, producer pairs, QoV arrays) and its JSON encoding
  (`store/ebow/codec/json`; keys through `codec/key`).
- Data on disk is never rewritten silently. Production held about 1.2 TB of Badger data (2026-09 figure, not re-checked).

### 8.1 Storage-format concept is mandatory

Every change that touches persisted data ships with a concept in
`docs/storage-format/<YYYY-MM-DD>-<slug>.md` (template: `docs/storage-format/TEMPLATE.md`) that answers:

1. **What changes** (keys, value layout, meta record, directories) and why.
2. **Data migration**: how existing data is converted (tool, `estore` command), expected volume and duration.
3. **Compatibility**: whether the previous version still reads the new data; what must be deployed together.
4. **Rollback**: the steps and the data that cannot be restored (backup first).
5. **Verification**: the test or the check on a copy of production data that proves it.

Document the main process only — the one path used to migrate the production system.

## 9. Local Runtime and Build

- The protobuf stubs are committed (M0). After changing a `.proto`: `bash scripts/dev/generate.sh`
  (pinned protoc 29.3, protoc-gen-go v1.36.11, protoc-gen-go-grpc v1.6.2; it checks the versions) and
  commit `protoc/`; CI fails when regeneration changes anything.
- Build: `go build -o energystore server.go` and `go build -o estore estore.go` (per file: the root package has several `main`).
- Tests: `bash scripts/dev/test.sh` (all packages but the root, `-race`, coverage), one package
  `go test -count=1 ./<pkg>/` — see `docs/improve-test-environment/HOWTO-run-tests.md`.
- Run the service: `config.yaml` plus a reachable Keycloak (`keycloak.json`, the middleware contacts it at start) and an MQTT broker; the whole platform runs from `eegfaktura-docker-compose`.

## 10. Testing Standards — unit tests are mandatory

No change without tests. A change that adds or alters behaviour without a test for that behaviour is incomplete.

### 10.1 Common rules

- Review the implementation before writing tests. If code is broken or insecure, report it first and fix it (or ask) before writing tests. Never write tests that bless broken behaviour.
- Prefer high-signal tests that protect API-visible behaviour and reported numbers; avoid tests that only print.
- Deterministic data: a fixed `*time.Location` (`Europe/Vienna`), fixed dates, no `time.Now()` in assertions, no sleeps, no real network, no broker, no Keycloak.
- Badger data only under `t.TempDir()` — never in the source tree, never outside the repository.
- No `t.Parallel()` in packages that use Viper or the `store/ebow` pool: both are process-wide globals.
- A fixed test blesses the current contract: when a test fails because the contract changed, update the test to the new contract, never weaken the assertion.

### 10.2 Levels

- Unit tests (`testing`, testify) for `utils`, `model`, `calculation`, `excel`, `store/function`, the codecs.
- Storage tests against a real Badger in `t.TempDir()` for `store`, `store/ebow`, imports and reports.
- HTTP tests with `httptest` and the real router and middleware (`internal/testsupport/apitest`), tokens signed by the in-process fake Keycloak `internal/testsupport/fakeoidc` (it initialises before `middleware`, `known-errors.md` #12).
- MQTT tests with fake `mqtt.Message`s against the dispatcher and the importer (no broker).
- Cover: happy path, tenant isolation (own, foreign, missing tenant, `superuser`), invalid input, empty store, DST days (spring 92, autumn 100 slots), month and year boundaries, quality flags.

### 10.3 Test efficiently

- While iterating: one package, `go test -count=1 ./store/`, or one test with `-run`.
- **Once per step**, before the merge: the whole suite with the race detector, `go test -race -count=1 ./...`. A change that only passes in the narrow loop is not green.
- Tests are written once, at the end of a step, against the shape that survived — not against a draft.
- No measurement is a test: benchmarks (`ExportBench_test.go`) and mass imports run only when the user says so.

## 11. Logging

The log must let a human or an AI reconstruct what happened without access to the running system.

- Message convention for new and changed code: `event=<domain.action.outcome>` followed by `key=value` pairs with stable keys, ids instead of names. Example: `event=energy.import.stored tenant=RC100001 ecId=RC100001 meter=… slots=96 durationMs=41`.
- `glog.Error` only once per failure with the cause; `Warning` for handled anomalies; `V(n)` for decisions and counts.
- Never log tokens, passwords, MQTT payloads or request bodies (today they are logged at `V(4)`/`V(5)`, `known-errors.md` #38, #40).
- Structured output (`log/slog` JSON) is `open-points.md` ES-5; until decided glog stays.

## 12. External sources — trusted only

- Every external source the build, the tests or the running service depends on is listed in `EXTERNAL_SOURCES.md`: registries, base images, Go modules with their licence, tools, runtime services.
- Adding a new external source — a module, a base image, a tool, a service the code calls — requires an explicit decision: record the question in `open-points.md`, wait for the answer, then add the row to `EXTERNAL_SOURCES.md` in the same change.
- Licences: the service is AGPL-3.0; every dependency must carry an open-source licence that allows redistribution and is compatible with it (permissive, MPL-2.0, LGPL, GPL-3.0, AGPL-3.0, EPL-2.0 with its secondary licence or EDL; not GPL-2.0-only). An unknown licence is a blocker. Check the module's own `LICENSE` file, not a summary site.

### 12.1 Strict rule: fixed versions, nothing younger than 7 days

1. **Every version is fixed.** Exact module versions in `go.mod`/`go.sum`, full tags in the `Dockerfile`, CI actions by commit SHA, tools by version (`go install …@vX.Y.Z`, never `@latest`) — no ranges, no `latest`, no floating major tags. The current floating ones are `known-errors.md` #10.
2. **Nothing younger than 7 days.** A version enters this repository only when it was published at least 7 days before. For a manual change, check the publish date (`proxy.golang.org/<module>/@v/<version>.info`, Docker Hub, release page) and record it in the commit or `AGENT_LOG.md`.
3. Dependency updates go through reviewed pull requests with a green test run.

## 13. Size limits — lines per file

| Lines | Status | Rule |
|---|---|---|
| ≤ 300 | green | fine |
| 301 – 450 | yellow | allowed; plan a split and note it in `open-points.md` if it does not happen in the same change |
| 451 – 600 | red | split before the change is merged unless the user explicitly accepts it (record in `open-points.md`) |
| > 600 | blocked | not accepted for new code; existing files above it are `open-points.md` ES-6 |

Applies to hand-written Go files including tests; generated code (`*.pb.go`, `graph/generated/`, `*_gen.go`) is excluded. `bash scripts/dev/loc-check.sh` prints the status. Split by responsibility, never by arbitrary cut.

## 14. Tracking files — keep them current in every task

- `AGENT_LOG.md` — every AI session appends one entry: date, task, what was changed, decisions, verification, what is still open. Newest at the top. Write it before the final summary.
- `known-errors.md` — every known defect, flaky test, workaround or gap, with status (open, mitigated, fixed on date). Add the entry the moment the problem is found, even when it is fixed in the same change.
- `open-points.md` — every decision that belongs to the user or the maintainer: one line with date, question and current assumption; answered points move to "Decided".
- `EXTERNAL_SOURCES.md` — see section 12.
- `docs/storage-format/` — see section 8.1.
- `CHANGELOG.md` — every change relevant for operation or the callers, under `[Unreleased]`.

## 15. Do Not

- Do not silently upgrade Go, Badger, gqlgen or Paho major versions.
- Do not change the storage format without the concept of section 8.1.
- Do not add a route or resolver that takes the tenant from anywhere but the checked header.
- Do not add an external source without a decision, and never one whose licence forbids redistribution.
- Do not use a version range, a floating tag, `@latest`, or a release younger than 7 days.
- Do not silence a failure to make a run green: no swallowed errors, weakened assertions, skipped tests or deleted checks (the one documented exception is `open-points.md` ES-10).

## 16. AI Behavior Rules

- Prefer small, targeted changes; keep documentation aligned with the repository state.
- When requirements are ambiguous, keep the existing conventions, state the assumption, and record the question in `open-points.md`.
- If docs and code disagree, fix the docs or call out the mismatch — never assume the docs are right.
- Fix pre-existing errors you meet, even outside the task, and say so in the summary; verify a "pre-existing" failure against a clean tree first, then fix the cause, not the symptom. Where the fix is a decision (a version jump, a changed report), record it in `open-points.md` instead.
- This is an upstream repository of the eegfaktura project: changes reach production through a pull request the maintainer reviews. Commit in small steps with descriptive messages; never push without being asked; never `git add -A` (generated and test files are not all ignored).
- End every task with the checklist: build green, tests green with `-race`, format concept written if stored data changed, logging events added, `CHANGELOG.md`, `AGENT_LOG.md`, `known-errors.md`, `open-points.md`, `EXTERNAL_SOURCES.md` updated.

## 17. Quick Reference

- Generate (only after a `.proto` change): `bash scripts/dev/generate.sh` — build: `go build -o energystore server.go`
- Tests: `go test -count=1 ./<pkg>/` — all: `bash scripts/dev/test.sh` (every package but the root, #1/ES-24; `-race`, `cover.out`)
- Coverage: `bash scripts/dev/coverage-check.sh cover.out` (floors in `scripts/dev/coverage-floors.txt`, raised at the end of each milestone)
- Format and vet: `bash scripts/dev/static-check.sh` (allows only the recorded findings #11, #47)
- Size check: `bash scripts/dev/loc-check.sh`
- Image: `docker build -t eegfaktura-energystore .` (the stubs are committed)
