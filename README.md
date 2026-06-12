# Energy Store

Energy Store is a Go-based service designed to manage, process, and store energy data. It provides both GraphQL and REST APIs for data interaction and supports MQTT for real-time energy data ingestion.

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

- Go 1.24 or later
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
