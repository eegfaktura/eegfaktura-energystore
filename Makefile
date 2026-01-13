# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
DOCKER=docker

GOPATH := ${PWD}/..:${GOPATH}
export GOPATH

DOCKER_TAG=v0.3.03
BINARY_NAME=energy-store
ORGANISATION=vfeeg-development
GLOBAL_ORG=eegfaktura

all: test build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v -ldflags="-s -w"
estore:
	$(GOBUILD) -o estore -v -ldflags="-s -w" estore.go
test:
	$(GOTEST) -v ./...
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

docker-clean:
	$(DOCKER) rmi ghcr.io/$(ORGANISATION)/energy-store:$(DOCKER_TAG)

docker:
	$(DOCKER) build -t ghcr.io/$(ORGANISATION)/energy-store:$(DOCKER_TAG) .

push: docker
	$(DOCKER) image tag ghcr.io/$(ORGANISATION)/energy-store:$(DOCKER_TAG) ghcr.io/$(GLOBAL_ORG)/energy-store:latest
	$(DOCKER) push ghcr.io/$(ORGANISATION)/energy-store:$(DOCKER_TAG)
	$(DOCKER) push ghcr.io/$(GLOBAL_ORG)/energy-store:latest

protoc: protoc/masterdata.proto protoc/excel.proto
	protoc --experimental_allow_proto3_optional=true --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative ./protoc/*.proto
