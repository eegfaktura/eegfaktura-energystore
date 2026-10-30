# Changelog

All notable changes to **eegfaktura-energystore (Go measurement/energy data store)** are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and
versioning follows the deployment release tags. Detailed diffs stay in the `git log`;
this changelog highlights the changes relevant for overview and operations.

## [Unreleased]

### Security
- **Basic authentication of the `/query` API** (energystore #39): security relevant — details not published.
  Compatibility: clients that send Basic credentials as standard base64 (RFC 7617) are unaffected; a client that
  encoded them URL-safe (`-`/`_`) to match the old server behaviour now gets 403.
- **Reading of the `tenant` token claim** (energystore #14): security relevant — details not published.
- **Sample `keycloak.json`** (energystore #15): security relevant — details not published.
- **Energy routes for member tokens** (energystore 261025-ca1): security relevant — details not published.
  Operations: new setting `services.backend-api` (the backend's HTTP address, e.g.
  `http://eegfaktura-backend-service.eegfaktura:8080`); set it before rolling out this version. energystore now
  calls the backend's `GET /participant` (contract: `meters[].meteringPoint`, `meters[].registeredSince`,
  `meters[].participantState.activeSince`/`inactiveSince`) and needs network access to the backend on port 8080.
  Without the setting energystore logs a warning at start. Member logins get 503 on per-meter requests only while
  the backend is unreachable or the setting is missing; admin and owner logins never call the backend.

### Fixed
- **Gap filling: no phantom quarter hours on the summer-time day, fill rows in their own slot** (energystore #22).
  **This changes values in reports, raw-data queries and Excel exports.** Where the store has no row for a
  quarter hour, the reports insert an empty row (0 kWh, quality L0). Two errors are fixed:
  - On the day summer time starts (e.g. 29.03.2026) the clock jumps from 01:45 to 03:00. This jump was taken
    for a missing hour: reports, raw-data queries and the Excel export showed **four extra quarter hours
    (02:00–02:45) with 0 kWh and quality L0**, and the summary sheet counted them as L0. They are gone: the day
    has its 92 quarter hours, and the L0 count of that day drops by four per metering point.
  - In reports and raw-data queries (not the Excel export) the empty row for a real gap was placed one or two
    hours **later** than the gap (winter/summer time), on top of a real quarter hour, and the gap itself had no
    row. It now sits in the missing quarter hour.
  kWh sums are unchanged (the inserted rows are 0 kWh). What changes: the number of rows, their timestamps, the
  L0 counts and the quality shown for the hour or day such a row falls into. **No data repair is needed:** the
  fill rows were never stored — both gap fills (reports/raw-data queries and the Excel export) build them in
  memory while reading — so the correction applies to already stored data as well. One possible exception, not
  touched by this change: an Excel file from a grid operator portal that contains rows 02:00–02:45 on the
  summer-time day would have stored real rows for those times when it was uploaded.
- **Tests: the export test for a metering point without stored data runs again** (energystore #24): the defect
  was fixed in 1.8.0; the stale skip of `TestS08_ExportForAMeterWithoutData` is removed.
- **Excel import: a new metering point with a missing or unknown "Energy direction" is refused with an error**
  (energystore #27) naming the metering point, instead of a nil-pointer panic that the upload showed as an opaque
  error. Nothing is stored for such a file.
- **Excel import: decimal comma and unreadable values** (energystore #28). A value with a decimal comma (`1,5`)
  is read as 1.5; before, it was stored as 0. Spaces around a value (also a non-breaking space) are ignored
  (` 1,5`, `1.5 `). Note: `1,234` is read as 1.234 (decimal comma), not as one thousand two hundred thirty-four.
  An empty or blank cell is still 0. **These cell contents now refuse the upload** with an error naming the row
  (its timestamp) and the metering point — before, they were stored as 0 without notice: text (`n/a`, `-`, `x`),
  Excel error values (`#N/A`, `#DIV/0!`, `#VALUE!`), numbers with thousands separators (`1.234,5`, `1,234.5`,
  `1 234`, `1'234`) or more than one comma (`1,2,3`), and `NaN`/`Inf`/`Infinity` (any sign; these were stored
  as such before, not as 0). Only the value cells of the recognised metering points are checked. A sheet
  without energy rows (e.g. an empty workbook) or without the `MeteringpointID`/`Metercode` header is refused
  with an error instead of reporting success; before, an empty sheet also rewrote the stored metering-point
  meta without any metering point. Not changed: the quality row is still not read and every imported slot keeps
  quality 1; negative values are still accepted.
- **No lost metering points when the MQTT import and an Excel upload write the same community at once**
  (energystore #33). Both read the metering-point record (`cpmeta/0`) and the rows, change them and write them
  back; run at the same time, one import's new metering point (or values) could be overwritten by the other.
  The imports of one store now take turns (a lock per tenant/ecId inside the service); an Excel upload holds it
  for the whole file, so MQTT imports of that community wait until it is done. During a long upload the
  community's MQTT messages wait unacknowledged; if the service restarts meanwhile, mosquitto delivers them again
  and the import is idempotent, so no data is lost. A graceful shutdown waits for a running upload to finish.
  A failed write of the record is now reported instead of being ignored.
- **Deleting raw data no longer loses values an MQTT import writes at the same time** (energystore #34). The
  delete (`/eeg/v2/{ecid}/rawdata/delete`) reads the rows of the range and writes them back with the metering
  point's slots zeroed; an import of the same community in between was overwritten (values of other metering
  points in that range lost). A real delete now takes the same per-community lock as the imports (#33); a dry run
  only reads and does not wait.
- **`POST /eeg/{ecid}/excel/export/{year}/{month}` answers 400 instead of crashing** (energystore #53). The route
  passes no participant list to the export, so every call panicked and the client saw a dropped connection; no
  mail was ever sent. It now answers `400 {"error":"no metering points given for the export"}` and still sends no
  mail. No caller is known (web and v3 do not use it); the Excel download (`/excel/report/download`) is unchanged.

## [1.8.1] – 2026-10-28

### Security
- **Go 1.26.10** (platform#181): the build image (`golang:1.26.10`), `go.mod` (`toolchain go1.26.10`; the
  `go 1.26.0` directive is unchanged), the PR checks and the image build move from 1.26.9 to 1.26.10 (released
  2026-10-19). It fixes CVE-2026-111943 / GO-2026-6809 (HIGH, `crypto/x509` trusted untrusted intermediate
  certificates; reached through the outgoing HTTPS calls). A patch release: language version, GODEBUG defaults and
  garbage collector are unchanged.
- **Slim runtime image** (prod image scan 2026-10-28): the image was the full `golang` image (Go toolchain, git,
  ssh, curl, gnupg, compilers, the sources until `rm`); it is now a two-stage build with the runtime on
  `debian:trixie-20261020-slim` (the Debian release of the build image) with only `ca-certificates` and `tzdata`.
  The image keeps `/usr/local/bin/energystore` and `/usr/local/bin/estore`, `/etc/energystore/config.yaml`,
  `TZ=Europe/Berlin`, user `app` (1000:1000), `/opt/rawdata` (volume) and `/opt/energy` owned by it, the working
  directory and the command. **Not in the image any more:** `curl`, `git`, `ssh`, `go`. eegfaktura-docker-compose
  wraps the command in `url-poller.sh`, which calls `curl` inside this container: it must be adapted before it moves
  to an image with this change (the Kubernetes manifests do not use it).

### Fixed
- **CI: the Snyk Code scan fails visibly when it cannot run.** An invalid `SNYK_TOKEN` ended the weekly and
  push scan green without any result (`continue-on-error`); now only „issues found“ keeps the run green (the
  issues go to Code Scanning), an authentication or tool error fails it. The Snyk CLI is pinned to 1.1308.1
  instead of the latest version.

## [1.8.0] – 2026-10-25

### Security
- **Bearer tokens: RS256 only** (platform#191, as backend#99). Both go-oidc verifiers took the allowed signing
  algorithms from the Keycloak discovery (HS* and `none` were already dropped); they now accept RS256 only, as
  required.
  - App verifier (`GQLProtect` and the bearer-protected REST routes): a client-supplied bearer token signed with
    RS384/RS512/PS*/ES*/EdDSA under a realm key was accepted; it is now rejected (401).
  - API verifier (`ProtectApi`): it only checks the ID token the service fetches itself via password grant, so a
    client cannot choose its algorithm. Effect: if the ID-token signature algorithm of the api client in Keycloak
    were ever set to something other than RS256, every `ProtectApi` call would fail with 403.
  - Logins are not affected. As of 2026-10-23 the public JWKS of Prod/Pilot (login.eegfaktura.at) and Dev hold
    exactly one signing key (RS256), and the realm sets no different ID-token algorithm. If a realm ever got an
    active non-RS256 signing key (e.g. on key rotation), logins would fail with 401/403.
  - Tests also cover HS256 signed with the realm's public key (PEM and DER) and `alg=none`: rejected.
  - The `middleware` package now has tests (fake OIDC provider, no live Keycloak) and runs in the unit-test list
    of `pr-checks.yml` and `docker-image.yml`.
- **CR_MSG: unpacking is limited** (platform#181). An energy data message from MQTT was unpacked completely into
  memory; a small but highly compressed payload could have filled the memory of the pod. It is now unpacked up to
  `mqtt.maxDecompressedMB` (env `ENERGYSTORE_MQTT_MAXDECOMPRESSEDMB`, default 512 MB); a larger one is logged as
  „Error decoding CR_MSG transport payload … decompressed message larger than …“ and dropped like any undecodable
  message: it is acknowledged and not redelivered, its data would have to be requested again. A message carries the
  data of one metering point and stays far below the default. The value is read once; a missing, non-numeric or
  non-positive value falls back to 512 MB with a warning (a limit of 0 would drop every message).

### Fixed
- **Excel energy report download: no panic for a metering point without stored data**
  (`POST /eeg/{ecid}/excel/report/download`). A requested metering point without an entry in the meta record
  `cpmeta/0` (no data ever imported for it, or none for the whole community) made the energy sheet dereference the
  missing meta and the request ended without an answer. Such a metering point now gets no columns (the summary
  sheet already skipped it); if no rows exist at all, the answer is the usual 404 `no_entries` („no Rows found“).
  eegfaktura-web only requests metering points from the community's metadata, so its file is unchanged. The
  export by mail (known error #53) has a different cause and is not changed.
- **Report of a community without any energy data: empty report instead of 500** (`POST /eeg/v2/{ecid}/report`).
  Without any imported data the meta record `cpmeta/0` does not exist and the report answered
  500 `{"error":"Record doesn't exist"}`. It now answers 200 with the same shape as a report without values: the
  requested participants with zero sums, `"meta": []`, totals 0. Other errors still answer 500. eegfaktura-web
  (web#191) handles both answers. Summary and billing-check already answered 200 without data.
- **Raw data export without a meter list: no empty query targets** (`POST /query/rawdata` without `cps`). The target
  list of the community's metering points started with one empty entry per metering point. No effect on the
  answer: the duplicates were collapsed to one empty entry and no metering point has an empty name, so it was
  skipped on every row; only wasted work.
- **CR_MSG: acknowledged only after it was processed, store errors no longer reported as stored** (#19). An energy
  data message was acknowledged to the broker as soon as it was handed to the tenant worker; a crash, OOM kill or
  restart before the store lost it. A store error was logged, but the message was still confirmed to the backend,
  which wrote „Energiedaten empfangen“ into the process history although the data was not stored.
  Now the message is acknowledged after it has been processed (stored, failed, undecodable, or a recovered panic);
  a message still in process when energystore stops is redelivered by mosquitto when energystore reconnects
  (persistent session) and imported again (the import is idempotent; the backend may get a second process
  history entry). If any day block of a message cannot be stored, no `cr_msg_history` reply is sent: EEG admins
  see no „Energiedaten empfangen“ entry for it, the gap shows in the check before billing, and the data has to be
  requested again. If only some days of a message fail, the reply is dropped for the whole message; the days that
  were stored are still in the store (a reply with only the stored days would change the entries in the process
  history, which list the blocks as sent). An error reading or updating the meta record `cpmeta/0` ends the import
  of the message at once and is always named in the log line; otherwise the other days are still stored. The log line is „energy data not stored, no cr_msg_history reply“
  with tenant, ecId, metering point, conversation id and at most five day errors („… N more“).
- **A stored row that cannot be read is an error, no longer the silent end of the data** (#31). Reports, the
  billing basis, the raw data and load curve queries and the Excel export stopped at such a row and returned a
  shortened result as success; they now fail with „stored record cannot be read“ (with the key for a record that
  cannot be decoded; the cause is in the ERROR log „ebow: cannot decode record …“ or „ebow: cannot read record …“,
  never in the API response, which could otherwise show local file paths), so a shortened result is no longer
  passed on as complete. The web app does not present these errors well yet (a report may come back empty or show
  a raw key such as `E_undefined_undefined`); a clear message there is a follow-up (platform#194). On the import path the store of that day is aborted: before, the rows after the unreadable one were rebuilt with the
  imported metering point only and the values of the other metering points in those quarter hours were lost; now
  nothing of the day is written and no „Energiedaten empfangen“ reply is sent (see #19). The CLI tools
  (`estore data`, `estore move`, `estore reset`) stop with the error as well. Iterators release their read
  transaction also after such an error, and a store whose meta record cannot be read or written no longer keeps
  Badger's directory lock until the restart. Without unreadable rows nothing changes.

### Changed
- **CR_MSG import: batched per community, fair lanes for all tenants** (concept „Energiedaten gebündelt
  importieren“). A large request (e.g. a month for 2,400 metering points) kept energystore busy for hours, because
  every metering point and day read and wrote the rows of all metering points; meanwhile the messages of every
  other tenant were stuck behind it (one dispatch loop for all tenants), and the burst afterwards opened hundreds
  of stores (OOM kills). Now:
  - the messages already queued for a tenant are imported together: per community and day the rows and the meta
    record `cpmeta/0` are read and written once for the whole batch, no format change. Tests compare the stored
    rows (as stored, including the length of the quality arrays) and the whole record `cpmeta/0` (ID, SourceIdx,
    direction, periods) with the import message by message: they are equal. A batch is unpacked and imported
    in parts of at most about `mqtt.maxDecompressedMB` of unpacked data each, so the limit per message still
    bounds a batch: about twice the limit of unpacked JSON per lane (the decoded data on the heap is several
    times that). Two differences that change bytes, not
    values: when a day batch fails and its blocks are stored one by one, rows created then may be sized with
    the counts of the whole batch (more trailing zeros); and a day block that spans two days after the DST change
    rewrites the rows of both days, the other day's unchanged (the single import does the same);
  - `import.parallel` lanes (env `ENERGYSTORE_IMPORT_PARALLEL`, default 4) import at the same time, each one batch
    of at most `import.batchMax` messages of one tenant (env `ENERGYSTORE_IMPORT_BATCHMAX`, default 50); a tenant
    with more work queues up again behind the others (round robin), so a large request no longer delays the daily
    data of the other communities by more than one batch per lane. Receiving a message never waits for another
    tenant;
  - a store is closed 15 s after its tenant's last batch as before, or already 2 s after it when more than
    `import.parallel` stores are idle (in the background, the lanes go on); a tenant waiting for its turn keeps
    its store open, except at the limit below when no idle store is left: then the store of the tenant waiting
    last is closed and opened again on its turn;
  - **limit on open importers:** `import.maxOpen` (env `ENERGYSTORE_IMPORT_MAXOPEN`, default 32; a value below
    `import.parallel` is raised to it with a warning) bounds the number of tenants with an open importer (one
    being closed still counts). Memory is about 90–112 MB per such tenant, more for a tenant with several
    communities (one store each) or while reports and exports hold the same stores open; it is not a hard
    memory limit. At the limit a lane
    closes the longest unused idle store first (about 0.3 s) or waits until one becomes idle: when many
    communities deliver at once, their data takes longer instead of filling the memory. Local run (100
    communities delivering every 6 s plus a month request of a large community, window 1000): at most 32 open
    importers, peak heap about 4.5 GB, small messages wait up to about 15–20 s (p95), instead of 100 open stores
    and about 10 GB without the limit;
  - per message as before: the `cr_msg_history` reply only when all its days are stored, the same ERROR log line
    otherwise, ack after processing (#19). A day that fails in a batch is stored again message by message, so only
    the messages with that day are affected; a meta record error falls back to the message-by-message import.
    After a panic in a batch its messages run one by one, so only the message that panics again is lost (acked,
    no reply).
  - The three values are read once at start; a missing, non-numeric or non-positive value falls back to the default
    with a warning.
  - **Release prerequisites (together with #19, manual ack), in prod and pilot, after reading out the current
    values:**
    - raise mosquitto's `max_inflight_messages` (proposal 1,000; default 20). With 20 unacknowledged messages at a
      time the batches stay at one or two messages and the throughput depends on the window, not on the import.
      The setting is global and applies to every client (it cannot be set for the energystore client only); for
      the clients that acknowledge right away raising it changes nothing in practice;
    - raise `max_queued_messages` (default 1000 per client) or set it to `0` (no limit): with the manual ack a
      burst queues at the broker, and mosquitto drops messages beyond that limit silently.
- **Fixed with it:** `Close` of the dispatcher could return before a worker had started and closed its store
  (known-errors #41); it now returns after every store is closed. A running batch stops at its next step
  (message, run of messages, day) instead of finishing (50 large messages could take longer than the pod's
  grace period); its unfinished messages stay unacknowledged and mosquitto delivers them again.
- Dependencies: `github.com/klauspost/compress` 1.18.0 → 1.18.7 (platform#181; Trivy CVE-2026-63209, HIGH),
  an indirect dependency of `badger/v4` (`badger/v4/table` uses its `s2`/`snappy` and `zstd` packages for block
  compression; same storage format within 1.18.x). Replaces Dependabot #76.

### Build and CI
- **The image job's gate is wider.** Besides the tests it checks that the committed protobuf stubs match a
  regeneration (`scripts/dev/generate.sh`, pinned protoc 29.3 and plugins), runs `gofmt`/`go vet`
  (`scripts/dev/static-check.sh`), the whole test suite with the race detector (`scripts/dev/test.sh`,
  every package except the root, was a fixed package list without `-race`) and per-package coverage floors, uploads the coverage report and runs
  `govulncheck` (reporting only) — all before the image is built.
- `protoc/masterdata*.pb.go` are committed; `docker build` works from a fresh checkout. The build paths
  of `energystore`/`estore` are unchanged (moving the entry points to `cmd/` was not done, `open-points.md` ES-24).
- Tests write Badger data only into temporary directories and fix the time zone to `Europe/Vienna`;
  the intermittent hang of the `calculation` tests is gone (`known-errors.md` #3).

### Documentation
- `EXTERNAL_SOURCES.md` lists every source of the build, the tests and the service. The concept for
  repairing the test environment and raising the test coverage, the storage-format concepts and the
  working files of that work are kept outside the repository. No code change.

### CI
- `docker-image.yml`: no image build on a draft pull request — it runs when the pull request is marked ready for review (`ready_for_review`) and on every later push to it; `pr-checks.yml` still checks drafts. Pushes, tags and the deploy dispatch are unchanged.
- `pr-checks.yml` on the test environment: the unit job runs every tested package (`scripts/dev/packages.sh`, no race detector), the full job the static check, `scripts/dev/test.sh` (race detector, coverage) and the coverage floors, in `TZ=UTC`; the report-only "Known errors" job and the protobuf generation (the stubs are committed) are gone. The results stay artifacts for the CI report collector.

## [1.7.0] – 2026-10-21

### Added
- Time-of-use window folding (ZVT): the participant report request
  (`POST /eeg/v2/{ecid}/report`) accepts up to two generic time windows per
  metering point (`timeWindows: [{key: T1|T2, from: "HH:MM", to: "HH:MM"}]`,
  15-min raster, `from > to` crosses midnight). The report response then
  carries per-window sums of the billing quantity per metering point
  (`report.buckets: [{key: BASE|T1|T2, kWh}]`, consumer = utilization,
  producer = production − allocation). `BASE` is the residual against the
  period total, so the kWh partition is exact by construction. Window
  membership is evaluated against the local wall-clock time encoded in the
  stored row ids (container TZ, Europe/Berlin = Vienna offset). energystore
  has no tariff or price knowledge — the windows are plain daytime ranges.
  Invalid windows (key, raster, `from == to`, >2 per meter) are rejected
  with 400.

### Changed
- Build: Go 1.26.9 instead of 1.25 (platform#181) in `go.mod` (`go 1.26.0`, `toolchain go1.26.9`), the Dockerfile
  (`golang:1.26.9`), the PR checks and the image build. Fixes the standard-library vulnerabilities govulncheck reported
  for 1.25.14 (`net/http`, `net/textproto`, `crypto/tls`). 1.26.10 (2026-10-19, `crypto/x509`, GO-2026-6809) was less
  than 7 days old and follows separately.
- Dependencies: `golang.org/x/net` 0.58.0 → 0.60.0 (platform#181; GO-2026-6603, -6610, -6611, -6612, -6617), which
  pulls `golang.org/x/crypto` 0.55.0 → 0.57.0 and minor bumps of `x/mod`, `x/sync`, `x/sys`, `x/text`, `x/tools`.
- CI: no more known failing test packages (platform#181). `calculation`, `model` and `mqttclient` move from the
  report-only job "Known errors" (removed) into the blocking test jobs, `store/ebow` runs as a whole package
  instead of only `-run TestVlogGC`; test timeout 400 s, 600 s for the full suite with the race detector
  (`calculation` takes 220–270 s there).
- Tests: `model` checks the current CR_MSG wire format (as eda-xp publishes it: no envelope, `energy` as an array,
  `conversationId`, `messageCode`). `mqttclient` `TestNewMqttEnergyImporter` closes its importer and works in a
  temporary directory (left Badger files open on Windows and data behind for the next run); `TestMassImport` is
  skipped while `energy-mass-test-data.json` (real data, not in the repository) is missing. `store/ebow`
  `TestReadOnly` is skipped on Windows (Badger has no read-only mode there); `Test` compares the bucket names
  sorted (`Buckets()` returns them in map order, so the test failed about every second run). `excel` „Initiate Metadata extended MM
  field" removed `test/rawdata/excelsource` instead of its own `excelsource3`: when `go test` ran `excel` and
  `calculation` in parallel on Linux, that deleted the open store of `calculation`, whose Badger close then hung
  until the timeout (the cause of the „calculation hangs" in CI).
- Tests: the `store/ebow` VlogGC tests retry the forced compaction until Badger has written discard statistics
  (Badger sets the read watermark asynchronously after opening; a compaction before that kept every version and
  the tests failed now and then with `ErrNoRewrite`). `Test` closes its reopened database, so the cleanup no
  longer leaves a ~2.2 GB `bow-*` folder in TMP on Windows. `mqttclient` `TestImportRawdataStore` removes
  `test/rawdata/te100190`, the folder it writes (it removed `rc100190`).
- CI: the unit job checks `gofmt` and runs `go vet` (every package except the root) before the tests;
  `calculation` uses a keyed `store.Engine` literal, an unreachable `return` in `excel` is gone.
- Tests: the `store/ebow` VlogGC test helper `flatten` counts a forced compaction as done only when it wrote new
  discard statistics itself (platform#181). Badger already compacts automatically while the tests fill the
  value log and records the files superseded up to then; when the forced compaction then lost the race against
  the asynchronous read watermark, the check "any statistics present" still passed, the later files had none,
  and `TestVlogGCReclaimsValueLog` failed now and then with 8 instead of 12 rewrites (31 MB value log left).
  Test code only; the value log GC itself is unchanged.

### Fixed
- Ops command `reset`: begin and end are now rounded down to the quarter hour as intended (the `Truncate`
  results were discarded). A begin of 10:07 now also resets the 10:00 slot.

### Notes
- Go 1.26: with `go 1.26.0` in go.mod the GODEBUG defaults of 1.26 apply (`cryptocustomrand`,
  `httpservecontentmaxranges`, `tlssecpmlkem`, `urlstrictcolons`); none affects this service (no custom crypto
  rand, no `ServeContent`, Keycloak URL has a host name). Go 1.26 uses the Green Tea garbage collector by
  default: watch the pod memory in Dynatrace after the first rollout (opt-out at build time:
  `GOEXPERIMENT=nogreenteagc`). (platform#181)

## [1.6.1] – 2026-10-19

### Added
- **Energy report: quantity balance in the summary** (customer request 19.10.2026). Row 10 of the summary sheet
  shows „Mengenbilanz: Erzeugung - Überschuss - Eigendeckung“ as the Excel formula `=ROUND(B9-B8-B7,3)`, with the
  value stored as well (visible without recalculation, e.g. in mail previews). With complete data it is close to 0,
  also with multiple participation (G.01T/P.01T are the community's share).

## [1.6.0] – 2026-10-18

### Added
- **`POST /eeg/v2/{ecid}/billing-check`** for the checks before billing in web (platform#148), read only,
  protected like `/eeg/v2/{ecid}/report`. Request: period (`start`, `end`, epoch ms of the first and the
  last day) and the metering points with their active windows (`cps`, like the Excel export). Response per
  entry: quarter hours with an L3 value and their days, first and last quarter hour with a value (each
  entry's active window ends at the end of its inactive day, local time), the number of quarter hours
  without a value in the active part of the period and the days they fall on (gaps anywhere, also in the
  middle; a day has its local wall-clock quarter hours, 92 on the spring clock change day); and the totals of all metering
  points of the community in the period (production, consumption, G.02 allocated, G.03 self coverage,
  P.01 surplus). A period longer than one year or ending after 2999, more than 10,000 entries, an invalid
  direction or a body above 4 MiB → 400.
- CI: PR checks (tests on every pull request) and a security scan (leaked secrets in new commits,
  vulnerable dependencies, Dockerfile/Kubernetes misconfigurations). Dependency and misconfiguration
  findings are only reported and do not block; a leaked secret in new commits fails the check. Jobs for
  known failing tests are report-only.
- CI: every run keeps its scan and test results as artifacts for 30 days (`scan-*`,
  `test-results-<job>`, `known-errors-<job>`) for a CI report collector; the secret scan keeps
  outcomes and counts only.

## [1.5.3] – 2026-10-17

### Changed
- Test fixtures: an unused spreadsheet removed, the Excel import test file replaced by a neutral
  copy with test metering point numbers (`TestImportExcelEnergyFile` unchanged).

### Fixed
- **Excel import: unzip size limit.** Workbooks are opened with a limit of 1 GiB on the total
  uncompressed size declared by their entries (excelize default: about 16 GB); opening stops at the
  first entry that crosses it. Uploads are capped at 32 MiB by the GraphQL multipart transport
  (gqlgen default); the workbooks in `test/` expand 7x to 27x, so 1 GiB (32x the cap) covers the
  largest observed ratio. Worksheets above 16 MiB still go to the temp directory as before.
- **Excel upload: temp files on failed opens.** An uploaded workbook (`OpenReader`) gets its own
  temp directory, removed when the import ends and also when opening fails or panics; before,
  temp files written before the failure stayed behind. `OpenExceFile` (tests only) keeps the
  default temp directory. The cause of a failed open is now logged.
  (`TestOpenReaderRejectsOversizedEntry`, `TestOpenReaderRejectsOversizedTotalAndCleansUp`,
  `TestOpenWorkbookWithinUnzipLimits`)

## [1.5.2] – 2026-10-05

### Fixed
- **Pool shutdown:** after `Pool.Close` a waiting `Get` is woken and gets nil instead of hanging,
  and no later `Get` reopens a database — not even for an ecId the pool had not seen yet. Handles
  still out may be returned afterwards. (`TestPoolCloseWakesWaitersAndRefusesGet`)
- **Lost wakeup in the pool:** a `Get` that was woken for a free handle but then failed to open the
  database returned without waking the next waiter, so that handle stayed unused while others
  waited. (`TestFailedOpenWakesNextWaiter`, which hangs without the fix)

## [1.5.1] – 2026-10-05

### Fixed
- **Connection pool: handing out and closing a database no longer race.** The per-database
  pool object decided "last handle returned, close the database" by looking at the fill level
  of a channel without the lock that `Get` held; a `Get` could hand out the database in exactly
  that window and the caller then wrote to a closed database. The pool map itself was written
  under one mutex and read under another. Handles are now counted under a single mutex per
  database, which also decides when to close, and one mutex guards the map; waiting for a free
  handle happens outside it, so a busy community no longer holds up the others. Relevant before
  the value-log GC is switched on, which shares the pool with the imports. Concurrency test
  `TestPoolConcurrentGetPut` (with `-race`); `TestOpenMaxObject` rewritten without its own race.
- **One failed store open no longer crashes the service.** When opening a store failed (invalid
  ecId, tenant longer than 8 characters), the importer set its whole store map to nil; the next
  message of that community wrote into the nil map and the panic took the service down, and with
  the broker's persistent session the redelivered message crashed it again after the restart.
  Now only the entry of that ecId is dropped and the message is reported as an error. As a second
  line of defence the tenant worker recovers from a panic in a single message and logs it.
  Tests `TestEnsureDbInvalidThenValid`, `TestEnsureDbTenantTooLong`.

## [1.5.0] – 2026-10-04

### Security
- GraphQL (`singleUpload`, `lastEnergyDate`): the `tenant` argument is now checked against the
  tenant that `GQLProtect` verified against the token (case-insensitive, `superuser` exempt); a
  mismatch is refused before any store is opened. The middleware passes the verified tenant on in
  the request context (new package `tenantctx`, no dependencies, so `graph` stays testable without
  Keycloak). The web sends the same tenant in header and argument and is not affected.
- **Tenant isolation: the connection pool was keyed by `ecId` alone.** A pool object opens its
  Badger store under `basePath/<tenant>/<ecId>`, using the tenant of whoever created the entry
  first. A later request for the same `ecId` under a *different* tenant received the first
  tenant's store back — cross-tenant read/write of energy data. The pool is now keyed by
  tenant **and** ecId (`poolKey`), so each tenant gets its own store.
- **`ecid` from the URL path was unvalidated before reaching `filepath.Join`.** A value
  containing `/` or `..` could escape the intended directory. `OpenStorage` now rejects any
  tenant or ecId that is not strictly alphanumeric, before it touches the filesystem. All
  real identifiers are alphanumeric and stay valid; every runtime caller goes through
  `OpenStorage`.
- The container no longer runs as root. The service listens on 8080, well above 1024, so the
  privileges were never needed. A dedicated `app` user (UID/GID 1000) owns `/opt/rawdata` and
  `/opt/energy`; the `chown` deliberately runs **before** the `VOLUME` instruction, because
  later changes to a declared volume do not end up in the image.
  The Badger data lives on a PVC, which is mounted `root:root` by default — the deployment
  therefore needs `fsGroup: 1000` alongside this. Order matters: `fsGroup` first, then the
  image, otherwise the service can no longer write to its data directory, and only at runtime.
  Note for production: the PVC there holds about 1.2 TB, and the kubelet walks the whole
  volume when applying `fsGroup`. `fsGroupChangePolicy: OnRootMismatch` keeps that from
  delaying every pod start.
- `google.golang.org/grpc` 1.83.1 -> 1.83.2 (Dependabot #40), patch release on top of the
  CVE-2026-84304 fix.

### Added
- **Value-log garbage collection** (#45), **off by default**. The production volume fills up by
  about 5 GiB a day: 96.6 % of it is Badger's value log, which only `RunValueLogGC` frees — and
  nothing called it. The cause is the community-wide metadata record `cpmeta/0`: above roughly
  750 metering points it exceeds the 128 KB `ValueThreshold` and every period extension writes a
  complete new copy into the value log (found by @artmanns).

  A background run inside the service (not a CronJob: Badger locks each directory exclusively, an
  external job could make imports fail) opens the largest databases one after another through the
  pool, calls `RunValueLogGC` until nothing is left or the budget is used, and logs the value-log
  size before and after for each database plus a summary line — all visible at `-v=3`.

  Configuration under `persistence.vlogGC`, also settable as `ENERGYSTORE_PERSISTENCE_VLOGGC_*`:
  `enabled` (false), `window` ("02:00-04:30", evaluated in Europe/Vienna whatever the container's
  time zone), `checkEvery` (10m), `discardRatio` (0.5), `probeRatio` (0.1, one diagnostic call when
  nothing qualifies), `minVlogMB` (100), `maxGBPerRun` (50, counted in bytes because Badger takes
  the largest files first). On shutdown the run is awaited before the pool closes.

  Safeguards: the directory of the database the pool actually returned is checked against the
  enumerated one, and for an ecId that exists under more than one tenant the run only works on a
  database the pool already holds under that tenant and never creates the pool entry itself.
  Value-log sizes leave out the newest file while it is pre-allocated at 2 GiB (database open); in
  a closed database it holds data and counts.

  A read-only diagnosis of production on 2026-09-11 found the whole backlog (1 087 GiB) already
  accounted as discardable, so the GC will reclaim it. Tests with a real Badger
  (`go test -run TestVlogGC ./store/ebow/`, now a separate CI step): 93 MB value log reclaimed to
  0 in 12 rewrites of ~6–10 ms each; one 976 MB file in 349 ms (local disk, warm cache).

  **Dry run** (`dryRun`, default false, env `ENERGYSTORE_PERSISTENCE_VLOGGC_DRYRUN`): with
  `enabled=true` and `dryRun=true` the run keeps its schedule, window and budget but opens no
  database. Per database it only reads `DISCARD` and the value-log file sizes and logs which files
  `RunValueLogGC` would rewrite (largest discard first, stopping at the first file under
  `discardRatio`, as Badger's `pickLog` does), how much it would read and the estimated space freed.
  All lines are tagged `vlogGC [DRY-RUN]`. Meant for checking production before switching the run
  on; run time, memory and the effect of open iterators only show in a real run. In the test the
  prediction matched the real run exactly (12 rewrites, 93 MB).
- CI builds `env/**` branches and deploys the resulting image into the matching feature
  environment (ADR-0008): a push to `env/<name>` pins this service in namespace `env-<name>`
  to that branch's `sha-…` image. Previously only the default branch, tags and `preview/**`
  produced an image at all. The environment itself is still provisioned manually.

## [1.4.0] – 2026-09-16

### Changed
- The energy export no longer writes a separate **"QoV Log" sheet**. The quality of every value
  is already visible in the "Energiedaten" sheet, where each cell carries the colour of its
  quality level; the extra sheet repeated the same values in twice the columns. Dropping it makes
  the export roughly **60 % faster and the file two thirds smaller** (benchmark, 250 consumers +
  40 producers over 31 days: 5.1 s / 18.3 MiB before, 2.1 s / 6.1 MiB after).

  So that nothing is lost, three things changed together:
  - **L0** (no measured value delivered) is now filled light grey instead of being written as a
    blank cell. Before, it was indistinguishable from "metering point not active in this period",
    which stays genuinely empty.
  - The **"Summary" sheet** shows, per metering point and quality level, the **number of affected
    quarter hours** instead of a plain marker, on the same colour the cell uses in the
    "Energiedaten" sheet — the overview doubles as the legend.
  - Each of those numbers carries a **cell comment listing the affected days**, so a metering
    point with both L0 and L3 keeps them apart: one comment per level instead of one merged range.

## [1.3.1] – 2026-09-16

### Fixed
- The energy export of a **large community** took about 11 minutes **regardless of the period** —
  even a single day — and ran into the ingress timeout of 600 s, so the file never arrived.
  Measured in production on 15.09. for a community with roughly 2,400 metering points: one day
  677 s, longer periods 688–694 s.

  The cause was the "QoV Log" sheet setting its column widths **one column at a time** (six
  columns per consumer, four per producer, alternating 25/5). excelize rebuilds and linearly
  searches the complete column list on every `SetColWidth` call, so the cost grows with the
  **cube** of the column count. Benchmark, one day: 500 metering points 6.2 s, 1,000 41.7 s,
  2,000 333 s — 91 % of it in `SetColWidth`.

  The data columns now get their width in a single call: 2,000 metering points 1.9 s. The file
  content is unchanged; the QoV columns are now as wide as the value columns (25) instead of
  alternating 25/5. The "Energiedaten" sheet also sets its width across all data columns
  instead of a fixed 1,000 (larger communities had columns without width). Smaller communities
  with a QoV sheet benefit as well.

## [1.3.0] – 2026-09-15

### Changed
- Two fixes to the energy export's Excel generation, found by @artmanns in #46. The file
  content is unchanged: same sheets in the same order, Summary still the active sheet, every
  cell value and column width identical (compared cell by cell). The only difference is that
  cells in the "QoV Log" sheet which had no style now carry the same neutral row style the
  "Energiedaten" sheet has carried since 1.2.1.

  **"QoV Log" sheet: neutral row style.** The same fix as 1.2.1, applied to the sheet it missed.
  excelize resolves a style for every cell by walking all column definitions of the sheet —
  for cells with their own style too, because it only sees the row style at that point. This
  sheet sets its widths one column at a time, so every cell walked one definition per column:
  quadratic in the number of metering points. excelize 2.9.1 introduced this walk into the
  streaming writer, which makes it the larger of the two steps of the 1.2.0 regression.
  The sheet is written whenever any active metering point has a quality value other than 1 in
  a row — including 0, a missing reading — so one metering point with gaps puts every row of
  the export into it.

  **Default sheet reused instead of deleted.** `DeleteSheet("Sheet1")` at the end made excelize
  parse every already-streamed sheet below 16 MB back into memory and encode it again. The
  Summary sheet now takes over the default sheet. This was already the case on excelize 2.8.1;
  it helps small and mid-size communities and does nothing for sheets above 16 MB.

  Measured with the benchmark (mocked storage, one month, every row in the QoV sheet):
  small 1.83s -> 0.52s, medium 3.83s -> 2.17s, large 15.97s -> 6.05s — the large case is back
  at its excelize 2.8.1 level (6.0s).

  Checked against a real production export (512 metering points, two months, 100 MB, 130s in
  production): the "QoV Log" sheet holds 92 % of all rows and 2 757 columns — 712 MB of the
  file's ~1 GB of sheet XML. The rows land there because of substitute (L2) and estimated (L3)
  values, not missing readings, so the sheet is written in normal operation. The benchmark in
  exactly that shape: 82.1s -> 25.7s. Mocked storage, so the production figure is an estimate
  — roughly 40s instead of 130s.

- The energy export no longer treats **L2 values as a quality issue**. L2 values are fine by
  now. A time slot with only L1 and L2 values no longer lands in the "QoV Log" sheet, and a
  metering point with only L1 and L2 values shows "data ok" in the Summary. L0 (no value) and
  L3 still count as issues, exactly as before. L2 values stay marked — yellow in the value
  sheets, the L2 column in the Summary — as information, not as a problem.

  Side effect on file size and export time: in a real production export (512 metering points,
  two months) 92 % of all rows were in the "QoV Log" sheet, most of them because of L2 values.
  Without L2 the sheet shrinks by 28 % on that file (5 492 -> 3 936 rows). It does not shrink
  further because one or two metering points without any readings (L0) put 3 360 rows in there,
  each a full-width copy of all metering points.

  Verified on Dev with a synthetic community of 900 metering points fed through MQTT like
  production (value log 981 MB → 134 MB, later 2.7 GB → 31 MB): the discard statistics build up
  by themselves, imports run alongside, a restart mid-run stops cleanly before the pool closes,
  the next pod finishes the job. Reading runs at **35–50 MB/s on network storage, and one call on
  a 1 GB file took 28 s** — close to Kubernetes' default 30 s grace period. Opening a database is
  not interruptible either: Badger reads the newest value-log file in full to check whether it
  needs truncating (`value.go:593`), which took 22 s for a 900 MB file; the per-database log line
  shows the open time. Raise `terminationGracePeriodSeconds` (e.g. to 90) before enabling this
  where 1 GB value-log files exist, as in production.

## [1.2.2] – 2026-09-09

### Fixed
- The energy export aborted with `write tcp …: i/o timeout` for larger communities, even though
  the file had been generated correctly. The cause was this service's own HTTP server:
  `WriteTimeout` was 180s, and in Go that covers the **entire handler plus writing the
  response**. Since the export builds the complete XLSX before the first byte is sent, the whole
  generation time counted against it. A community with 644 members needs about 218s, so it died
  38s short of the finish line — the user saw an error for a file that existed.

  Raised to 900s, deliberately **above** the ingress limit of 600s: that way the proxy is the
  one that cuts a run short, cleanly, rather than the application running into its own deadline
  mid-write. `ReadTimeout` stays at 180s — only the request is read there, which is fast even
  for a thousand members.

  This is headroom, not a fix. Roughly 1700 members' worth of monthly export at today's speed;
  a yearly export or further growth exceeds it too. The structural answer is to decouple the
  export from the request — see `konzept-async-energy-export.md`.

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
