# External sources

Every source outside this repository that the build, the tests or the running service depends
on. Adding a source requires a decision first (AGENTS.md section 12): record the question in
`open-points.md`, then add the row here in the same change. Licences were read from each
module's own `LICENSE` file in the local module cache on 2026-10-02.

## Build and toolchain

| Source | Used for | Version / licence | Trust / pinning |
|---|---|---|---|
| Go module proxy (`proxy.golang.org`) and checksum database (`sum.golang.org`) | all modules | — | official; `go.sum` pins every module hash |
| Go toolchain | build and tests | `go 1.25.0` in `go.mod`; BSD-3-Clause | the `go` line is a minimum, not a pin (local runs 2026-10-02 used go 1.26.0) |
| Docker Hub `golang:1.25` | build **and runtime** image (single stage) | BSD-3-Clause (Go), Debian | **floating tag** (`known-errors.md` #10, `open-points.md` ES-1, ES-3) |
| `protoc` (protobuf-compiler) | generating `protoc/*.pb.go` | 29.3 (released 2025-01-08); BSD-3-Clause | pinned in `scripts/dev/generate.sh`; CI downloads the release zip `protoc-29.3-linux-x86_64.zip` from GitHub |
| `google.golang.org/protobuf/cmd/protoc-gen-go`, `google.golang.org/grpc/cmd/protoc-gen-go-grpc` | protobuf/gRPC code generation | v1.36.11 (2025-12-12), v1.6.2 (2026-05-07); BSD-3-Clause / Apache-2.0 | `go install …@vX.Y.Z` in CI, checked by `generate.sh` |
| `golang.org/x/vuln/cmd/govulncheck` | CI vulnerability report (reporting only, ES-12) | v1.8.0 (2026-09-08); BSD-3-Clause (read from its `LICENSE`) | `go install …@v1.8.0` in CI |
| `actions/upload-artifact` | CI coverage artefact | v7.0.1 (2026-04-10); MIT | by SHA `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` |
| `github.com/99designs/gqlgen` (generator) | `graph/generated/` | MIT | same version as the runtime module |
| GitHub Actions in `docker-image.yml`: `actions/checkout@v4`, `actions/setup-go@v5`, `docker/metadata-action@v5`, `docker/login-action@v3`, `docker/build-push-action@v6`, `actions/attest-build-provenance@v1` | CI | MIT / Apache-2.0 | **by major tag** (#10) |
| GitHub Actions in `snyk.yml`: `actions/checkout` v4.2.2, `actions/setup-node` v4.1.0, `github/codeql-action/upload-sarif` v3.27.9; Snyk CLI from npm | SAST | MIT; Snyk CLI Apache-2.0 | by SHA; the Snyk CLI is installed unpinned |
| GitHub Container Registry `ghcr.io/vfeeg-development/eegfaktura-energystore` | where CI publishes the image | AGPL-3.0 | tags per branch, `sha-…`, `latest` |

## Go modules (direct requirements in `go.mod`)

| Module | Used for | Version | Licence |
|---|---|---|---|
| `github.com/dgraph-io/badger/v4` | storage | v4.9.1 | Apache-2.0 |
| `github.com/tinylib/msgp` | `store/ebow/codec/msgp` — compiled, but no store uses it (values are JSON) | v1.2.1 | MIT |
| `github.com/eclipse/paho.mqtt.golang` | MQTT client | v1.5.1 | EPL-2.0 / EDL-1.0 (dual) |
| `github.com/99designs/gqlgen`, `github.com/vektah/gqlparser/v2` | GraphQL server | v0.17.49, v2.5.16 | MIT |
| `github.com/gorilla/mux`, `github.com/gorilla/handlers` | REST router, CORS | v1.8.1, v1.5.2 | BSD-3-Clause |
| `github.com/coreos/go-oidc/v3` | token verification | v3.11.0 | Apache-2.0 |
| `github.com/golang-jwt/jwt/v4` | claims types | v4.5.2 | MIT |
| `github.com/golang/glog` | logging | v1.2.5 | Apache-2.0 |
| `github.com/spf13/viper`, `github.com/spf13/cobra` | configuration, CLI | v1.19.0, v1.9.1 | MIT, Apache-2.0 |
| `github.com/xuri/excelize/v2` | Excel import/export | v2.11.0 | BSD-3-Clause |
| `github.com/sony/sonyflake` | id generation | v1.2.0 | MIT |
| `google.golang.org/grpc`, `google.golang.org/protobuf` | gRPC client to the backend's master data | v1.83.1, v1.36.11 | Apache-2.0, BSD-3-Clause |
| `golang.org/x/net` | HTTP helpers | v0.56.0 | BSD-3-Clause |
| `github.com/stretchr/testify` | tests only | v1.11.1 | MIT |

Indirect modules are pinned by `go.mod`/`go.sum`; `go list -m all` lists them.

## Contract fixtures (tests only, M6)

| Source | Used for | Licence | Pinning |
|---|---|---|---|
| eegfaktura-web `src/service/energy.service.ts`, `graphql-query.ts` | `contract/testdata/web/` | AGPL-3.0 | commit and sha256 in `contract/testdata/README.md` |
| eegfaktura-v3 `EnergyStoreClient.kt`, `EnergyStoreWire.kt`, `cr-msg-history.json`; `CrMessage.kt` (derived fixture) | `contract/testdata/v3/` | AGPL-3.0 | as above |
| eegfaktura-backend `proto/masterdata.proto` | hand-written field list `contract/testdata/backend/` | no LICENSE file — not copied | commit `f4974b2` |

## Runtime services

| Service | Used for | Where configured |
|---|---|---|
| Keycloak (realm `EEGFaktura`, clients `at.ourproject.vfeeg.app` / `.api`) | token verification (discovery and keys at start), password grant for `/query/*` | `keycloak.json` / `KEYCLOAK_CONFIG` |
| MQTT broker (Mosquitto in the platform) | energy ingest (`mqtt.energySubscriptionTopic`), inverter topic, `cr_msg_history` replies | `config.yaml` `mqtt.*` |
| eegfaktura-backend gRPC (`services.master-server`) | active metering points (`protoc/masterdata.proto`) | `config.yaml` |
| Mail server gRPC (`services.mail-server`) | sending the Excel export | `config.yaml` |
| Persistent volume | Badger data (`persistence.path`, about 1.2 TB in production, 2026-09 figure) | deployment |

## Test data

| Source | Used for | Note |
|---|---|---|
| `test/*.xlsx`, `test/*.json` | Excel and report tests | in the repository; origin of the real-looking files not recorded |
| generated in the test | `mqttclient.TestMassImport` | three days built by `internal/testsupport.CrMsg` (was `../energy-mass-test-data.json`, in no repository, `known-errors.md` #5) |
| `scenario/testdata/v3world/crmsg.jsonl.gz` | scenario S13 (`s13_v3world_test.go`) | CR_MSG payloads generated by eegfaktura-v3's `energy-mock` (AGPL-3.0, same organisation, commit `0b785d2`, seed 7), recorded off a local broker 2026-10-08, decoded; fictional data; sha256 `ce7fd35105b7a71d…`; provenance and refresh in `testdata/v3world/README.md` |
