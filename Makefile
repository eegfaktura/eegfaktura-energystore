# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=energystore
DOCKER=docker

GOPATH := ${PWD}/..:${GOPATH}
export GOPATH

DOCKER_TAG=v0.1.0

all: test build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v -ldflags="-s -w"
test:
	$(GOTEST) -v ./...
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

docker-clean:
	$(DOCKER) rmi ghcr.io/vfeeg-development/energy-store:$(DOCKER_TAG)

docker: docker-clean
	$(DOCKER) build -t ghcr.io/vfeeg-development/energy-store:$(DOCKER_TAG) .

push: docker
	$(DOCKER) push ghcr.io/vfeeg-development/energy-store:$(DOCKER_TAG)

protoc:
	protoc --experimental_allow_proto3_optional=true --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative ./protoc/*.proto
