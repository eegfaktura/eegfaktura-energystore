# Build stage
FROM golang:1.19 AS builder

ENV TZ="Europe/Berlin"

WORKDIR /usr/src/app

# Pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Build the Go binaries
# -s skip symbol table
# -w skip DWARF
RUN go build -o /usr/local/bin/energystore -ldflags="-s -w" ./
RUN go build -o /usr/local/bin/initQoV -ldflags="-s -w" ./initqov
RUN go build -o /usr/local/bin/ebowctl -ldflags="-s -w" ./ebowctl
RUN go build -o /usr/local/bin/estore -ldflags="-s -w" ./estore

# Runtime stage
FROM golang:1.19

ENV TZ="Europe/Berlin"

WORKDIR /usr/src/app

# Copy the built binaries from the builder stage
COPY --from=builder /usr/local/bin/energystore /usr/local/bin/energystore
COPY --from=builder /usr/local/bin/initQoV /usr/local/bin/initQoV
COPY --from=builder /usr/local/bin/ebowctl /usr/local/bin/ebowctl
COPY --from=builder /usr/local/bin/estore /usr/local/bin/estore

COPY config.yaml /etc/energystore/

VOLUME /opt/rawdata

EXPOSE 8080

CMD ["energystore", "-configPath", "/etc/energystore/", "-logtostderr=true", "-stderrthreshold=INFO"]