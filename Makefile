# Shortcuts for the scripts CI runs (scripts/dev/*.sh). The root package holds two func main
# (known-errors #1), so each binary is built from its file.
.PHONY: build test check generate

build:
	go build -o energystore -ldflags="-s -w" server.go
	go build -o estore -ldflags="-s -w" estore.go
test:
	bash scripts/dev/test.sh
check:
	bash scripts/dev/static-check.sh
generate:
	bash scripts/dev/generate.sh
