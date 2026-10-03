# M4 — Refactoring for testability

**Concept:** phase 4 · **Status:** **blocked, optional** (`open-points.md` ES-18) · **Production code:** yes
**Depends on:** M0 – M3 complete (the scenarios are the safety net), M5's tests for the pool, ES-18 go;
ES-17 for the ack change. **Effort:** 4.5 – 6.5 days (sum of the steps below).

## Goal

Remove the obstacles of concept §5 so the core rules become fast unit tests, and fix the high findings
whose clean fix needs the new structure.

## Steps

| Step | Content | Findings | Effort |
|---|---|---|---|
| 4a | **Clock, base path and connections injected**: a clock parameter for intraday/engine; the storage base path and opener passed in instead of read from Viper inside `store/ebow`; the gRPC connections (master data, mail) passed in | T8, §5 | 0.5 – 1 day |
| 4b | **`calculation` on `IBowStorage`**; bucket and day-split rules as named pure functions (then table tests in milliseconds; the existing mock works) | F8, F27 | 1 – 1.5 days |
| 4c | **Pool rewrite** (fix design of F5/F6 kept outside the repository), idle eviction | F5, F6, F21 | 1 day |
| 4d | **Importer**: the F3 fix (design kept outside the repository); `Import` returns store errors; no `cr_msg_history` on failure; manual ack after the store (ES-17); `recover` per message with a dead-letter log line | F3, F4, F17, F26 | 1 – 1.5 days |
| 4e | **Iterator errors** checked in every reader; `Close` always closes; `Open` closes Badger on failure | F16, F30 | 0.5 day |
| 4f | `restServer.go` split per route group; dead code removed | ES-6, F31 | 0.5 – 1 day |

Each step is its own pull request with the full suite under `-race`; the skipped defect tests of the
findings it fixes are enabled in the same change.

## Out of scope

Business rule changes (ES-14/15/16): the refactoring keeps today's numbers; the scenarios prove it.
The move to energystore-v2 (ES-19).

## Acceptance criteria

- All S1 – S12 and M1/M2/M5 tests green before and after each step without changing an expected value.
- The defect tests of the fixed findings enabled and green; `grep 'known-errors #NN'` finds none of them.
- `loc-check.sh`: no red file among the touched ones.
- `CHANGELOG.md` names the behaviour changes callers see (4d: redelivery on failure).

## Risks

- 4d changes what eda-xp sees (redelivered messages) → agreed first (ES-17).
- 4c touches the hottest path of production (1.2 TB, all communities) → a load comparison before and
  after, only with the user's go (no benchmark without go).
