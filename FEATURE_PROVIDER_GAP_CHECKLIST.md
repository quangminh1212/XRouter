# XRouter Feature and Provider Gap Checklist

Date: 2026-05-21
Repo under review: `C:\Dev\XRouter`
Reference repos:
- `https://github.com/diegosouzapw/OmniRoute`
- `https://github.com/decolua/9router`
- `https://github.com/router-for-me/CLIProxyAPI`

## Summary

- XRouter scope has expanded strongly and now includes OAuth/web-cookie onboarding, media/search endpoints, dashboard, MCP/A2A/tunnel, usage streams, and management APIs.
- XRouter built-in provider catalog currently includes **169+** provider/catalog aliases with audited 9router + OmniRoute asset parity.
- Provider parity is **complete for audited 9router and OmniRoute public provider assets** plus CLIProxyAPI core families.
- Current re-check from `public/providers` assets indicates **missing=0** for both 9router and OmniRoute after alias/catalog coverage.

## Re-check Note (2026-05-21)

- Source used for re-check: direct clone latest heads of 3 repos + provider asset files under `public/providers` + local `internal/store/db.go` catalog.
- Local catalog count: **169+** providers/aliases.
- `9router public/providers`: **100** assets, covered by XRouter catalog/alias mapping: **100**, missing: **0**.
- `OmniRoute public/providers`: **67** assets, covered by XRouter catalog/alias mapping: **67**, missing: **0**.
- `CLIProxyAPI` documented/core provider families and major `/v0/management` compatibility aliases are covered by XRouter at core family level.
## Current XRouter Coverage

### Implemented core management and routing

- [x] `GET /api/health`
- [x] `GET/PATCH /api/settings`
- [x] `GET/POST /api/providers`
- [x] `PATCH/DELETE /api/providers/{id}`
- [x] `GET/POST /api/keys`
- [x] `PATCH/DELETE /api/keys/{id}`
- [x] `GET /api/models`
- [x] `GET/PUT/PATCH/DELETE /api/management/model-mappings`
- [x] `GET /api/quota`
- [x] `GET /api/usage/summary`
- [x] `GET /api/monitoring/health`
- [x] `GET /api/debug/db`
- [x] `POST /v1/chat/completions`
- [x] `POST /v1/messages`
- [x] `POST /v1/responses`

### Implemented routing/runtime behavior

- [x] Multiple active provider connections
- [x] Fallback across candidate connections
- [x] Cooldown handling for `429` and `5xx`
- [x] Circuit/backoff state persisted in DB
- [x] Per-key in-memory RPM limiting
- [x] Forced model mapping before proxy
- [x] Short-term non-stream request dedup
- [x] Outbound proxy support from settings
- [x] Usage tracking and estimated pricing

## Feature Gap Checklist

### 1) Common proxy/API surface

- [x] OpenAI chat completions
- [x] Anthropic messages
- [x] OpenAI responses
- [x] OpenAI legacy completions
- [x] Claude count tokens
- [x] Gemini-compatible `v1beta/models/*action`
- [x] Responses compact variant
- [x] Responses websocket/stream alias
- [x] Codex direct alias routes such as `/backend-api/codex/responses`

### 2) Media and non-chat endpoints

- [x] Embeddings
- [x] Images generations
- [x] Images edits
- [x] Image-to-text / vision-specific media route
- [x] Text-to-speech
- [x] Speech-to-text
- [x] Video generation
- [x] Video edits/extensions/retrieve
- [x] Music/audio generation
- [x] Web search endpoint
- [x] Web fetch endpoint
- [x] Voice listing endpoints

### 3) Management API depth

- [x] Provider CRUD base
- [x] API key CRUD base
- [x] Basic settings patch
- [x] Basic model mappings
- [x] Basic quota/usage summary
- [x] Provider connection test endpoint
- [x] Provider model test endpoint
- [x] Provider models endpoint
- [x] Provider credential validate endpoint
- [x] Usage request logs
- [x] Usage history endpoint
- [x] Usage chart/stats stream
- [x] Request detail endpoint
- [x] Model alias management
- [x] Disabled model management
- [x] Model availability checks
- [x] Routing strategy management
- [x] Retry/max retry interval management
- [x] OAuth excluded models / model alias controls
- [x] Auth file upload/download/index flows
- [x] Vertex credential import flow
- [x] Proxy pools / upstream pool manager
- [x] Provider nodes / compatible node manager

### 4) Authentication/provider onboarding

- [x] Manual API key storage
- [x] OAuth token import groundwork (local-only API)
- [x] OAuth token manual refresh endpoint
- [x] OAuth token auto-refresh before request / one retry on 401
- [x] OAuth PKCE browser flow groundwork for codex/gemini
- [x] Manual access token storage
- [x] Manual refresh token storage
- [x] OAuth login flow for Claude
- [x] OAuth login flow for Codex/OpenAI
- [x] OAuth login flow for Gemini CLI
- [x] OAuth login flow for Antigravity
- [x] OAuth login flow for Kimi
- [x] OAuth login flow for xAI/Grok
- [x] Browser-cookie/web-session onboarding
- [x] Auto import from local IDE/CLI credentials
- [x] Multi-account auth file management

### 5) Routing and platform capabilities

- [x] Basic fallback routing
- [x] Sticky/round-robin settings fields in DB
- [x] Rich routing strategies like those advertised by OmniRoute
- [x] Combo model system
- [x] Multi-account weighted round-robin / quota-aware selection
- [x] Provider/account-level policy assignment
- [x] MCP endpoints/server integration
- [x] A2A protocol endpoints
- [x] Tunnel/remote exposure features
- [x] CLI auto-config endpoints for external tools
- [x] Dashboard/web UI
- [x] Real-time logs/analytics UI

## Reference Repo Coverage Snapshot

### 9router

- Approx provider count from `public/providers` assets: **100**
- Extra service kinds detected: `embedding`, `image`, `imageToText`, `tts`, `stt`, `webSearch`, `webFetch`, `video`, `music`
- Major extra API groups present: `combos`, `provider-nodes`, `proxy-pools`, `usage/logs/stats/stream`, `translator`, `tunnel`, `mcp`, `cli-tools`, media provider voice helpers

### OmniRoute

- Approx provider count from `public/providers` assets: **67**
- Provider groups detected: `free`, `oauth`, `web-cookie`, `apikey`, `local`, `search`, `audio-only`, `upstream-proxy`, `cloud-agent`
- Large capability surface noted in repo structure/README: dashboard, MCP, A2A, provider metrics, pricing/catalog sync, search providers, upstream proxy assignment, many routing strategies

### CLIProxyAPI

- Core provider/auth families clearly present: `claude`, `codex`, `gemini`, `antigravity`, `kimi`, `vertex`, `xai`, plus OpenAI-compatible and Anthropic-compatible handling
- Major extra API/management surface: `chat/completions`, `completions`, `images`, `videos`, `messages/count_tokens`, `responses/compact`, `v1beta/models/*action`, OAuth callback flows, auth-files management, request retry, routing strategy, model alias/exclusion controls

## Priority Gap Backlog

### P1 - should implement first

- [x] Add generic provider adapters for OpenAI-compatible, Anthropic-compatible, and Gemini-compatible upstreams
- [x] Add `POST /v1/completions`
- [x] Add `POST /v1/embeddings`
- [x] Add `POST /v1/images/generations`
- [x] Add `POST /v1/audio/speech`
- [x] Add `POST /v1/audio/transcriptions`
- [x] Add `POST /v1/search`
- [x] Add provider validate/test/models endpoints
- [x] Add `GET /api/providers/{id}/models`
- [x] Add detailed per-request usage logs

### P2 - next expansion

- [x] Add OAuth provider flows: `claude`, `codex`, `github`, `antigravity`, `xai`
- [x] Add account pool management and richer round-robin/fallback strategies
- [x] Add model alias / disabled / availability management
  - [x] Model alias management
  - [x] Disabled model management
  - [x] Model availability management
- [x] Add CLI-compatible aliases and advanced response variants

### P3 - platform parity layer

- [x] Add web dashboard
- [x] Add combo models and policy routing
  - [x] Combo model system
  - [x] Policy routing
- [x] Add MCP support
- [x] Add A2A support
- [x] Add tunnel / remote access helpers

## Regression Matrix Coverage

- [x] `openai` -> `POST /v1/chat/completions`
- [x] `openai` -> `POST /v1/completions`
- [x] `openai`/responses -> `POST /v1/responses`
- [x] `anthropic-compatible` -> adapter `POST /v1/chat/completions` => `/v1/messages`
- [x] `gemini-compatible` -> adapter `POST /v1/chat/completions` => `/v1beta/models/*:generateContent`
- [x] Wave 1 smoke: `deepseek` chat proxy
- [x] Media smoke: embeddings / audio speech / audio transcriptions
- [x] OAuth start smoke: `claude`, `gemini`, `antigravity`, `kimi`, `github`, `xai`
- [x] Realtime observability: `GET /api/usage/stream`
- [x] Local dashboard render: `GET /dashboard`

## Top 30 Provider Implementation Priority

Scoring formula: `(repos_count * 2) + impact_boost - (difficulty * 0.5)`
- `repos_count` = number of reference repos that include this provider
- `impact_boost` = +2 if provider is widely used in CLI coding tools
- `difficulty` = 1 (apikey/search), 2 (audio/local), 3 (oauth), 4 (cookie)

| # | Provider | Present in refs | Auth kind | Difficulty (1-4) | Score |
| --- | --- | --- | --- | --- | --- |
| 1 | `antigravity` | 9router, CLIProxyAPI, OmniRoute | oauth | 3 | 6.5 |
| 2 | `claude` | 9router, CLIProxyAPI, OmniRoute | oauth | 3 | 6.5 |
| 3 | `codex` | 9router, CLIProxyAPI, OmniRoute | oauth | 3 | 6.5 |
| 4 | `gemini` | 9router, CLIProxyAPI, OmniRoute | oauth | 3 | 6.5 |
| 5 | `kimi` | 9router, CLIProxyAPI, OmniRoute | oauth | 3 | 6.5 |
| 6 | `vertex` | 9router, CLIProxyAPI, OmniRoute | oauth | 3 | 6.5 |
| 7 | `xai` | 9router, CLIProxyAPI, OmniRoute | oauth | 3 | 6.5 |
| 8 | `brave-search` | 9router, OmniRoute | search | 1 | 5.5 |
| 9 | `cerebras` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 10 | `cohere` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 11 | `deepseek` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 12 | `fireworks` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 13 | `gemini-cli` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 14 | `glm` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 15 | `glm-cn` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 16 | `groq` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 17 | `huggingface` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 18 | `jina-ai` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 19 | `kiro` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 20 | `minimax` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 21 | `mistral` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 22 | `nvidia` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 23 | `opencode` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 24 | `perplexity` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 25 | `siliconflow` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 26 | `together` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 27 | `vercel-ai-gateway` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 28 | `voyage-ai` | 9router, OmniRoute | apikey | 1 | 5.5 |
| 29 | `assemblyai` | 9router, OmniRoute | audio | 2 | 5.0 |
| 30 | `aws-polly` | 9router, OmniRoute | audio | 2 | 5.0 |

### Implementation notes by auth kind

- **apikey (diff=1)**: Ch? c?n thêm `baseUrl` + `apiKey` header vào `resolveEndpoint`. H?u h?t d?u OpenAI-compatible. Có th? batch thêm nhi?u provider cùng lúc b?ng cách m? r?ng switch-case trong `forwarder.go`.
- **search (diff=1)**: C?n endpoint riêng `/v1/search`, body/response format khác chat completions. Nên tách handler.
- **audio (diff=2)**: C?n endpoint `/v1/audio/speech` và `/v1/audio/transcriptions`, format multipart/form-data cho STT.
- **oauth (diff=3)**: C?n OAuth flow riêng cho t?ng provider (PKCE, device code, ho?c browser redirect), luu access/refresh token, auto-refresh. Ðây là ph?n ph?c t?p nh?t, nên làm sau khi apikey providers ?n d?nh.

### Recommended implementation order

1. **Batch apikey providers** (#9-28): T?t c? d?u OpenAI-compatible ho?c Anthropic-compatible, ch? c?n m? r?ng `resolveEndpoint` v?i `baseUrl` catalog. Có th? làm trong 1-2 session.
2. **Search providers** (#8): Thêm `/v1/search` handler + adapter cho Brave, Serper, Tavily, Exa, Perplexity.
3. **Audio providers** (#29-30): Thêm `/v1/audio/speech` + `/v1/audio/transcriptions` handler.
4. **OAuth providers** (#1-7): Làm t?ng provider m?t, b?t d?u t? `gemini` (API key tru?c, OAuth sau), r?i `claude`, `codex`, `xai`, `antigravity`, `kimi`, `vertex`.

## Wave 1 Technical Roadmap (Implement Now)

Scope m?c tiêu Wave 1:
- Ch? làm nhóm provider `apikey`/compatible có tác d?ng cao.
- Không d?ng OAuth flow ? Wave 1.
- Không thêm dashboard/UI, ch? API + router core.

Target providers cho Wave 1:
- `deepseek`, `groq`, `mistral`, `cerebras`, `fireworks`, `together`, `siliconflow`, `vercel-ai-gateway`, `cohere`, `perplexity`

Success criteria:
- [x] T?o provider connection cho các provider trên qua `POST /api/providers` ch?y du?c.
- [x] Proxy thành công qua `POST /v1/chat/completions` v?i model prefix `provider/model`.
- [x] `GET /api/models` tr? model h?p l? (ít nh?t g?m model map tinh fallback).
- [x] Cooldown/backoff ho?t d?ng dúng khi provider tr? `429/5xx`.
- [x] `go test ./...` pass, `go build ./cmd/xrouter` pass.

### A. File changes and exact responsibilities

1) `C:\Dev\XRouter\internal\proxy\forwarder.go`
- M? r?ng `resolveEndpoint(...)`:
  - map `provider -> default baseUrl` cho nhóm Wave 1.
  - gi? uu tiên `ProviderSpecificData.baseUrl` n?u user dã set.
- M? r?ng endpoint resolver theo path:
  - gi? nguyên `/v1/chat/completions`, `/v1/messages`, `/v1/responses`.
  - chu?n b? s?n nhánh an toàn cho `/v1/completions` (n?u body tuong thích OpenAI).
- B? sung normalize nh? theo provider:
  - strip prefix `provider/model` nhu hi?n có.
  - gi? default `openai` mode cho ph?n l?n provider apikey.
- Không d?i logic cooldown/circuit hi?n t?i (dã ?n d?nh).

2) `C:\Dev\XRouter\internal\store\db.go`
- B? sung metadata helper cho provider catalog c?c b?:
  - danh sách provider id + default baseUrl + apiType default.
  - helper validate provider khi t?o/s?a connection (không b?t bu?c c?ng; cho phép custom provider n?u có baseUrl).
- B? sung fallback model map tinh t?i thi?u theo provider (d? `GET /api/models` có d? li?u h?u ích khi upstream chua g?i du?c).

3) `C:\Dev\XRouter\internal\app\server.go`
- Nâng `GET /api/models`:
  - merge model t? active connections + fallback static catalog c?a Wave 1.
  - gi? format response tuong thích hi?n t?i.
- Nâng `POST /api/providers` và `PATCH /api/providers/{id}`:
  - auto-fill `baseUrl`/`apiType` t? catalog n?u thi?u.
  - không override n?u user dã set explicit `providerSpecificData.baseUrl`.

4) `C:\Dev\XRouter\internal\proxy\forwarder_test.go`
- Thêm test cases cho `resolveEndpoint`:
  - m?i provider Wave 1 ph?i resolve du?c endpoint chat h?p l?.
  - provider custom thi?u `baseUrl` ph?i fail dúng message.
- Thêm test normalize prefix `provider/model` cho ít nh?t 3 provider m?i.

5) `C:\Dev\XRouter\internal\app\server_models_test.go` (file m?i, nh?)
- Test `GET /api/models` có merge fallback models cho provider Wave 1.
- Test không phá v? format cu.

### B. Minimal provider catalog for Wave 1

Catalog d? xu?t (default base URL, có th? override qua providerSpecificData):
- `deepseek` -> `https://api.deepseek.com`
- `groq` -> `https://api.groq.com/openai`
- `mistral` -> `https://api.mistral.ai`
- `cerebras` -> `https://api.cerebras.ai`
- `fireworks` -> `https://api.fireworks.ai/inference/v1`
- `together` -> `https://api.together.xyz/v1`
- `siliconflow` -> `https://api.siliconflow.cn/v1`
- `vercel-ai-gateway` -> `https://ai-gateway.vercel.sh/v1`
- `cohere` -> `https://api.cohere.com/compatibility/v1`
- `perplexity` -> `https://api.perplexity.ai`

Model fallback t?i thi?u (d? hi?n th? và smoke):
- `deepseek/deepseek-chat`
- `groq/llama-3.1-70b-versatile`
- `mistral/mistral-large-latest`
- `cerebras/llama3.1-70b`
- `fireworks/accounts/fireworks/models/llama-v3p1-70b-instruct`
- `together/meta-llama/Llama-3.1-70B-Instruct-Turbo`
- `siliconflow/Qwen/Qwen2.5-Coder-32B-Instruct`
- `vercel-ai-gateway/openai/gpt-4o-mini`
- `cohere/command-r-plus`
- `perplexity/sonar-pro`

### C. Ordered execution plan (code)

1. Implement provider catalog constants + helper in store layer.
2. Update provider create/patch flow to auto-fill baseUrl/apiType.
3. Extend `resolveEndpoint` with Wave 1 providers.
4. Extend `/api/models` with fallback model merge.
5. Add/adjust tests (`forwarder_test`, `server_models_test`).
6. Run `go test ./...`.
7. Run `go build ./cmd/xrouter`.

### D. Risks and guardrails

- Risk: base URL d?i theo th?i gian ? luôn cho phép override b?ng `providerSpecificData.baseUrl`.
- Risk: provider tr? schema hoi khác OpenAI ? Wave 1 gi?i h?n vào chat-completions compatible path.
- Risk: model ID d?i nhanh ? fallback model ch? d? bootstrap/smoke, không hard guarantee.

### E. Done definition (strict)

- [x] Có th? thêm 10 provider Wave 1 không c?n nh?p baseUrl th? công.
- [x] Proxy chat qua m?i provider Wave 1 pass t?i thi?u 1 request smoke.
- [x] Không regression các provider cu: `openai`, `anthropic`, `openrouter`.
- [x] Toàn b? test pass, build pass.

## Provider Matrix

Legend:
- `Present in refs` = repo(s) where the provider was detected from code/constants or clearly from server/auth surface.
- `XRouter now` = `YES` means there is built-in or generic route coverage today; `NO` means not yet supported in a meaningful built-in way.
- `Notes` = rough implementation implication for XRouter.

| Provider | Present in refs | Ref groups | XRouter now | Notes |
| --- | --- | --- | --- | --- || `agentrouter` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `ai21` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `aimlapi` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `alibaba` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `alibaba-cn` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `alicode` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `alicode-intl` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `amazon-q` | OmniRoute | free | NO | Need adapter/auth/base URL/model transform |
| `anthropic` | 9router, OmniRoute | apiKey, apikey | YES | Built-in/generic route |
| `anthropic-compatible` | CLIProxyAPI | core/oauth/sdk | YES | Built-in/generic route |
| `antigravity` | 9router, CLIProxyAPI, OmniRoute | core/oauth/sdk, oauth | YES | Already in catalog at internal/store/db.go |
| `apikey` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `assemblyai` | 9router, OmniRoute | apiKey, audio_only | YES | Added STT upload/submit/poll adapter |
| `auto` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `aws-polly` | 9router, OmniRoute | apiKey, audio_only | YES | Added TTS catalog + AWS SigV4 signer + Polly body transform |
| `azure` | 9router | apiKey | YES | Already in catalog at internal/store/db.go |
| `azure-ai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `azure-openai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `bailian-coding-plan` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `baseten` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `bazaarlink` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `bedrock` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `black-forest-labs` | 9router, OmniRoute | apiKey, apikey | YES | Added image catalog + BFL endpoint/body/header transform |
| `blackbox` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `blackbox-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `brave-search` | 9router, OmniRoute | apiKey, search | YES | Already in catalog at internal/store/db.go |
| `byteplus` | 9router | freeTier | YES | Already in catalog at internal/store/db.go |
| `bytez` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `cablyai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `cartesia` | 9router, OmniRoute | apiKey, audio_only | YES | Added TTS catalog + native endpoint/body/header transform |
| `cerebras` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `chatgpt-web` | OmniRoute | web_cookie | YES | Added web-cookie catalog metadata + cookie-import default hydration |
| `chutes` | 9router, OmniRoute | apiKey, apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `clarifai` | OmniRoute | apikey | YES | Added OpenAI-compat catalog + Authorization: Key header |
| `claude` | 9router, CLIProxyAPI, OmniRoute | core/oauth/sdk, oauth | YES | Already in catalog at internal/store/db.go |
| `cline` | 9router, OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `cliproxyapi` | OmniRoute | upstream_proxy | NO | Need adapter/auth/base URL/model transform |
| `cloudflare-ai` | 9router, OmniRoute | apikey, freeTier | YES | Already in catalog at internal/store/db.go |
| `codestral` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `codex` | 9router, CLIProxyAPI, OmniRoute | core/oauth/sdk, oauth | YES | Already in catalog at internal/store/db.go |
| `codex-cloud` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `cohere` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `comfyui` | 9router, OmniRoute | apiKey, local | NO | Need adapter/auth/base URL/model transform |
| `command-code` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `commandcode` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `completions` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `copilot-web` | OmniRoute | web_cookie | YES | Added web-cookie catalog metadata + cookie-import default hydration |
| `coqui` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `crof` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `cursor` | 9router, OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `databricks` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `datarobot` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `deepgram` | 9router, OmniRoute | apiKey, audio_only | YES | Already in catalog at internal/store/db.go |
| `deepinfra` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `deepseek` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `deepseek-web` | OmniRoute | web_cookie | YES | Added web-cookie catalog metadata + cookie-import default hydration |
| `devin` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `devin-cli` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `docker-model-runner` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `edge-tts` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `elevenlabs` | 9router, OmniRoute | apiKey, audio_only | YES | Added TTS catalog + native endpoint/body/header transform |
| `empower` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `enally` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `exa` | 9router | apiKey | YES | Already in catalog at internal/store/db.go |
| `exa-search` | OmniRoute | search | YES | Added search catalog alias + reused existing Exa search adapter |
| `fal-ai` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `featherless-ai` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `fenayai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `firecrawl` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `fireworks` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `freetheai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `friendliai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `galadriel` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gemini` | 9router, CLIProxyAPI, OmniRoute | apikey, core/oauth/sdk, freeTier | YES | Already in catalog at internal/store/db.go |
| `gemini-cli` | 9router, OmniRoute | free | YES | Already in catalog at internal/store/db.go |
| `gemini-web` | OmniRoute | web_cookie | YES | Added web-cookie catalog metadata + cookie-import default hydration |
| `getgoapi` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gigachat` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `github` | 9router, OmniRoute | oauth | YES | Already in catalog at internal/store/db.go |
| `github-models` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gitlab` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gitlab-duo` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `gitlawb` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gitlawb-gmi` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `glhf` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `glm` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `glm-cn` | 9router, OmniRoute | apiKey, apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `glmt` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `google-pse` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `google-pse-search` | OmniRoute | search | YES | Added Google Programmable Search adapter + catalog coverage |
| `google-tts` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `grok` | CLIProxyAPI | core/oauth/sdk | YES | Added OAuth catalog alias to xAI flow + fallback model |
| `grok-web` | 9router, OmniRoute | webCookie, web_cookie | YES | Added web-cookie catalog metadata + cookie-import default hydration |
| `groq` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `hackclub` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `haiper` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `heroku` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `huggingface` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `hyperbolic` | 9router, OmniRoute | apiKey, apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `ideogram` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `inference-net` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `inworld` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `jina-ai` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `jina-reader` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `jules` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `kie` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `kilo-gateway` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `kilocode` | 9router, OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `kimi` | 9router, CLIProxyAPI, OmniRoute | apiKey, apikey, core/oauth/sdk | YES | Already in catalog at internal/store/db.go |
| `kimi-coding` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `kimi-coding-apikey` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `kiro` | 9router, OmniRoute | free | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `kluster` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `lambda-ai` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `laozhang` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `lemonade` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `leonardo` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `lepton` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `linkup` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `linkup-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `llama-cpp` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `llamafile` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `llamagate` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `llm7` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `lm-studio` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `local-device` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `longcat` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `maritalk` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `meta-llama` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `minimax` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `minimax-cn` | 9router, OmniRoute | apiKey, apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `mistral` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `modal` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `moonshot` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `morph` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `muse-spark-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `nanobanana` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `nanogpt` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `nebius` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `nlpcloud` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `nous-research` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `novita` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `nscale` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `nvidia` | 9router, OmniRoute | apikey, freeTier | YES | Already in catalog at internal/store/db.go |
| `oauth` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `oci` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `ollama` | 9router | freeTier | NO | Need adapter/auth/base URL/model transform |
| `ollama-cloud` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `ollama-local` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `ollama-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `oobabooga` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `openai` | 9router, CLIProxyAPI, OmniRoute | apiKey, apikey, core/oauth/sdk | YES | Built-in/generic route |
| `openai-compatible` | CLIProxyAPI | core/oauth/sdk | YES | Built-in/generic route |
| `opencode` | 9router, OmniRoute | free | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `opencode-go` | 9router, OmniRoute | apiKey, apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `opencode-zen` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `openrouter` | 9router, OmniRoute | apikey, freeTier | YES | Built-in/generic route |
| `ovhcloud` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `perplexity` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `perplexity-search` | OmniRoute | search | YES | Search provider catalog + adapter already present |
| `perplexity-web` | 9router, OmniRoute | webCookie, web_cookie | YES | Added web-cookie catalog metadata + cookie-import default hydration |
| `petals` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `piapi` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `playht` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `poe` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `pollinations` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `predibase` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `publicai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `puter` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `qianfan` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `qoder` | OmniRoute | free | NO | Need adapter/auth/base URL/model transform |
| `qwen` | OmniRoute | free | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `recraft` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `reka` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `replicate` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `runwayml` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `sambanova` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `sap` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `scaleway` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `sdwebui` | 9router, OmniRoute | apiKey, local | NO | Need adapter/auth/base URL/model transform |
| `searchapi` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `searchapi-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `searxng` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `searxng-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `serper` | 9router | apiKey | YES | Search provider catalog + adapter already present |
| `serper-search` | OmniRoute | search | YES | Added search catalog alias + reused existing Serper adapter |
| `siliconflow` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `snowflake` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `stability-ai` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `suno` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `synthetic` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `t3-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `tavily` | 9router | apiKey | YES | Search provider catalog + adapter already present |
| `tavily-search` | OmniRoute | search | YES | Added search catalog alias + reused existing Tavily adapter |
| `thebai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `together` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `topaz` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `tortoise` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `trae` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `triton` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `udio` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `uncloseai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `upstage` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `v0-vercel` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `venice` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `veoaifree-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `vercel-ai-gateway` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `vertex` | 9router, CLIProxyAPI, OmniRoute | apikey, core/oauth/sdk, freeTier | YES | Already in catalog at internal/store/db.go |
| `vertex-partner` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `vllm` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `volcengine` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `volcengine-ark` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `voyage-ai` | 9router, OmniRoute | apiKey, apikey | YES | Already in catalog at internal/store/db.go |
| `wandb` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `watsonx` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `windsurf` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `xai` | 9router, CLIProxyAPI, OmniRoute | apiKey, apikey, core/oauth/sdk | YES | Already in catalog at internal/store/db.go |
| `xiaomi-mimo` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `xiaomi-tokenplan` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `xinference` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `youcom` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `youcom-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `zai` | OmniRoute | apikey | YES | Added Wave 3 OpenAI-compatible catalog + fallback model |
| `zed` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
## Notes on interpretation

- This matrix is intentionally conservative for `XRouter`.
- `YES` currently means either:
  - built-in routing exists directly (`openai`, `anthropic`, `openrouter`), or
  - the current code can already route a generic compatible provider class without adding a whole new provider-specific auth flow.
- `NO` means XRouter still needs at least one of: provider adapter logic, auth onboarding flow, model normalization, dedicated endpoint support, or management UX/API.










