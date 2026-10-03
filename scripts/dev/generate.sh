#!/usr/bin/env bash
# Generates the protobuf/gRPC stubs in protoc/ with pinned tool versions (known-errors #1,
# open-points ES-2). The generated files are committed; CI runs this script and then
# `git diff --exit-code protoc/` so a drift between .proto and .pb.go turns the build red.
#
# Pinned versions (publish dates checked 2026-10-03, all older than 7 days, AGENTS.md 12.1):
#   protoc              29.3     (github.com/protocolbuffers/protobuf release v29.3, 2025-01-08)
#   protoc-gen-go       v1.36.11 (google.golang.org/protobuf, 2025-12-12)
#   protoc-gen-go-grpc  v1.6.2   (google.golang.org/grpc/cmd/protoc-gen-go-grpc, 2026-05-07)
#
# Install the plugins with:
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
# protoc itself: the release zip protoc-29.3-linux-x86_64.zip, bin/ on PATH.
set -euo pipefail
cd "$(dirname "$0")/../.."

PROTOC_VERSION="29.3"
PROTOC_GEN_GO_VERSION="v1.36.11"
PROTOC_GEN_GO_GRPC_VERSION="1.6.2"

export PATH="${HOME}/go/bin:${HOME}/.local/opt/protoc/bin:${PATH}"

check() {
  local name=$1 want=$2 got=$3
  if [ "$got" != "$want" ]; then
    echo "generate.sh: $name is '$got', expected '$want' (see the header of this script)" >&2
    exit 1
  fi
}

check protoc "libprotoc ${PROTOC_VERSION}" "$(protoc --version 2>/dev/null || true)"
check protoc-gen-go "protoc-gen-go ${PROTOC_GEN_GO_VERSION}" "$(protoc-gen-go --version 2>/dev/null || true)"
check protoc-gen-go-grpc "protoc-gen-go-grpc ${PROTOC_GEN_GO_GRPC_VERSION}" "$(protoc-gen-go-grpc --version 2>/dev/null || true)"

protoc --experimental_allow_proto3_optional=true \
  --proto_path=. \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  ./protoc/*.proto
