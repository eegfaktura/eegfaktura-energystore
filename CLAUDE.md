# CLAUDE.md

The working agreement for this repository lives in [AGENTS.md](AGENTS.md).
Read it in full before changing anything. It covers standards, tenant isolation, mandatory tests,
the storage-format concept, logging, external sources, size limits and the tracking files you must
update (`AGENT_LOG.md`, `known-errors.md`, `open-points.md`, `EXTERNAL_SOURCES.md`).

## Strict rules

- **Tenant isolation:** every handler and resolver uses the tenant of the checked header, which must
  be in the token's `tenant` claim (or the token has `superuser`) — never a tenant from the path, body
  or GraphQL arguments (AGENTS.md section 6).
- **Dependencies:** every version fixed exactly, no release younger than **7 days** — Go modules,
  tools, the Go image, Docker images and CI actions alike (AGENTS.md section 12.1).
- **Tests:** Badger data only in `t.TempDir()`, a fixed time zone, `go test -race ./...` once per step
  (AGENTS.md section 10).
