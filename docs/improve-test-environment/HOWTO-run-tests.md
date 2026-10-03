# How to run the tests here

State after M0 (2026-10-03), verified on a host with Go 1.26.0 (`go.mod` asks for 1.25.0), protoc
29.3, protoc-gen-go v1.36.11 and protoc-gen-go-grpc v1.6.2 under `$HOME`, no `make` (`make test`
would run `go test -v ./...`, which fails on the root package).

```bash
bash scripts/dev/test.sh                        # every package but the root, -race, coverage → cover.out
bash scripts/dev/coverage-check.sh cover.out    # per-package floors (scripts/dev/coverage-floors.txt)
bash scripts/dev/static-check.sh                # gofmt + go vet, the recorded findings #11/#47 allowed
```

- The generated stubs are committed. Only after changing a `.proto`: `bash scripts/dev/generate.sh`
  (checks the tool versions; install lines in its header) and commit `protoc/`.
- The root package (`server.go`, `estore.go`, `initQoV.go`: three `func main`, known-errors #1) is
  left out by `scripts/dev/packages.sh`; build the binaries one file at a time
  (`go build -o energystore server.go`). Moving the entry points (ES-22) waits for ES-24.
- Expected: green. `calculation` takes about 13 s, about 145 s under `-race` (timeout 300 s).
- Defect tests are skipped with `t.Skip("known-errors #NN")`; list them with
  `grep -rn 'known-errors #' --include=*_test.go .`. To see one red: comment out the skip line and
  run it with `-run` (some race tests crash the binary, see M5).
- One package: `go test -count=1 ./store/`; one test: `-run 'TestName'`; all but one: `-skip 'TestName'`.
- Time zone: every package with time-dependent tests sets `Europe/Vienna` in `TestMain`
  (`internal/testsupport/tz`), so `TZ` of the host does not matter (CI runs with `TZ=UTC`).
- Badger data lives only in `t.TempDir()` (`internal/testsupport.TempStore`/`TestDriverStore`); a run
  leaves nothing in the tree (`git status --short --ignored`).
- Coverage by hand: `go tool cover -func=cover.out | tail -1` or `go tool cover -html=cover.out`.
- Do not run the benchmarks (`excel/ExportBench_test.go`) or mass imports unless asked.
- `docker build` works from a checkout (the stubs are committed).
- Never `git add -A`.

## HTTP tests (`rest`)

`rest` imports `middleware`, whose package `init()` runs two OIDC discoveries (known-errors #12). The
tests import `internal/testsupport/fakeoidc`, whose own `init()` runs first and starts a fake Keycloak
on a loopback port (see m02 "Result"); `go test ./rest/` needs nothing else. Do not add non-standard
imports to `fakeoidc` — `rest.TestInitOrder` fails if the order breaks. Private tests of critical
security findings are not in the repository; they are run with `go test -overlay <private overlay.json>`.
