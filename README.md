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
- `GET/PATCH /api/settings`
- `GET/POST /api/providers`
- `PATCH/DELETE /api/providers/{id}`
- `GET /api/models`
- `GET /api/quota`
- `GET /api/usage/summary`
- `POST /v1/chat/completions`
- `POST /v1/messages`
- `POST /v1/responses`
