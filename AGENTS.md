# No Lights App (Go) — Agent Instructions

## Project Overview

Real-time power outage reporting system. Users report outages/in-restorations via a
Mapbox map, the backend stores spatial data in Redis Geo and PostgreSQL/PostGIS,
and a Telegram bot notifies nearby users.

**Stack**: Go 1.25 (compiled, single binary) + Redis Geo + PostgreSQL/PostGIS + server-rendered HTML + Mapbox GL
**Infrastructure**: Docker Compose, multi-stage Docker build, embedded frontend (go:embed)

## Architecture

```
Browser (server-rendered HTML + Mapbox GL) ──► Go API (/api/ + /static/) ──► Redis (real-time geo)
                                                    │                            │
                                                    ▼                            ▼
                                           PostgreSQL/PostGIS              Telegram Bot API
                                                    ▲
                              Telegram Bot (polling) ┘
```

The frontend is **not** a separate app — HTML templates and static assets are
embedded into the `api` binary via `go:embed` (`web/embed.go`). One binary serves
everything.

### Services

| Service | Source | Port | Tech |
|---------|--------|------|------|
| `api` | `cmd/api/` | 8000 | Go + chi + pgx + go-redis (embeds web/) |
| `bot` | `cmd/bot/` | — | Go + go-telegram-bot-api |
| `postgres_db` | `migrations/` | 5432 | PostgreSQL 16 + PostGIS 3.4 |
| `redis_db` | — | 6379 | Redis 7 |

### Key files

- **API handlers**: `internal/api/server.go` — router, middleware, all endpoints
- **DB access**: `internal/store/store.go` — pgx queries (PostGIS), bumps query counter
- **Redis geo**: `internal/geo/geo.go` — GEOADD/GEOSEARCH
- **Telegram notifications**: `internal/notify/notify.go` — sendMessage + cooldown
- **Reverse geocoding**: `internal/geocode/geocode.go` — Nominatim (1 req/sec)
- **Telegram bot**: `cmd/bot/main.go` — polling, in-memory conversation state
- **Schema**: `migrations/001_init.sql` — all tables + `v_outage_summary` view
- **Query counter**: `internal/metrics/metrics.go` — deterministic per-request DB count

### Data flow

1. User reports outage on map → `POST /api/reportar` (usuario_id, longitud, latitud, tiene_luz)
2. Backend stores point in Redis Geo key `outages:geo`
3. Backend persists event in PostgreSQL `outage_events`
4. Goroutines (fire-and-forget): reverse geocode (Nominatim), notify Telegram users
5. Map queries `GET /api/consultar-radio?lon=&lat=` → Redis GEOSEARCH
6. Dashboard queries `/api/stats/summary`, `/api/stats/by-day`, `/api/stats/by-zone`

### External APIs

- **Mapbox GL** (map rendering) — token `MAPBOX_TOKEN`
- **Nominatim OSM** (reverse geocoding, 1 req/sec) — no key
- **Telegram Bot API** (notifications) — token `TELEGRAM_TOKEN`

## Commands

### Development

```bash
docker compose up --build    # build & start api + bot + redis + postgres
docker compose down          # stop
docker compose logs -f api   # follow api logs

# Without Docker
go run ./cmd/api
go run ./cmd/bot
```

### Build, test, lint

```bash
make build          # go build ./...
make test           # go test ./...
make vet            # go vet ./...
make fmt            # gofmt -w .
make lint           # go vet + gofmt check
```

### Load testing

```bash
make load-test      # k6 run (requires a running backend)
```

## Code Conventions

### Go
- Go 1.25, standard library first; only add deps when justified
- Packages under `internal/` (not importable externally); `cmd/` for binaries
- No ORM — raw SQL via `pgx` with parameterized queries (`$1`, `$2`, …)
- Background work: `go func()` goroutines (equivalent of `asyncio.create_task`)
- Errors: return `error` up; log with `log.Printf` at the boundary; no panics in handlers
- Env vars via small `getenv(key, default)` helpers (see `cmd/api/main.go`)
- `gofmt`-clean, `go vet`-clean (enforced in CI)

### Database
- PostgreSQL 16 + PostGIS 3.4
- `GIST` indexes on geometry columns; all timestamps `TIMESTAMPTZ`
- Schema in `migrations/001_init.sql` — mounted into postgres `docker-entrypoint-initdb.d`
- New schema changes: append a new numbered migration file (e.g. `002_*.sql`)

### Performance harness (RED method)
- Every response carries `X-Query-Count` (exact DB round-trips) and `X-Process-Time-Ms`
- `internal/metrics` holds the context-based counter; `internal/store` bumps it per query
- `internal/api` middleware resets the counter per request and emits the headers
- Query counts per endpoint are deterministic: `/api/consultar-radio` = 0 (Redis-only),
  `/api/stats/*` = 1, `/api/reportar` = 1 INSERT

## Environment Variables

Copy `.env.example` → `.env`.

| Variable | Required | Used by |
|----------|----------|---------|
| `MAPBOX_TOKEN` | yes | api (map) |
| `TELEGRAM_TOKEN` | yes | api, bot |
| `POSTGRES_USER` | no (nolights) | api, bot, postgres |
| `POSTGRES_PASSWORD` | no (nolights_secret) | api, bot, postgres |
| `SEARCH_RADIUS_KM` | no (0.5) | api |
| `DATABASE_URL` | yes | api, bot |

## Gotchas

- **Nominatim rate limit**: 1 req/sec max — `geocode.Lookup` sleeps 1s before each call. Don't remove.
- **Migrations**: `migrations/` runs only on first postgres volume creation (initdb.d). For an existing DB, apply new files manually or reset the volume.
- **go:embed**: the frontend is compiled into the binary. Editing `web/` requires recompiling (`go build`).
- **Bot is polling-based**: no webhook. `cmd/bot` runs `GetUpdatesChan` in a loop.
- **Zone fallback**: when `neighborhood` is NULL, stats cascade to `municipality` → `state` → `country` (`internal/store` `validZoneFields`).
- **Geo key**: all outage points use the single Redis key `outages:geo` (no TTL; `ZREM` on restore).

## Testing Approach

- **Unit**: `go test ./...` — metrics counter, handler validation (422 paths), JSON types. No mocks for validation-only paths.
- **Integration**: CI provides real Redis + PostGIS (GitHub Actions services); `DATABASE_URL`/`REDIS_HOST` point at them.
- **Perf**: deterministic query-count via `X-Query-Count` header; load via k6 (`k6/load.js`, scheduled).
- **Race detector**: `go test -race` in CI.

## References

- [Google "Software Engineering at Google"](https://abseil.io/resources/swe-book) — Ch. 11–16 (testing, CI/CD)
- [DORA State of DevOps 2024](https://dora.dev/research/) — 4 key metrics
- [Google SRE — RED method](https://grafana.com/blog/2018/08/02/the-red-method-how-to-instrument-your-services/) — Rate, Errors, Duration
- [Grafana k6](https://grafana.com/oss/k6/) — load testing
- [Effective Go](https://go.dev/doc/effective_go) — idiomatic Go
- [12-Factor App](https://12factor.net/) — cloud-native methodology
