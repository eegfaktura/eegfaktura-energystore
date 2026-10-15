# Contract fixtures (M6)

Copied from the neighbours on 2026-10-03 (open-points ES-21: byte-identical where possible, source commit
and sha256 recorded). Refreshed by hand — no job compares them with the neighbours (a remaining gap).
All three source repositories are AGPL-3.0 like this one.

| File | Source | Commit | sha256 | Copy |
|---|---|---|---|---|
| `web/energy.service.ts` | eegfaktura-web `src/service/energy.service.ts` | `8b52ed8` (HEAD `c37a0b7`) | `c3d3e75f8c8a782088e59cf005d7821c7f618a93bcfb7b142ecf447ec3837f51` | byte-identical |
| `web/graphql-query.ts` | eegfaktura-web `src/service/graphql-query.ts` | `bf26937` (HEAD `c37a0b7`) | `b1bb0ab595d5f168db7532918c65d09770e25e3952a8ee3c1bb8b2be65dd98b0` | byte-identical |
| `v3/EnergyStoreClient.kt` | eegfaktura-v3 `backend/src/main/kotlin/at/eegfaktura/integration/energystore/EnergyStoreClient.kt` | `677b8fb` (HEAD `0b785d2`) | `f07bbf057f51a4beffd409e3c2cbdc621b9b474aa9e5b40ff305b280bb941f67` | byte-identical |
| `v3/EnergyStoreWire.kt` | eegfaktura-v3 `…/integration/energystore/EnergyStoreWire.kt` | `b4d2382` (HEAD `0b785d2`) | `c3119d66659fa1678895fa8f303b052adb62fe5ca58c1e46e0c13262ed1c124d` | byte-identical |
| `v3/cr-msg-history.json` | eegfaktura-v3 `backend/src/test/resources/mqtt/cr-msg-history.json` (what v3's `CrMsgHistoryWriter` reads) | `f039b9b` (HEAD `0b785d2`) | `1a48e47f9d5f81c4255028c4afca4339c992a8967f0b0d28085d8a0930d64440` | byte-identical |
| `v3/mock-cr-msg.json` | built from v3's `energy-mock/…/channel/CrMessage.kt` at `769c44b` (keys, order, `messageCodeVersion` 01.40, `interval`, `nInterval`, exclusive `end`); one consumer, 2026-06-01, 96 slots | `769c44b` | — | **derived** (the mock builds it at run time) |
| `backend/masterdata-fields.txt` | eegfaktura-backend `proto/masterdata.proto` | `f4974b2` | — | **hand-written field list** (no LICENSE in the backend repository) |

The eda-xp payload is `test/energy-response-new-text.json` of this repository (its origin is not
recorded; its keys match eda-xp's `EbMsMessage` JSON: `messageId`, `sender`, `receiver`,
`messageCodeVersion`). eda-xp encodes CR_MSG as base64 of gzip of the JSON
(`src/main/scala/at/energydash/mqtt/MqttSystem.scala:209-214` at `e51492b`).

To check energystore-v2 with the same fixtures (ES-19), point `apitest.Router` at a v2 base URL — not
done here.
