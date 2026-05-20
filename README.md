# XRouter

XRouter is a lightweight Go router for OpenAI-compatible APIs. It reads the existing local `db.json` format used by XLab Router and exposes a small, low-memory HTTP API for health, settings, providers, models, usage summary, and upstream proxying.

## Run

```powershell
go run ./cmd/xrouter
```

Default address: `:1213`. Override with `XROUTER_ADDR`, for example:

```powershell
$env:XROUTER_ADDR=':1214'; go run ./cmd/xrouter
```

## Core Endpoints

- `GET /api/health`
- `GET /api/version`
- `GET/PATCH /api/settings`
- `GET/POST /api/providers`
- `PATCH/DELETE /api/providers/{id}`
- `GET /api/models`
- `GET /api/quota`
- `GET /api/usage/summary`
- `POST /v1/chat/completions`
- `POST /v1/messages`
- `POST /v1/responses`

## Provider Quickstart

Create an OpenAI-compatible provider:

```powershell
Invoke-RestMethod -Method Post http://localhost:1213/api/providers `
  -ContentType 'application/json' `
  -Body '{"provider":"openai-compatible","name":"Local OpenAI-compatible","apiKey":"sk-local","providerSpecificData":{"baseUrl":"http://localhost:8080/v1","apiType":"openai"}}'
```

Create an Anthropic-compatible provider:

```powershell
Invoke-RestMethod -Method Post http://localhost:1213/api/providers `
  -ContentType 'application/json' `
  -Body '{"provider":"anthropic-compatible","name":"Local Anthropic-compatible","apiKey":"sk-local","providerSpecificData":{"baseUrl":"http://localhost:8080","apiType":"anthropic"}}'
```

Create a Gemini-compatible provider:

```powershell
Invoke-RestMethod -Method Post http://localhost:1213/api/providers `
  -ContentType 'application/json' `
  -Body '{"provider":"gemini-compatible","name":"Local Gemini-compatible","apiKey":"sk-local","providerSpecificData":{"baseUrl":"http://localhost:8080","apiType":"gemini"}}'
```

Use `provider/model` names, for example `openai-compatible/gpt-test`, `anthropic-compatible/claude-test` or `gemini-compatible/gemini-test`.

## Observability

- `GET /api/usage/stats` - aggregated counts/cost by provider, model and day.
- `GET /api/usage/logs?limit=100` - recent request logs.
- `GET /api/usage/logs/{id}` - full request log entry by id.
- `GET /api/usage/history?provider=` - usage history rows.
- `GET /api/usage/stream?limit=50` - Server-Sent Events stream emitting `snapshot`, `update` and `heartbeat` events. Add `?once=1` to receive a single snapshot frame and close.

## Local Dashboard

- `GET /dashboard` (localhost only) renders an embedded HTML page that consumes `/api/usage/stats`, `/api/usage/logs` and `/api/usage/stream` for a live view of activity.

## Versioning

- `GET /api/version` returns build metadata: `version`, `commit`, `date`, `goVersion`, `uptimeSec`.
- Override defaults at build time via `-ldflags`:

```powershell
go build -ldflags "-X xrouter/internal/version.Version=v0.1.0 -X xrouter/internal/version.Commit=abc123 -X xrouter/internal/version.Date=2026-05-20T00:00:00Z" ./cmd/xrouter
```

## Release

Push a tag matching `v*` to trigger `.github/workflows/release.yml`. The workflow builds static binaries for Linux amd64/arm64, Windows amd64 and macOS arm64, writes SHA256 files, and publishes them to a GitHub Release.
## Tests and CI

Run the full suite locally:

```powershell
gofmt -l .
go vet ./...
go test ./...
go build ./cmd/xrouter
```

The repository ships with `.github/workflows/ci.yml` which runs `gofmt`, `go vet ./...`, `go test ./...` and `go build ./cmd/xrouter` on every push and pull request to `main`.
