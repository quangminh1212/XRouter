# XRouter Feature and Provider Gap Checklist

Date: 2026-05-20
Repo under review: `C:\Dev\XRouter`
Reference repos:
- `https://github.com/diegosouzapw/OmniRoute`
- `https://github.com/decolua/9router`
- `https://github.com/router-for-me/CLIProxyAPI`

## Summary

- Current XRouter scope is a lightweight Go router with local DB-backed settings, provider CRUD, API key CRUD, usage summary, forced model mappings, monitoring health, and proxy forwarding.
- Current built-in upstream/provider compatibility is effectively limited to `openai`, `anthropic`, `openrouter`, plus generic OpenAI-compatible / Anthropic-compatible style routing when manually configured with `baseUrl` and `apiType`.
- XRouter does **not** yet cover the majority of provider integrations, media endpoints, management APIs, OAuth flows, or advanced routing/dashboard capabilities found in the 3 reference repos.

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
- [ ] OpenAI legacy completions
- [ ] Claude count tokens
- [ ] Gemini-compatible `v1beta/models/*action`
- [ ] Responses compact variant
- [ ] Responses websocket/stream alias
- [ ] Codex direct alias routes such as `/backend-api/codex/responses`

### 2) Media and non-chat endpoints

- [x] Embeddings
- [ ] Images generations
- [ ] Images edits
- [ ] Image-to-text / vision-specific media route
- [x] Text-to-speech
- [x] Speech-to-text
- [ ] Video generation
- [ ] Video edits/extensions/retrieve
- [ ] Music/audio generation
- [ ] Web search endpoint
- [ ] Web fetch endpoint
- [ ] Voice listing endpoints

### 3) Management API depth

- [x] Provider CRUD base
- [x] API key CRUD base
- [x] Basic settings patch
- [x] Basic model mappings
- [x] Basic quota/usage summary
- [x] Provider connection test endpoint
- [ ] Provider model test endpoint
- [ ] Provider credential validate endpoint
- [ ] Usage request logs
- [ ] Usage history endpoint
- [ ] Usage chart/stats stream
- [ ] Request detail endpoint
- [ ] Model alias management
- [ ] Disabled model management
- [ ] Model availability checks
- [ ] Routing strategy management
- [ ] Retry/max retry interval management
- [ ] OAuth excluded models / model alias controls
- [ ] Auth file upload/download/index flows
- [ ] Vertex credential import flow
- [ ] Proxy pools / upstream pool manager
- [ ] Provider nodes / compatible node manager

### 4) Authentication/provider onboarding

- [x] Manual API key storage
- [x] OAuth token import groundwork (local-only API)`r`n- [x] OAuth token manual refresh endpoint`r`n- [x] OAuth token auto-refresh before request / one retry on 401`r`n- [x] OAuth PKCE browser flow groundwork for codex/gemini
- [x] Manual access token storage
- [x] Manual refresh token storage
- [ ] OAuth login flow for Claude
- [x] OAuth login flow for Codex/OpenAI
- [ ] OAuth login flow for Gemini CLI
- [ ] OAuth login flow for Antigravity
- [ ] OAuth login flow for Kimi
- [ ] OAuth login flow for xAI/Grok
- [ ] Browser-cookie/web-session onboarding
- [ ] Auto import from local IDE/CLI credentials
- [ ] Multi-account auth file management

### 5) Routing and platform capabilities

- [x] Basic fallback routing
- [x] Sticky/round-robin settings fields in DB
- [ ] Rich routing strategies like those advertised by OmniRoute
- [ ] Combo model system
- [ ] Multi-account weighted round-robin / quota-aware selection
- [ ] Provider/account-level policy assignment
- [ ] MCP endpoints/server integration
- [ ] A2A protocol endpoints
- [ ] Tunnel/remote exposure features
- [ ] CLI auto-config endpoints for external tools
- [ ] Dashboard/web UI
- [ ] Real-time logs/analytics UI

## Reference Repo Coverage Snapshot

### 9router

- Approx provider count found from provider constants: **86**
- Extra service kinds detected: `embedding`, `image`, `imageToText`, `tts`, `stt`, `webSearch`, `webFetch`, `video`, `music`
- Major extra API groups present: `combos`, `provider-nodes`, `proxy-pools`, `usage/logs/stats/stream`, `translator`, `tunnel`, `mcp`, `cli-tools`, media provider voice helpers

### OmniRoute

- Approx unique provider count parsed from provider constants: **197**
- Provider groups detected: `free`, `oauth`, `web-cookie`, `apikey`, `local`, `search`, `audio-only`, `upstream-proxy`, `cloud-agent`
- Large capability surface noted in repo structure/README: dashboard, MCP, A2A, provider metrics, pricing/catalog sync, search providers, upstream proxy assignment, many routing strategies

### CLIProxyAPI

- Core provider/auth families clearly present: `claude`, `codex`, `gemini`, `antigravity`, `kimi`, `vertex`, `xai`, plus OpenAI-compatible and Anthropic-compatible handling
- Major extra API/management surface: `chat/completions`, `completions`, `images`, `videos`, `messages/count_tokens`, `responses/compact`, `v1beta/models/*action`, OAuth callback flows, auth-files management, request retry, routing strategy, model alias/exclusion controls

## Priority Gap Backlog

### P1 - should implement first

- [ ] Add generic provider adapters for OpenAI-compatible, Anthropic-compatible, and Gemini-compatible upstreams
- [ ] Add `POST /v1/completions`
- [ ] Add `POST /v1/embeddings`
- [ ] Add `POST /v1/images/generations`
- [ ] Add `POST /v1/audio/speech`
- [ ] Add `POST /v1/audio/transcriptions`
- [x] Add `POST /v1/search`
- [ ] Add provider validate/test/models endpoints
- [ ] Add detailed per-request usage logs

### P2 - next expansion

- [ ] Add OAuth provider flows: `claude`, `codex`, `github`, `gemini-cli`, `antigravity`, `xai`
- [ ] Add account pool management and richer round-robin/fallback strategies
- [ ] Add model alias / disabled / availability management
- [ ] Add CLI-compatible aliases and advanced response variants

### P3 - platform parity layer

- [ ] Add web dashboard
- [ ] Add combo models and policy routing
- [ ] Add MCP support
- [ ] Add A2A support
- [ ] Add tunnel / remote access helpers

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

- **apikey (diff=1)**: Chỉ cần thêm `baseUrl` + `apiKey` header vào `resolveEndpoint`. Hầu hết đều OpenAI-compatible. Có thể batch thêm nhiều provider cùng lúc bằng cách mở rộng switch-case trong `forwarder.go`.
- **search (diff=1)**: Cần endpoint riêng `/v1/search`, body/response format khác chat completions. Nên tách handler.
- **audio (diff=2)**: Cần endpoint `/v1/audio/speech` và `/v1/audio/transcriptions`, format multipart/form-data cho STT.
- **oauth (diff=3)**: Cần OAuth flow riêng cho từng provider (PKCE, device code, hoặc browser redirect), lưu access/refresh token, auto-refresh. Đây là phần phức tạp nhất, nên làm sau khi apikey providers ổn định.

### Recommended implementation order

1. **Batch apikey providers** (#9-28): Tất cả đều OpenAI-compatible hoặc Anthropic-compatible, chỉ cần mở rộng `resolveEndpoint` với `baseUrl` catalog. Có thể làm trong 1-2 session.
2. **Search providers** (#8): Thêm `/v1/search` handler + adapter cho Brave, Serper, Tavily, Exa, Perplexity.
3. **Audio providers** (#29-30): Thêm `/v1/audio/speech` + `/v1/audio/transcriptions` handler.
4. **OAuth providers** (#1-7): Làm từng provider một, bắt đầu từ `gemini` (API key trước, OAuth sau), rồi `claude`, `codex`, `xai`, `antigravity`, `kimi`, `vertex`.

## Wave 1 Technical Roadmap (Implement Now)

Scope mục tiêu Wave 1:
- Chỉ làm nhóm provider `apikey`/compatible có tác động cao.
- Không đụng OAuth flow ở Wave 1.
- Không thêm dashboard/UI, chỉ API + router core.

Target providers cho Wave 1:
- `deepseek`, `groq`, `mistral`, `cerebras`, `fireworks`, `together`, `siliconflow`, `vercel-ai-gateway`, `cohere`, `perplexity`

Success criteria:
- [x] Tạo provider connection cho các provider trên qua `POST /api/providers` chạy được.
- [ ] Proxy thành công qua `POST /v1/chat/completions` với model prefix `provider/model`.
- [x] `GET /api/models` trả model hợp lệ (ít nhất gồm model map tĩnh fallback).
- [ ] Cooldown/backoff hoạt động đúng khi provider trả `429/5xx`.
- [x] `go test ./...` pass, `go build ./cmd/xrouter` pass.

### A. File changes and exact responsibilities

1) `C:\Dev\XRouter\internal\proxy\forwarder.go`
- Mở rộng `resolveEndpoint(...)`:
  - map `provider -> default baseUrl` cho nhóm Wave 1.
  - giữ ưu tiên `ProviderSpecificData.baseUrl` nếu user đã set.
- Mở rộng endpoint resolver theo path:
  - giữ nguyên `/v1/chat/completions`, `/v1/messages`, `/v1/responses`.
  - chuẩn bị sẵn nhánh an toàn cho `/v1/completions` (nếu body tương thích OpenAI).
- Bổ sung normalize nhẹ theo provider:
  - strip prefix `provider/model` như hiện có.
  - giữ default `openai` mode cho phần lớn provider apikey.
- Không đổi logic cooldown/circuit hiện tại (đã ổn định).

2) `C:\Dev\XRouter\internal\store\db.go`
- Bổ sung metadata helper cho provider catalog cục bộ:
  - danh sách provider id + default baseUrl + apiType default.
  - helper validate provider khi tạo/sửa connection (không bắt buộc cứng; cho phép custom provider nếu có baseUrl).
- Bổ sung fallback model map tĩnh tối thiểu theo provider (để `GET /api/models` có dữ liệu hữu ích khi upstream chưa gọi được).

3) `C:\Dev\XRouter\internal\app\server.go`
- Nâng `GET /api/models`:
  - merge model từ active connections + fallback static catalog của Wave 1.
  - giữ format response tương thích hiện tại.
- Nâng `POST /api/providers` và `PATCH /api/providers/{id}`:
  - auto-fill `baseUrl`/`apiType` từ catalog nếu thiếu.
  - không override nếu user đã set explicit `providerSpecificData.baseUrl`.

4) `C:\Dev\XRouter\internal\proxy\forwarder_test.go`
- Thêm test cases cho `resolveEndpoint`:
  - mỗi provider Wave 1 phải resolve được endpoint chat hợp lệ.
  - provider custom thiếu `baseUrl` phải fail đúng message.
- Thêm test normalize prefix `provider/model` cho ít nhất 3 provider mới.

5) `C:\Dev\XRouter\internal\app\server_models_test.go` (file mới, nhỏ)
- Test `GET /api/models` có merge fallback models cho provider Wave 1.
- Test không phá vỡ format cũ.

### B. Minimal provider catalog for Wave 1

Catalog đề xuất (default base URL, có thể override qua providerSpecificData):
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

Model fallback tối thiểu (để hiển thị và smoke):
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

- Risk: base URL đổi theo thời gian → luôn cho phép override bằng `providerSpecificData.baseUrl`.
- Risk: provider trả schema hơi khác OpenAI → Wave 1 giới hạn vào chat-completions compatible path.
- Risk: model ID đổi nhanh → fallback model chỉ để bootstrap/smoke, không hard guarantee.

### E. Done definition (strict)

- [ ] Có thể thêm 10 provider Wave 1 không cần nhập baseUrl thủ công.
- [ ] Proxy chat qua mỗi provider Wave 1 pass tối thiểu 1 request smoke.
- [ ] Không regression các provider cũ: `openai`, `anthropic`, `openrouter`.
- [ ] Toàn bộ test pass, build pass.

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
| `antigravity` | 9router, CLIProxyAPI, OmniRoute | core/oauth/sdk, oauth | NO | Need adapter/auth/base URL/model transform |
| `apikey` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `assemblyai` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `auto` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `aws-polly` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `azure` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `azure-ai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `azure-openai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `bailian-coding-plan` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `baseten` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `bazaarlink` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `bedrock` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `black-forest-labs` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `blackbox` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `blackbox-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `brave-search` | 9router, OmniRoute | apiKey, search | NO | Need adapter/auth/base URL/model transform |
| `byteplus` | 9router | freeTier | NO | Need adapter/auth/base URL/model transform |
| `bytez` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `cablyai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `cartesia` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `cerebras` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `chatgpt-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `chutes` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `clarifai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `claude` | 9router, CLIProxyAPI, OmniRoute | core/oauth/sdk, oauth | NO | Need adapter/auth/base URL/model transform |
| `cline` | 9router, OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `cliproxyapi` | OmniRoute | upstream_proxy | NO | Need adapter/auth/base URL/model transform |
| `cloudflare-ai` | 9router, OmniRoute | apikey, freeTier | NO | Need adapter/auth/base URL/model transform |
| `codestral` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `codex` | 9router, CLIProxyAPI, OmniRoute | core/oauth/sdk, oauth | NO | Need adapter/auth/base URL/model transform |
| `codex-cloud` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `cohere` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `comfyui` | 9router, OmniRoute | apiKey, local | NO | Need adapter/auth/base URL/model transform |
| `command-code` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `commandcode` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `completions` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `copilot-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `coqui` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `crof` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `cursor` | 9router, OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `databricks` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `datarobot` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `deepgram` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `deepinfra` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `deepseek` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `deepseek-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `devin` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `devin-cli` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `docker-model-runner` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `edge-tts` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `elevenlabs` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `empower` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `enally` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `exa` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `exa-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `fal-ai` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `featherless-ai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `fenayai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `firecrawl` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `fireworks` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `freetheai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `friendliai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `galadriel` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gemini` | 9router, CLIProxyAPI, OmniRoute | apikey, core/oauth/sdk, freeTier | NO | Need adapter/auth/base URL/model transform |
| `gemini-cli` | 9router, OmniRoute | free | NO | Need adapter/auth/base URL/model transform |
| `gemini-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `getgoapi` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gigachat` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `github` | 9router, OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `github-models` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gitlab` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gitlab-duo` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `gitlawb` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `gitlawb-gmi` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `glhf` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `glm` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `glm-cn` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `glmt` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `google-pse` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `google-pse-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `google-tts` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `grok` | CLIProxyAPI | core/oauth/sdk | NO | Need adapter/auth/base URL/model transform |
| `grok-web` | 9router, OmniRoute | webCookie, web_cookie | NO | Need adapter/auth/base URL/model transform |
| `groq` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `hackclub` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `haiper` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `heroku` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `huggingface` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `hyperbolic` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `ideogram` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `inference-net` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `inworld` | 9router, OmniRoute | apiKey, audio_only | NO | Need adapter/auth/base URL/model transform |
| `jina-ai` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `jina-reader` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `jules` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `kie` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `kilo-gateway` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `kilocode` | 9router, OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `kimi` | 9router, CLIProxyAPI, OmniRoute | apiKey, apikey, core/oauth/sdk | NO | Need adapter/auth/base URL/model transform |
| `kimi-coding` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `kimi-coding-apikey` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `kiro` | 9router, OmniRoute | free | NO | Need adapter/auth/base URL/model transform |
| `kluster` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `lambda-ai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
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
| `minimax` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `minimax-cn` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `mistral` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `modal` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `moonshot` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `morph` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `muse-spark-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `nanobanana` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `nanogpt` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `nebius` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `nlpcloud` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `nous-research` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `novita` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `nscale` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `nvidia` | 9router, OmniRoute | apikey, freeTier | NO | Need adapter/auth/base URL/model transform |
| `oauth` | OmniRoute | cloud_agent | NO | Need adapter/auth/base URL/model transform |
| `oci` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `ollama` | 9router | freeTier | NO | Need adapter/auth/base URL/model transform |
| `ollama-cloud` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `ollama-local` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `ollama-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `oobabooga` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `openai` | 9router, CLIProxyAPI, OmniRoute | apiKey, apikey, core/oauth/sdk | YES | Built-in/generic route |
| `openai-compatible` | CLIProxyAPI | core/oauth/sdk | YES | Built-in/generic route |
| `opencode` | 9router, OmniRoute | free | NO | Need adapter/auth/base URL/model transform |
| `opencode-go` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `opencode-zen` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `openrouter` | 9router, OmniRoute | apikey, freeTier | YES | Built-in/generic route |
| `ovhcloud` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `perplexity` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `perplexity-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `perplexity-web` | 9router, OmniRoute | webCookie, web_cookie | NO | Need adapter/auth/base URL/model transform |
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
| `qwen` | OmniRoute | free | NO | Need adapter/auth/base URL/model transform |
| `recraft` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `reka` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `replicate` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `runwayml` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `sambanova` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `sap` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `scaleway` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `sdwebui` | 9router, OmniRoute | apiKey, local | NO | Need adapter/auth/base URL/model transform |
| `searchapi` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `searchapi-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `searxng` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `searxng-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `serper` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `serper-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `siliconflow` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `snowflake` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `stability-ai` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `suno` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `synthetic` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `t3-web` | OmniRoute | web_cookie | NO | Need adapter/auth/base URL/model transform |
| `tavily` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `tavily-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `thebai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `together` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
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
| `vercel-ai-gateway` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `vertex` | 9router, CLIProxyAPI, OmniRoute | apikey, core/oauth/sdk, freeTier | NO | Need adapter/auth/base URL/model transform |
| `vertex-partner` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `vllm` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `volcengine` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `volcengine-ark` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `voyage-ai` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `wandb` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `watsonx` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `windsurf` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
| `xai` | 9router, CLIProxyAPI, OmniRoute | apiKey, apikey, core/oauth/sdk | NO | Need adapter/auth/base URL/model transform |
| `xiaomi-mimo` | 9router, OmniRoute | apiKey, apikey | NO | Need adapter/auth/base URL/model transform |
| `xiaomi-tokenplan` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `xinference` | OmniRoute | local | NO | Need adapter/auth/base URL/model transform |
| `youcom` | 9router | apiKey | NO | Need adapter/auth/base URL/model transform |
| `youcom-search` | OmniRoute | search | NO | Need adapter/auth/base URL/model transform |
| `zai` | OmniRoute | apikey | NO | Need adapter/auth/base URL/model transform |
| `zed` | OmniRoute | oauth | NO | Need adapter/auth/base URL/model transform |
## Notes on interpretation

- This matrix is intentionally conservative for `XRouter`.
- `YES` currently means either:
  - built-in routing exists directly (`openai`, `anthropic`, `openrouter`), or
  - the current code can already route a generic compatible provider class without adding a whole new provider-specific auth flow.
- `NO` means XRouter still needs at least one of: provider adapter logic, auth onboarding flow, model normalization, dedicated endpoint support, or management UX/API.







