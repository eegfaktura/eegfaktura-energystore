# M6 — Contracts with the neighbours

**Concept:** phase 6 · **Status:** done 2026-10-03 · **Production code:** none
**Depends on:** M0, M2's router helper, M5's fake `mqtt.Client` (for the `cr_msg_history` reply); ES-21 (source and copy rule of the fixtures); read access to
eegfaktura-web, eegfaktura-v3, eda-xp and eegfaktura-backend. **Effort:** 1.5 – 2 days.

## Goal

A change on either side of an interface — a path, a field, the MQTT payload, the master-data proto —
turns a test red here, with a message that names the caller and the line.

## Inventory (to be counted in the first task)

- **eegfaktura-web** (`src/service/energy.service.ts`, `graphql-query.ts`; repository HEAD `c37a0b7`, the files last
  changed in `8b52ed8`): all `POST` — `report`, `intra-day-report`, `load-curve-report`, `combined-report`,
  `summary`, `raw` (meters in the body), `excel/report/download`, GraphQL `lastEnergyDate` and
  `singleUpload` (tenant in the `X-Tenant` header **and** the argument). The `GET` variants of
  `load-curve-report`/`combined-report`, `meta`, `rawdata/delete`, `excel/export` and `lastRecordDate` are
  not called by the web.
- **eegfaktura-v3** (`backend/…/integration/energystore/EnergyStoreClient.kt`, `EnergyStoreWire.kt`; repository HEAD
  `0b785d2`, the package last changed in `498e008`): `GET meta`, `GET lastRecordDate`, `POST raw` (body
  **and** `cp` query parameters), `POST report`, `summary`, `intra-day-report`, `load-curve-report`,
  `rawdata/delete` — eight calls, `X-Tenant` header.
- **MQTT input**: eda-xp's `cr_msg` publication and v3's `energy-mock` (`CrMessage`): topic
  `eda/response/<tenant>/protocol/cr_msg`, payload (base64, gzip, JSON with `conversationId`,
  `messageCode`, `meter`, `energy[]`, `ecId`); the reply on `cr_msg_history`.
- **gRPC**: `protoc/masterdata.proto` against eegfaktura-backend's copy.

## Tests

| Test | What it checks |
|---|---|
| `CallerEndpointContract` | every call of the inventory (method + path template) matches a route of the router (M2's walk); message names the caller file and line |
| `CallerRequestContract` | each caller's request body fixture decodes into the handler's request type without loss; unknown fields reported |
| `CallerResponseContract` | snapshots of the response fields the callers read (`EnergyStoreWire.kt`, web types); a removed or renamed field fails |
| `CrMsgContract` | eda-xp's and the mock's message fixtures import into a temp store with the expected slots and values; the history reply has the fields eda-xp reads |
| `MasterdataProtoContract` | the `.proto` here equals the backend's (or the differences are listed and accepted) |

Fixtures are copied byte-identical where possible with the source commit id and sha256 in a README
(ES-21); refreshed by hand — no CI job diffs them against the neighbours (recorded as a remaining gap).

## Tasks

- [x] Inventory counted (calls per caller, fields read); written into the fixtures README
- [x] Fixtures copied with commit ids; tests of the table
- [x] A throw-away change on each side once (rename a path, drop a JSON field, change the payload) → red with a useful message; restored
- [x] `known-errors.md` (contract gaps found), `AGENT_LOG.md`

## Acceptance criteria

- Every call of the inventory is covered; the counts are in `AGENT_LOG.md`.
- The four throw-away checks are red with a message naming the caller.
- The same fixtures can be pointed at energystore-v2 (documented, not run here — ES-19).

## Risks

- `model/mqtt_test.go` (M0) and eda-xp disagree on the payload → the eda-xp fixture is the truth; the
  difference becomes a `known-errors.md` row.
- v3 sends `cp` query parameters v1 ignores → pinned as "tolerated", not as a defect.

## Result (2026-10-03)

- Inventory (counted by the tests from the copied sources): eegfaktura-web **9** calls in
  `energy.service.ts` (7 REST, `/query` for `lastEnergyDate` and `singleUpload`), eegfaktura-v3 **8** calls
  in `EnergyStoreClient.kt`. Fixtures and their commits/sha256 in `contract/testdata/README.md`; the backend
  proto is a hand-written field list (no LICENSE in eegfaktura-backend), the mock CR_MSG is derived from
  `CrMessage.kt`.
- Tests: `contract/endpoints_test.go` (`TestCallerEndpointContract`, `TestGraphQLContract`),
  `contract/dto_test.go` (`TestCallerRequestContract` — v3's request classes parsed from
  `EnergyStoreWire.kt`, decoded strictly; web bodies checked against their source lines;
  `TestCallerResponseContract` — every field of 12 v3 response classes is a key of the real JSON),
  `contract/proto_test.go` (`TestMasterdataProtoContract`), `mqttclient/contract_test.go`
  (`TestCrMsgContract` — eda-xp's and the mock's payloads import with their sums; the `cr_msg_history`
  reply has the keys v3's `CrMsgHistoryWriter` reads and none its fixture lacks).
- Throw-away checks (each red, then restored): a renamed web path ("no route for eegfaktura-web
  src/service/energy.service.ts:124 POST …/summaries"), a renamed v3 response field ("EnergyStoreWire.kt:41
  EsRecord.consumed is not in the response"), a renamed `meterCode` in the mock payload (nothing stored),
  a changed field type in the proto list (diff of the field line).
- No contract defect found. Recorded: v3 sends the meters also as `cp` query parameters (ignored by v1,
  tolerated); the reply omits `meter.direction` when the message has none (v3 does not read it); the
  master-data request carries no `ecId` (#38). The web's response types are not checked (TypeScript types
  not parsed) — a remaining gap, as is the manual refresh of the fixtures.
- Energystore-v2 (ES-19): the same fixtures can be pointed at v2 by replacing `apitest.Router` with an
  HTTP client against a v2 base URL; not done here.
- Coverage total 68.2 %.
