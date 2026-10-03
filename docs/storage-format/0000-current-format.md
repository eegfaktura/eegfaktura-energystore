# 0000 — The storage format as it is (2026-10-03)

Description of today's format, written in M3 of the test environment (`open-points.md` ES-7). It is
not a change and needs no migration; later concepts (AGENTS.md §8.1) refer to it. Each statement is
exercised by a test in `scenario/` or `store/`, named in brackets.

## Directories

- One Badger database per community: `<persistence.path>/<tenant in lower case>/<ecId>`
  (`store/ebow/pool.go` `OpenStorage`). Tenant at most 8 characters, tenant and `ecId` only
  `[A-Za-z0-9]` (`rowdata.go` `idPattern`) [`rest.TestInvalidEcIdCreatesNoDirectory`].
- A read of a valid but unknown `ecId` creates an empty database (known-errors #36)
  [`rest.TestUnknownEcIdCreatesNoDirectory`, skipped].

## Keys

- Bow, the wrapper in `store/ebow`, prefixes every key with a 2-byte bucket id; the bucket ids come
  from a Badger sequence under the key `[0x00, 0x00]` (`bucket.go` `internalKey`, `db.go`
  `bucketIdSequence`); the bow meta record (bucket names → ids) is JSON under `[0x00, 0x01]` (`metaKey`).
- Buckets used by the service: `rawdata` (energy lines) and `metadata` (`cpmeta/0`).
- Keys are strings encoded by `codec/key` as their bytes. Integer keys would not round-trip (known-errors
  #52) but none are stored.

## Row ids (bucket `rawdata`)

- `CP/yyyy/MM/dd/hh/mm/ss` — the **local wall-clock time** (Europe/Vienna rules) of the start of the
  quarter hour [`utils.TestRowIdWallClockInVienna`].
- Spring day: 92 ids, 01:45 is followed by 03:00 [`scenario.TestS02_SpringDayStores92Slots`].
- Autumn day: the 02:xx wall-clock hour exists twice but has one id each; the MQTT and the Excel import
  add the second hour into the first key — 96 ids, the daily total is kept (known-errors #25, #29)
  [`TestS03_AutumnDayTotalIsKept`, `TestS03_AutumnDayKeeps100Slots` skipped].
- A missing slot has no row; readers fill gaps themselves (known-errors #22) [`TestS04_*`].

## Meta record `cpmeta/0` (bucket `metadata`)

`model.RawSourceMeta`: `Id` `cpmeta/0`, `CounterPoints` (one `CounterPointMeta` per metering point),
`NumberOfMetering`. A `CounterPointMeta` holds `ID` (three digits), `Name` (the metering point),
`SourceIdx` (its index **within its direction**), `Dir` (`CONSUMPTION` / `GENERATION`), `Count`,
`PeriodStart`, `PeriodEnd` (`dd.MM.yyyy HH:mm:ss`; older records carry four-digit seconds, still read)
[`services.TestGetLastEnergyEntry`]. A new metering point gets max index + 1 of its direction.

## A raw line (bucket `rawdata`)

`model.RawSourceLine`, JSON-encoded by `codec/json` with the Go field names:

| Field | Layout |
|---|---|
| `Id` | the row id |
| `Consumers` | triples per consumer at `3·SourceIdx`: G.01 consumption, G.02 share of the community's production, G.03 own coverage |
| `Producers` | pairs per producer at `2·SourceIdx`: G.01 production, P.01 surplus |
| `QoVConsumers`, `QoVProducers` | quality per value, same positions: 1 = L1, 2 = L2, 3 = L3, 0 = none |

[`scenario.TestS01_OneConsumerOneProducer`, `TestS07_QualityIsStoredPerSlot`, `codec/json.TestRoundTrip`]

- Lines are as long as the highest index written so far; shorter lines are valid (missing positions
  read as 0).
- TF codes (G.01T, P.01T) replace the base value of the same position; G.03R is ignored
  [`TestS03_ExcelTFReplacesBase`].
- A later message overwrites a slot unconditionally, also with worse quality (known-errors #35,
  `open-points.md` ES-15) [`TestS07_LateMessageOverwrites`].

## Encoding note

The `msgp` codec exists but no store uses it. Values on disk are JSON; a change to field names, the
position rules or the row-id rule is a format change under AGENTS.md §8.1.
