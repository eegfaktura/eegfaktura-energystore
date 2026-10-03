# M2 — HTTP layer, GraphQL and the tenant matrix

**Concept:** phase 2 · **Status:** done 2026-10-03 (fallback without a production change, ES-24) · **Production code:** one enabling change (ES-13):
`middleware`'s `init()` becomes `InitKeycloak()`, called by `main` — no behaviour change at run time
**Depends on:** M0; ES-13 (and ideally the F1 fix merged first); ES-20 for the nested-claim case.
**Effort:** 2 – 2.5 days.

## Goal

Every entry point is tested for authentication and tenant isolation, so that a defect like F1 cannot
stay unnoticed again; the HTTP error behaviour is pinned.

## Entry points (counted 2026-10-02)

13 × `ProtectApp` (`rest/restServer.go:28-40`): `POST report`, `GET meta`, `POST raw`,
`POST rawdata/delete`, `POST intra-day-report`, `POST`/`GET load-curve-report`,
`POST`/`GET combined-report`, `POST summary` (all `/eeg/v2/{ecid}/…`), `GET /eeg/{ecid}/lastRecordDate`,
`POST /eeg/{ecid}/excel/export/{year}/{month}`, `POST /eeg/{ecid}/excel/report/download`.
2 × `ProtectApi` (`rest/energy.go:25-26`): `POST /query/rawdata`, `POST /query/{ecid}/metadata`.
2 GraphQL fields behind `GQLProtect` (`/query`): `lastEnergyDate`, mutation `singleUpload`.

## Set-up

- **Why a production change or a script is needed.** `middleware`'s `init()` runs two OIDC discoveries
  (`api` client `authentication.go:71`, `app` provider `:115`) and panics without them. Package `init()`s
  run before `TestMain`, so a test cannot set `VerifyTokenClaims` or a URL in time (`known-errors.md` #12).
- **With ES-13:** `InitKeycloak(cfg)` is called by `main` only; tests never call it and set
  `VerifyTokenClaims`; the `ProtectApi` tests call it with a config that points at an `httptest.Server`.
- **Fallback without ES-13 (no production change):** `scripts/dev/test.sh` starts a small fake OIDC
  server on a loopback port before `go test` and stops it afterwards. It serves, per realm,
  `/realms/<realm>/.well-known/openid-configuration` (the `issuer` must equal that URL, go-oidc checks
  it), a JWKS with a test key and a token endpoint that answers the password grant with an ID token
  signed by that key. `KEYCLOAK_CONFIG` is an **absolute** path (the default `./keycloak.json` is
  resolved per package directory) to a test config with `api` and `app` entries and no secrets. Signing
  the ID token needs a JOSE library: `go-jose/v4` is already in `go.sum` (indirect via go-oidc), so no
  new source, but `go.mod` marks it direct. A plain
  `go test ./rest/` without the script panics; the HOWTO says so. There is no in-process alternative.
- Router: `rest.NewRestServer()` plus `/query` exactly as `server.go` wires it (a small test helper,
  because `server.go` is `package main`; `graph` itself does not import `middleware`).
- Tokens: `middleware.VerifyTokenClaims` (set in `TestMain`) returns claims built in the test (`tenant`,
  realm roles, email); "invalid token" = the hook returns an error. No key pair needed.
- `ProtectApi` does not use the hook: it always runs the password grant against the client from
  `init()` (unexported `kcClientAPI`), so its token endpoint is the fake server's (fallback) or the
  `httptest.Server` passed to `InitKeycloak` (ES-13).
- Storage: `t.TempDir()` with a small imported data set (M0 builders) so 200 responses carry data.

## Matrix

Applied per entry point where it makes sense (e.g. "invalid body" only where a body is read):

Status codes read from `middleware/authentication.go` (`verifyRequest` :221-255, `GQLProtect` :185-219)
and `api_authentication.go` (:15-63) on 2026-10-02:

| Case | `ProtectApp` | `GQLProtect` | `ProtectApi` | Note |
|---|---|---|---|---|
| no `Authorization` | 403 | 403 | 403 | pinned contract |
| wrong scheme | 403 | 403 | **400** | |
| invalid token / failed password grant | 401 | 401 | 403 | 401 body carries the verifier text (F25, recorded) |
| own tenant (header) | 200 | 200 | 200 | the comparison ignores case (`jwt.go:179-186`) |
| foreign tenant in the header | 403 | **401** | 403 | inconsistency recorded, not changed |
| no tenant header | 403 | 403 | 403 | App/GQL read `tenant`, then `X-Tenant`; `ProtectApi` reads only `X-Tenant` |
| `superuser`, foreign tenant | 200 | 200 | **403** | `ProtectApi` has no `superuser` bypass |

Handler-level cases, applied where they make sense:

| Case | Expectation today | Note |
|---|---|---|
| GraphQL tenant isolation case | refused | **F1** (#16) — details not published; the test case is kept outside the repository |
| invalid `ecId` (`a-b`, `a.b`, 40 characters) | an error status (today 400 or 500 by route), no directory under the temp root | `rowdata.go:113-124`; a path with `..` is redirected by mux before any handler |
| unknown but valid `ecId` | today: creates a directory (F21, defect test) | |
| invalid JSON / query (`start=abc`) | 400 | F22 (defect test for the GET reports) |
| `rawdata/delete` without `superuser` | 403 | |
| `lastRecordDate` without data | exactly one status and one body | F2 (defect test) |
| `/query/rawdata` | targets = requested meters only | F23 |
| Basic credentials whose standard base64 contains `+` or `/`; a password containing `:` | accepted | F24 (defect test; today decoded with `URLEncoding`, split at every `:`) |

A guard test walks the router (`mux.Router.Walk`) and fails if a route exists that the matrix table
does not list — a new route without tests is red.

## Tasks

- [x] Fallback with the fake OIDC server — in-process instead of a script (see Result); ES-13 not done (ES-24)
- [x] Test router helper, claims builder, fake token endpoint
- [x] Matrix per entry point; route-walk guard
- [x] Defect tests skipped with their numbers, each run red once
- [x] Floors raised; `known-errors.md`, `open-points.md`, `AGENT_LOG.md`

## Acceptance criteria

- 17 of 17 entry points in the matrix; the guard fails when a route is added on a throw-away branch.
- `rest`, `middleware`, `graph` (hand-written) ≥ 80 % statements (target).
- No test needs Keycloak, a broker or a non-loopback address; the package tests run in under 20 s.
- `git diff --stat` outside `_test.go` and `scripts/dev/`: only the `InitKeycloak` change and `server.go`'s call (none with the fallback).

## Risks

- `POST excel/export/{year}/{month}` mails the workbook through the gRPC mail server (`utils/admin.go:16`)
  and `POST /query/rawdata` without meters asks the backend's master data (`services/apiService.go:17`).
  Both dial an address from Viper inside the function, so `bufconn` cannot be used: a fake gRPC server
  on `127.0.0.1:0` with `viper.Set("services.mail-server"/"services.master-server", addr)`.
- The fallback script is one more moving part; if it proves fragile, ES-13 is the way out.

## Result (2026-10-03)

- **No script needed.** `internal/testsupport/fakeoidc` starts the fake Keycloak (discovery, JWKS,
  password-grant token endpoint, RS256 tokens signed with the standard library) in its own `init()`.
  Go initialises packages in import-path order among those whose imports are done; `fakeoidc` sorts
  before `middleware` and imports only standard-library packages `middleware` imports anyway, so it
  always runs first and sets `KEYCLOAK_CONFIG` to a generated config without secrets. A blank or direct
  import is enough; `rest.TestInitOrder` fails if the order ever changes. `go test ./rest/` works as is.
  It also sets `time.Local` (a later write in `TestMain` raced with its server goroutines under `-race`).
- Tokens are real: signed by the fake, verified by the production go-oidc verifier (no
  `VerifyTokenClaims` hook). `ProtectApi` runs its real password grant against the fake.
- `internal/testsupport/apitest`: router as `server.go` wires it, token and Basic helpers, a loopback gRPC
  server for mail and master data (`StartFakes`), `Do` that records a handler panic as 599 (net/http
  would drop the connection).
- 17 of 17 entry points in `rest/matrix_test.go` (`TestAuthMatrix`, 14 cases per route where they apply);
  `TestRouteWalkGuard` checked once with a removed entry (red, restored). Handler cases in
  `rest/handlers_test.go`, GraphQL in `rest/graphql_test.go`.
- Defect tests (skipped, run red once): #17, #28, #36, #37, #39, #40, new #53. The F1 case (#16) is
  **not** in the repository: it is a private test outside it (critical, details not published).
- Recorded, not changed: the guards answer differently (`ProtectApi` 400 for a wrong scheme, GraphQL 401
  for a foreign tenant, no `superuser` bypass in `ProtectApi`); raw queries are widened to whole days.
- Coverage: `rest` 84 %, `graph` 100 % (hand-written), `middleware` 55 % — the guard functions are at
  100 %, the rest is dead code (`jwt.go` `JWTMiddleware`, `ExtractClaims`, `KeycloakClient.Authenticate`)
  and the internal-issuer branch of `init()`; target 80 % not met for that reason. Total 48.8 → 65.2 %.
- The package runs in about 2 s (under 20 s), only loopback addresses.
