# Energy Store

Energy Store is a Go-based service designed to manage, process, and store energy data. It provides both GraphQL and REST APIs for data interaction and supports MQTT for real-time energy data ingestion.

Part of the **eegfaktura** suite — an open-source billing and management platform
for Austrian renewable energy communities (*Erneuerbare-Energiegemeinschaften*, EEG).
It ingests interval consumption/production data arriving over the EDA pipeline via
MQTT and serves the aggregations used for billing and visualisation.
(Container image: `energy-store`.)

**Tech stack:** Go · BadgerDB · Gorilla mux (REST) · gqlgen (GraphQL) ·
Eclipse Paho (MQTT) · Viper · Cobra · Keycloak/OIDC (JWT).

## Features

- **Data Management**: Store and retrieve energy readings and related metadata.
- **GraphQL API**: Flexible data querying using GraphQL (powered by `gqlgen`).
- **REST API**: Standard RESTful endpoints for energy data operations.
- **MQTT Integration**: Real-time data streaming and processing via MQTT.
- **Storage**: Uses `badger` DB for efficient data storage.
- **Excel Support**: Import and export energy data using Excel files.
- **Calculations**: Built-in modules for EEG (Renewable Energy Community) calculations and energy allocations.

## Getting Started

### Prerequisites

- Go 1.26 or later
- Access to an MQTT broker (for real-time features)

### Installation

1. Clone the repository.
2. Navigate to the project directory:
   ```bash
   cd at.ourproject/energystore
   ```
3. Download dependencies:
   ```bash
   go mod download
   ```

### Configuration

Configuration is handled via `config.yaml` or environment variables. You can specify the config file path using the `-configPath` flag.

Key settings: `persistence.path` (BadgerDB directory), `mqtt.host` / `mqtt.id` /
`mqtt.qos`, the subscription topics `mqtt.energySubscriptionTopic`
(e.g. `eda/response/+/protocol/cr_msg`) and `mqtt.inverterSubscriptionTopic`,
`jwt.pubKeyFile`, and `services.mail-server`. Exposed port: **8080** (HTTP).

**Dependencies:** an MQTT broker (mosquitto) for data ingestion, Keycloak for JWT
validation, and on-disk BadgerDB storage.

### Operations: CR_MSG delivery

energystore subscribes to the energy data messages (CR_MSG) with QoS 1 in a persistent session (fixed client id
`mqtt.id`, clean session off) and acknowledges a message only after it has been processed. A message that was
still in process when energystore stopped is delivered again by mosquitto when energystore reconnects. This needs:

- mosquitto with `persistence true`, otherwise the session and its queued messages are lost when the **broker**
  restarts;
- mosquitto `max_queued_messages` (default 1000 per client) raised, or `0` for no limit: while energystore is
  offline or slow, mosquitto queues QoS 1 messages for its session only up to this number and **drops further
  ones silently** (no log on the energystore side; the gap shows in the check before billing). With the manual
  ack a large request queues at the broker instead of in energystore, so this has to be raised before that
  release;
- a unique `mqtt.id` per running energystore; two instances with the same id disconnect each other;
- mosquitto sends at most `max_inflight_messages` (default 20) unacknowledged messages at a time, so a large burst
  is delivered more slowly instead of being buffered in energystore. Raise it (proposal 1,000): the messages
  waiting in energystore are what it batches, and with 20 the batches stay at one or two messages. The setting is
  global in mosquitto.conf and applies to every client (energystore connects with MQTT 3.1.1, so it cannot set
  its own limit); for the clients that acknowledge right away raising it changes nothing in practice.

Read the current values of both settings in prod and pilot before changing them (the mosquitto configuration
there is not in git).

Import settings: `import.parallel` (env `ENERGYSTORE_IMPORT_PARALLEL`, default 4) tenants are imported at the same
time, each in batches of at most `import.batchMax` messages (env `ENERGYSTORE_IMPORT_BATCHMAX`, default 50); a
tenant with more messages queues up again behind the other waiting tenants. `import.maxOpen` (env
`ENERGYSTORE_IMPORT_MAXOPEN`, default 32; a value below `import.parallel` is raised to it) is a hard limit on the tenants whose stores
are open for the import (a store being closed still counts): an open store holds about 90–112 MB, so the default
bounds them to about 3–3.5 GB, plus the working memory of the running imports. At the limit a lane first closes
the longest unused idle store (about 0.3 s) or waits until a store becomes idle, so many communities delivering
at once are imported more slowly instead of filling the memory. All three are read once at start.

A message that is stored, cannot be stored, cannot be decoded or causes a recovered panic is acknowledged and not
delivered again. A panic in a batch costs only the message that causes it: the batch is then run message by
message, and the other messages are stored and replied. Only stored data gets the `cr_msg_history` reply
(„Energiedaten empfangen“ in the backend). Log
searches (e.g. Dynatrace): `energy data not stored`, `cannot decode record`, `Error decoding`,
`panic while executing message`, and OOMKilled restarts of the pod.

**Crash loop right after start:** a message that kills the process outright (e.g. out of memory) is delivered again
after every restart. To get out of it, stop energystore (scale to 0), then discard its session at the broker by
connecting once with the **same client id** and a clean session, without subscribing, e.g.
`mosquitto_pub -h <host> -p <port> -i <mqtt.id> -t energystore/session-reset -n` with host and port from
`mqtt.host` (mosquitto_pub uses a clean session by default). Then start energystore again. This drops **every**
message queued for energystore, not only the faulty one, and between the reset and the restart there is no
session, so messages published then are not kept either. The loss window runs from the first crash until
energystore is connected again: note both times and request the energy data of that period again (check before
billing).

### Running the Service

You can run the server using the provided `Makefile`:

```bash
make run
```

Or manually:

```bash
go run server.go -configPath ./config.yaml
```

The service will be available at `http://localhost:8080`.

### Building

To build the project binary:

```bash
make build
```

To build the `estore` CLI tool:

```bash
make estore
```

## API Documentation

- **GraphQL**: Access the GraphQL playground at `http://localhost:8080/query` (requires proper authentication headers if enabled).
- **REST**: Endpoints are defined in the `rest` package.

## Development

- **Testing**: Run tests using `make test` or `go test ./...`.
- **Docker**: Build the Docker image using `make docker`.

## Project Structure

- `calculation/`: Energy and EEG calculation logic.
- `cmd/`: CLI commands.
- `graph/`: GraphQL schema and resolvers.
- `middleware/`: HTTP middlewares for authentication and logging.
- `model/`: Data structures and models.
- `mqttclient/`: MQTT subscriber and dispatcher logic.
- `rest/`: REST API implementation.
- `store/`: Data persistence layer (BadgerDB/ebow).

## License

GNU Affero General Public License v3.0 (AGPL-3.0) — see [`LICENSE`](LICENSE).
