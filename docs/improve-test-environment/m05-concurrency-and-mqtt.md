# M5 — Concurrency and MQTT robustness

**Concept:** phase 5 · **Status:** done 2026-10-03 · **Production code:** none
**Depends on:** M0 (CI with `-race`, builders), M3's import helpers; ES-12 (no broker library assumed).
**Effort:** 1.5 – 2 days.

## Goal

The ingest path and the storage pool are tested for the failures that cost data or availability:
crashes on bad input, swallowed store errors, races, one tenant blocking all.

## Set-up

- Fake `mqtt.Message` (Paho's interface: `Topic`, `Payload`, `Ack`, …) built with the CR_MSG builder;
  `mqttclient/dispatcher_test.go` already has one (`testMsg`) to move into `internal/testsupport`.
- Fake `mqtt.Client` (Paho's interface): importer and dispatcher take the concrete `*MQTTStreamer`, but its
  `client` field is the interface, so an in-package test builds `&MQTTStreamer{client: fake}`. The fake
  records `AddRoute` (gives the test the dispatcher's receiver) and `Publish` (`cr_msg_history` replies,
  `mqttClient.go:117`) and returns a completed token. No production change needed.
- Paho's auto-ack (#19) happens inside Paho after the callback returns, so the ack order itself is not
  testable with fakes; the tests check what is stored and what is published (concept §7.1).
- Deterministic concurrency: channels and `sync.WaitGroup` latches, no sleeps; every test under `-race`
  with `-count=20` once while writing it.

## Tests

| Test | Expectation | Finding |
|---|---|---|
| F3 defect test (scenario kept outside the repository) | details not published | **F3** (defect test) |
| importer: the store fails | error returned, no `cr_msg_history` "processed" | **F4** (defect test) |
| importer: undecodable payload | dropped with one error line, counted | F4 |
| F5 defect test (scenario kept outside the repository) | details not published | **F5** (defect test) |
| F6 defect test (scenario kept outside the repository) | details not published | **F6** (defect test) |
| F17 defect test (scenario kept outside the repository) | details not published | **F17** (defect test) |
| dispatcher: start, 100 messages, close | `Close` returns after every worker closed its store; no race | **F26** |
| two concurrent imports of two **new** metering points into one `ecId` | different `SourceIdx`, both stored | F18 (suspicion) |
| raw-data delete and import of the same day in parallel | the documented outcome, no lost slot outside the range | F19 (suspicion) |
| duplicate message (same slots twice) | idempotent | — |

## Tasks

- [x] Fake message and publisher helpers in `internal/testsupport`
- [x] Tests of the table; defect tests skipped with their numbers, each run red once
- [x] Floors raised (`mqttclient` ≥ 70 % target); `known-errors.md`, `AGENT_LOG.md`

## Acceptance criteria

- `go test -race -count=20 ./mqttclient/ ./store/ebow/` green (skipped defect tests excluded) and under 60 s.
- F3, F4, F5, F6, F17 each reproduced by a skipped test (red result recorded) or recorded as not reproducible.
- No test needs a broker, the network or a sleep.

## Risks

- The suspicions F18/F19 do not reproduce deterministically → the test stays as a regression test,
  the finding stays "suspicion".
- Race tests on today's pool crash the test binary (concurrent map write is fatal, not a test failure)
  → those defect tests run only with the skip removed, in a separate `go test -run` call.

## Result (2026-10-03)

- `internal/testsupport/fakemqtt.go`: fake `mqtt.Message` and `mqtt.Client` (routes, recorded publishes,
  a `Notify` latch instead of sleeps, a `Gate` to hold one topic). `mqttclient/dispatcher_test.go`
  (replaces the empty skeleton) and `concurrency_test.go`.
- Green: a stored message gets one `cr_msg_history` reply with ecId, meter, conversationId and the block
  dates; undecodable payloads are dropped without a reply or panic; a duplicate message is idempotent;
  the dispatcher routes per tenant; 100 messages for 5 tenants then `Close` closes every store.
- Reproduced (skipped, red runs recorded): #19 (F4), #33 (F18, was a suspicion), #34 (F19, was a
  suspicion). F26 (#41) not reproduced in 60 runs. The F3, F5, F6 and F17 cases are critical and
  security relevant: their tests are kept outside the repository (results recorded there).
- `go test -race -count=20 ./mqttclient/ ./store/ebow/` green in 22 s after fixing two old tests that
  were not repeatable (#54).
- Coverage: `mqttclient` 66 % (target 70 %: the rest is the broker connection `NewMqttStreamer`,
  `Connect`, `SubscribeTopic` and the unused `Dispatcher`/`Subscriber`). Total 68.0 %.
