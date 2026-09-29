# Task Management API (Multi-User)

REST API for a multi-user task manager. Go + Fiber + PostgreSQL, raw SQL (no ORM).

## Run

```bash
docker compose up --build
```

API on `http://localhost:8080`, Postgres on `5432`. Migrations run automatically on API startup.

Local dev without Docker:

```bash
cp .env.example .env   # edit if needed
go run ./cmd/api
```

Requires a reachable Postgres matching `.env`.

## Tests

```bash
go test ./...
```

Unit tests use in-memory fakes/mocks — no DB connection required. The idempotency tests specifically cover:
- sequential duplicate `Idempotency-Key` → identical cached response, no second task created
- 50 concurrent requests sharing the same `Idempotency-Key` → exactly one task created

## Endpoints

### Auth
| Method | Path | Body |
|---|---|---|
| POST | `/auth/register` | `{name, email, password, team_id?}` |
| POST | `/auth/login` | `{email, password}` |

Both return `{access_token, token_type, expires_in_seconds, user}`. Send `Authorization: Bearer <access_token>` on every `/tasks` request below.

### Tasks
| Method | Path | Notes |
|---|---|---|
| POST | `/tasks` | Create. Optional `Idempotency-Key` header (UUID) |
| GET | `/tasks` | List, own tasks only |
| GET | `/tasks/:id` | Detail |
| PUT | `/tasks/:id` | Update `title` / `description` / `status` (partial) |
| DELETE | `/tasks/:id` | Delete |
| POST | `/tasks/:id/assign` | `{assignee_id}` — reassign to a user in the same team |

`GET /tasks` query params: `status` (`pending`\|`in_progress`\|`done`), `search` (matches title), `page`, `limit`.

### Health
`GET /healthz`

## Architecture

Clean, layer-per-domain structure:

```
cmd/api            entrypoint: wiring, routing, server start
internal/
  apperror          single structured error type (HTTP status + code + message)
  response          reusable success/error JSON envelope
  logger            zerolog JSON logger setup
  middleware         RequestLogger, ErrorHandler, JWTAuth, Idempotency
  config, db         env config, connection, tiny migration runner
  auth, task, idempotency   one package per domain: model → repository → service → handler
```

Each domain package follows `handler → service → repository`: handlers only parse/validate
input and shape the HTTP response, services hold business rules, repositories are the only
place that talks SQL. Nothing outside a repository imports `sqlx`/pgx directly.

### Structured error handling
Every error is an `*apperror.AppError{HTTPStatus, Code, Message}`. `middleware.ErrorHandler`
is the single place that renders it (and recovers panics) into:
```json
{"status":"error","code":"NOT_FOUND","message":"task not found","timestamp":"..."}
```
No stack trace or raw internal error ever reaches the client; those go to the log only.

### Structured logging — wide event
`middleware.RequestLogger` emits exactly **one** JSON log line per request (a "wide event"),
carrying `request_id`, `method`, `path`, `status_code`, `latency_ms`, `user_id`, plus whatever
extra fields handlers attached via `middleware.AddField` (e.g. `task_id`, `idempotency_key`)
— instead of several fragmented log statements per request. Level is INFO (<400), WARN (4xx),
ERROR (5xx/panic).

### Idempotency
`Idempotency-Key` (UUID, header) on `POST /tasks` is optional but deduplicated for 24h
(`IDEMPOTENCY_TTL_HOURS`) when present:
1. atomically `Reserve` the key (`idempotency_keys` unique constraint on `key`) — exactly one
   concurrent caller wins.
2. the winner runs the handler, its rendered response is cached via `Complete`.
3. losers/replays with a completed key get the byte-identical cached response back; a still
   in-flight key gets `409 IDEMPOTENCY_IN_PROGRESS`.

### Assign — DB transaction
`POST /tasks/:id/assign` runs entirely inside one transaction: lock the task row, verify the
new assignee is in the same team, update `assignee_id`, insert a `task_logs` row, dispatch the
(mocked/logged) notification. Any failure rolls back the whole thing.

## Schema

`teams`, `users`, `tasks`, `task_logs`, `idempotency_keys` — see `migrations/0001_init.sql`.
