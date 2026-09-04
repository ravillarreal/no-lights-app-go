---
name: harness
description: Run tests, linting, build, and performance checks for the No Lights Go app. Use when you need to verify code changes, run the test suite, measure query counts, or set up testing/perf infrastructure.
---

## Overview

The No Lights Go app uses `go build`, `go vet`, `gofmt`, `go test`, and k6 for its quality gate. All commands run from the repo root.

## Build, format, vet

```bash
go build ./...          # compile everything
gofmt -l .              # list unformatted files (empty = clean)
go vet ./...            # static analysis
make build              # alias for go build ./...
```

## Tests

```bash
go test ./...           # all packages
go test -race ./...     # with race detector (runs in CI)
go test ./internal/api/ -v   # just the API handlers
```

Integration tests need real Redis + PostGIS. In CI these are GitHub Actions services; locally use `docker compose up -d redis_db postgres_db` first.

## Performance harness (deterministic)

Every response carries two metrics headers:
- `X-Query-Count` — exact DB round-trips in the request path
- `X-Process-Time-Ms` — wall-clock latency

The counter lives in `internal/metrics/metrics.go` (context-based); `internal/store` bumps it per query; `internal/api` middleware resets per request and emits headers.

When adding an endpoint:
1. Decide its expected query count (`/api/consultar-radio` = 0, `/api/stats/*` = 1, `/api/reportar` = 1 INSERT)
2. Add an assertion in `internal/api/server_test.go` (validation paths don't need infra)
3. For DB-touching counts, add an integration test that hits a real DB

## Load testing (k6 — scheduled, non-deterministic)

```bash
make load-test      # k6 run k6/load.js (requires running backend)
```

Runs in CI only on `workflow_dispatch` or the weekly schedule, never on push.

## When to use this skill

Invoke when:
- Verifying changes (build + vet + test)
- Measuring query count or latency of an endpoint
- Adding new tests or perf assertions
- Debugging a CI failure
