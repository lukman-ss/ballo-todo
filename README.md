# Ballo Todo

Minimal production-ready Go web API demonstrating the **Ballo** framework.

## Scope

- Ballo version: `v0.0.0-20261027061017-e84a7679c8a4`
- Go version: `1.23.0`
- Standard library only beyond Ballo (no extra deps)

## Quick Start

```bash
# Clone and run
go run cmd/server/main.go

# Or with Makefile
make run
```

Server binds to `HTTP_ADDR` (default `:8080`). Check health:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Build & Test

```bash
make check       # fmt + vet + test + race
make build       # binary → bin/server
make test        # unit tests only
make race        # race detector
```

## Docker

```bash
docker build -t ballo-todo .
docker run -p 8080:8080 ballo-todo
```

## Configuration

All values via environment variables (see `.env.example`):

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | `development` | `production` enables stricter defaults |
| `HTTP_ADDR` | `:8080` | Listen address |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | |
| `HTTP_READ_TIMEOUT` | `10s` | |
| `HTTP_WRITE_TIMEOUT` | `10s` | |
| `HTTP_IDLE_TIMEOUT` | `120s` | |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown window |
| `MAX_BODY_BYTES` | `1048576` | 1 MB |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Endpoints

### Health probes

| Method | Path | Status |
|---|---|---|
| `GET` | `/healthz` | `200` with `{"status":"pass"}` |
| `GET` | `/readyz` | `200` or `503` |

### Tasks (in-memory demo store)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/tasks` | Create task |
| `GET` | `/api/v1/tasks` | List tasks (sorted by ID) |
| `GET` | `/api/v1/tasks/:id` | Get task by ID |
| `PATCH` | `/api/v1/tasks/:id` | Partial update (title / done) |
| `DELETE` | `/api/v1/tasks/:id` | Delete task |

#### Example: create a task

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy milk"}'
```

Response:

```json
{
  "id": "1",
  "title": "Buy milk",
  "done": false,
  "created_at": "2026-10-27T06:00:00Z",
  "updated_at": "2026-10-27T06:00:00Z"
}
```

#### Example: list tasks

```bash
curl http://localhost:8080/api/v1/tasks
```

#### Example: partial update

```bash
curl -X PATCH http://localhost:8080/api/v1/tasks/1 \
  -H "Content-Type: application/json" \
  -d '{"done": true}'
```

## Response Formats

### Success

Standard JSON response with `200`, `201`, `204` as appropriate.

### Error

```json
{"error": "<message>"}
```

Client errors return `400`, resource-not-found returns `404`, method-not-allowed returns `405`. Internal errors return `500` without leaking stack traces.

## Structure

```
cmd/server/main.go       # entrypoint, signal handling, graceful shutdown
internal/app/            # wiring (engine, middleware, routes)
internal/config/         # env loading + validation
internal/http/handler/   # HTTP request → business logic
internal/task/           # in-memory store + domain model
```

## Limitations

- Data is **in-memory**: lost on restart; not shared across instances.
- No authentication, authorization, or rate limiting.
- No pagination, filtering, or sorting beyond ID-order for `LIST`.
- No persistence layer (database, cache).

## Security Notes

- No secrets in code or `.env.example`.
- `.env` is gitignored; do not commit it.
- Request body limited by `MAX_BODY_BYTES`.
- Logs record `method`, `path`, `status`, `duration_ms`, `remote_addr` — no headers, no body, no credentials.

## Porting to Your Project

1. Copy this repository.
2. Update module path in `go.mod` and all imports (`github.com/lukman-ss/ballo-todo` → your path).
3. Replace or extend the `task` package with your domain model.
4. Add your routes in `internal/app/app.go`.
