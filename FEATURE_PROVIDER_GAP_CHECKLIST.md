# XRouter Router Parity Checklist - 2026-05-26

Legend: Done = implemented and tested locally; Partial = present but not as broad/deep as reference projects; Missing = no clear implementation found in XRouter.

## Core API compatibility

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| OpenAI-compatible `/v1/chat/completions` | OmniRoute, 9router, CLIProxyAPI | Done | `internal/app/scoped.go`, `internal/app/completions_test.go`, smoke test pass | P0 |
| OpenAI `/v1/completions` | CLIProxyAPI / OpenAI-compatible clients | Done | Routed in `internal/app/scoped.go` | P1 |
| Anthropic-compatible `/v1/messages` | OmniRoute, 9router, CLIProxyAPI | Done | `README.md`, scoped route, `internal/app/v1compat_test.go` | P0 |
| OpenAI Responses `/v1/responses` | OmniRoute, CLIProxyAPI | Done | `internal/app/stream_test.go`, `README.md` | P0 |
| `/v1/responses/stream` alias | CLIProxyAPI-like CLI compatibility | Done | `internal/app/stream_test.go` | P1 |
| Gemini-compatible forwarding | 9router, CLIProxyAPI | Done | `internal/proxy/forwarder_test.go`, `internal/app/gemini_test.go` | P0 |
| OpenAI <-> Gemini body transform | 9router, CLIProxyAPI | Done/Partial | Added request-side tools mapping and response-side Gemini `functionCall` -> OpenAI `tool_calls`; multimodal/image parts still need broader coverage | P1 |
| OpenAI <-> Claude body transform | 9router, CLIProxyAPI | Done/Partial | Added request transform plus Anthropic JSON/SSE response normalization OpenAI tools -> Anthropic tools mapping and Anthropic SSE text/tool-use -> OpenAI chunk normalization; broader multimodal parity still needs more audit | P1 |
| Ollama format translation | 9router | Done/Partial | Added OpenAI -> Ollama `/api/chat` request mapping plus Ollama JSON/NDJSON -> OpenAI chat response/chunk normalization; broader Ollama tool edge cases still need live testing | P2 |
| Function/tool calling pass-through | OmniRoute, CLIProxyAPI | Done/Partial | Gemini/Ollama/Anthropic request+response tool-call normalization added; broader provider coverage still partial | Added translator coverage for OpenAI tool declarations to Gemini/Anthropic and Gemini function-call response normalization | P1 |
| Multimodal text+image | CLIProxyAPI | Done/Partial | Added OpenAI text+data-URL image content translation to Gemini chat payloads; Anthropic path currently preserves text and skips image parts safely | P1 |
| Streaming pass-through | all 3 | Done/Partial | Added Gemini and Anthropic SSE -> OpenAI `chat.completion.chunk` normalization; other provider streaming parity still needs more tests | P0 |
| Provider-scoped compat routes `/api/provider/{provider}/...` | CLIProxyAPI | Done/Partial | Added real scoped routing for chat, responses direct/compact, count_tokens, media, search/fetch, voices, models with regression tests in `scoped_test.go` | P1 |
| Provider-scoped compat routes `/api/v1/providers/{provider}/...` | 9router, CLIProxyAPI | Done/Partial | Registered real route alias and regression tests to avoid falling through to root handler; added `/web/fetch` and `/audio/voices` alias coverage | P1 |

## Provider coverage

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| OpenAI, Anthropic, OpenRouter | all 3 | Done | `README.md` provider families | P0 |
| Claude/Codex/Gemini OAuth families | 9router, CLIProxyAPI | Done/Partial | OAuth endpoints and tests exist; provider-specific parity requires live OAuth validation | P0 |
| Vertex/Gemini advanced support | CLIProxyAPI | Done/Partial | `vertex_test.go`, `gemini_test.go`; not guaranteed equal to CLIProxyAPI | P1 |
| xAI/Grok provider | CLIProxyAPI | Partial | Catalog/provider listed; OAuth Grok Build parity not proven | P1 |
| GitHub/Copilot provider | 9router, OmniRoute | Partial | Catalog entries exist; no MITM/Copilot token refresh parity confirmed | P2 |
| Amp provider/module | CLIProxyAPI | Partial | `internal/app/amp.go` exists; CLIProxyAPI Amp module is broader | P1 |
| 50+ free providers | OmniRoute | Partial | Many catalog providers exist, but not OmniRoute 177/50+ parity | P2 |
| 177 providers | OmniRoute | Missing/Partial | XRouter catalog is broad but not 177-provider parity | P3 |
| Kilo/free model discovery | OmniRoute-like free routing | Done/Partial | `internal/app/kilo_test.go`, `/api/providers/kilo/free-models` | P2 |
| Generic OpenAI-compatible provider | all 3 | Done | README quickstart and tests | P0 |
| Generic Anthropic-compatible provider | all 3 | Done | README quickstart and tests | P0 |
| Generic Gemini-compatible provider | all 3 | Done | README quickstart and tests | P0 |
| Provider model probing `/models` | all 3 | Done | `internal/app/providercheck_test.go` | P1 |
| Batch provider test | dashboard/management parity | Done | `/api/providers/test-batch`, tests | P1 |

## Routing, fallback, quota, resilience

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| Combo model routing | OmniRoute, 9router | Done | `internal/app/combo_test.go` | P0 |
| Fallback to next target | OmniRoute, 9router | Done | `TestComboModelFallbacksToNextTarget` | P0 |
| Round-robin account selection | CLIProxyAPI, 9router | Done | `internal/app/weighted_test.go`, settings | P0 |
| Weighted routing | OmniRoute/CLIProxyAPI-like | Done | `TestWeightedQuotaAwareRoundRobin` | P0 |
| Sticky round robin | OmniRoute-style strategies | Done/Partial | Strategy endpoint supports sticky; not 14 strategies | P1 |
| Cost-optimized routing | OmniRoute | Done/Partial | Added `cost_optimized` candidate ordering using provider-level pricing hints; still much simpler than OmniRoute scoring engine | P2 |
| Auto routing / `auto` variants | OmniRoute | Done/Partial | Added minimal `auto` ordering based on cooldown status, recent success rate, average latency and pricing hints; not full OmniRoute 9-factor engine | P2 |
| 14 routing strategies | OmniRoute | Missing | XRouter now supports `fallback`, `round_robin`, `sticky_round_robin`, `cost_optimized`, `auto`, but still far below OmniRoute breadth | P2 |
| Circuit breaker | OmniRoute | Done/Partial | `internal/store/db_test.go` has circuit tests; parity depth unknown | P1 |
| Connection cooldown / Retry-After | OmniRoute, 9router | Done | `internal/proxy/forwarder_test.go` cooldown tests | P0 |
| Model lockout | OmniRoute | Partial | Disabled/excluded/availability models exist, but not exact dynamic lockout parity | P1 |
| Exponential backoff per account | 9router | Partial | Cooldown/backoff behavior exists; exact 9router account backoff parity not guaranteed | P1 |
| Retry config management | all 3 | Done | `internal/app/retry_test.go` | P0 |
| Provider availability tracking | OmniRoute | Done/Partial | `availability_test.go`, management endpoint | P1 |
| Quota summary | all 3 | Done | `/api/quota`, `/api/usage/summary`, smoke test | P0 |

## Management API and local dashboard

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| Provider CRUD | all 3 | Done | `README.md`, `route.go`, tests | P0 |
| API key CRUD | 9router/CLIProxyAPI dashboards | Done | `internal/app/keys_test.go` | P1 |
| Auth file import/manage | CLIProxyAPI/9router | Done | `internal/app/accounts_test.go` | P1 |
| OAuth provider list/import/start/callback | 9router, CLIProxyAPI | Done/Partial | `oauth_test.go`, `start.go`; live provider tests not run | P0 |
| Local OAuth scan | 9router/CLIProxyAPI | Done/Partial | `/api/oauth/providers/scan-local`, `localscan_test.go` | P1 |
| Cursor auto-import | 9router | Done/Partial | `/api/oauth/cursor/auto-import`, tests likely present | P2 |
| Provider node management | OmniRoute/advanced routing | Done | `route.go`, `nodes_test.go` | P1 |
| Proxy pool management | router management | Done | `route.go`, `pools_test.go` | P1 |
| Route policies | OmniRoute-like policy routing | Done/Partial | `policies_test.go`, `policyacct_test.go`; not full guardrail policy engine | P1 |
| Model aliases/mappings | all 3 | Done | `aliases_test.go`, `modelalias_test.go` | P0 |
| Disabled model management | OmniRoute/9router | Done | `disabled_test.go` | P1 |
| Usage stats/logs/history | all 3 | Done | `usage.go`, `stats_test.go`, `logs_test.go`, `history_test.go`, smoke test | P0 |
| Realtime usage stream | OmniRoute/9router dashboard | Done | `/api/usage/stream`, smoke dashboard | P1 |
| Embedded dashboard | OmniRoute/9router | Partial | Local minimal dashboard exists; not full Next.js/Electron dashboards | P2 |
| Debug DB endpoint | local admin | Done | `/api/debug/db` | P2 |
| Version/build metadata | international standard release | Done | `README.md`, `version_test.go` | P0 |

## CLI/tool integration

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| Claude Code compatibility | all 3 | Done/Partial | `/v1/messages`, Claude family, CLI config | P0 |
| Codex compatibility | all 3 | Done/Partial | Responses/OpenAI support, Codex family | P0 |
| Cursor compatibility | OmniRoute, 9router | Partial | Provider/catalog/auto-import exists; no MITM Cursor interception parity | P2 |
| Cline/Roo/Continue compatibility | 9router, OmniRoute | Partial | OpenAI-compatible endpoint likely works; no dedicated config templates confirmed | P2 |
| Antigravity compatibility | OmniRoute, 9router | Partial | Provider family listed; no MITM/desktop parity | P2 |
| Copilot compatibility | OmniRoute, 9router | Partial | Catalog entries; no Copilot MITM/token-refresh parity | P3 |
| `/api/cli/config` helper | CLI onboarding | Done | `route.go`, `cli_test.go` | P1 |
| MCP server registry | OmniRoute | Done/Partial | CRUD/list implemented; not OmniRoute 37 tools/3 transports parity | P2 |
| A2A agent registry/RPC | OmniRoute | Done/Partial | A2A CRUD/RPC implemented; full agent protocol parity unknown | P2 |
| ACP agent compatibility | agent protocols | Done/Partial | `/api/acp/agents` tests exist | P2 |

## Media, search, and extra OpenAI-compatible APIs

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| Embeddings | 9router, CLIProxyAPI | Done | Scoped route and forwarder media tests | P1 |
| Image generation | 9router/OmniRoute | Done | `images_test.go`, provider-scoped generation coverage, media resolver | P1 |
| Image edits | OpenAI-compatible media | Done/Partial | Added direct and provider-scoped route coverage; broader provider-specific payload variants still partial | P2 |
| Audio speech/TTS | 9router/CLIProxyAPI | Done | `voices_test.go`, forwarder media tests, provider-scoped audio generations/speech/transcriptions coverage | P1 |
| Audio transcriptions/STT | 9router/CLIProxyAPI | Done/Partial | Added direct, `/api/v1`, AssemblyAI flow, and provider-scoped STT tests; broader provider set still partial | P1 |
| Video generations/edits/extensions | OmniRoute-like extended API | Done/Partial | `video_test.go`, `videoextra_test.go`, provider-scoped generations/edits/extensions coverage; deeper variants still partial | P2 |
| Web search providers | OmniRoute-like helpers | Done/Partial | `websearch_test.go`, Brave/Serper normalization tests | P2 |
| Fetch/files helper APIs | tool ecosystem | Done | `fetch_test.go`, `files_test.go` | P2 |
| Music/media extra APIs | extended providers | Done/Partial | `music_test.go`, `media_test.go` | P3 |

## Security, auth, deployment, operations

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| Require API key setting | all 3 | Done | `/api/settings`, smoke shows `requireApiKey:true` | P0 |
| Local-only management protection | all 3 | Done/Partial | Added localhost-only guard for usage/debug endpoints; remaining audit mainly concerns non-management read-only surfaces | P0 |
| Dashboard login/session | 9router | Partial | `requireLogin` setting exists, but not full 9router NextAuth/session UI parity | P2 |
| Outbound proxy settings | CLIProxyAPI/9router | Done/Partial | Added `outboundNoProxy` bypass matching for localhost, exact host, suffix and wildcard domains; env parity still partial | P1 |
| WebSocket relay/gateway | CLIProxyAPI | Done/Partial | Added minimal `/api/v1/ws` relay with ping/pong, HTTP/stream messages, plus native JSON body support for `http_request`; still not as full-featured as CLIProxyAPI wsrelay manager | P1 |
| Redis usage output/queue | CLIProxyAPI | Missing | No clear Redis queue protocol parity found | P3 |
| SDK embeddability | CLIProxyAPI | Missing | No public reusable Go SDK package comparable to CLIProxyAPI sdk | P3 |
| Docker/self-host docs | all 3 | Partial/Missing | Release/CI exists; Docker parity not seen in README | P3 |
| Cross-platform release artifacts | OmniRoute/CLIProxyAPI | Done | README release workflow states Linux/Windows/macOS builds | P1 |
| Graceful shutdown | standard Go | Done | `cmd/xrouter/main.go` | P0 |
| HTTP server timeouts | standard Go | Done | `cmd/xrouter/main.go` | P0 |
| Gofmt/go vet/go test/build CI | standard Go | Done | `README.md`, local validation pass | P0 |

## Compression / optimization / advanced OmniRoute features

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| RTK compression | OmniRoute | Missing | No implementation found | P3 |
| Caveman compression | OmniRoute | Missing | No implementation found | P3 |
| Automatic token-saving pipeline | OmniRoute | Done/Partial | Added opt-in request compaction via `/compact` paths or `xrouter_compact`; not full OmniRoute RTK/Caveman | P3 |
| Context relay / context optimized routing | OmniRoute | Missing | No evidence of context handoff strategy | P3 |
| Last-known-good-provider routing | OmniRoute | Done/Partial | Added `last_known_good` strategy using most recent successful provider history from request logs | P2 |
| Latency/success/freshness scoring | OmniRoute | Done/Partial | Added freshness bonus from most recent successful request timestamp alongside success rate and latency; still simpler than OmniRoute engine | P2 |
| Guardrails/evals | OmniRoute | Missing | No clear guardrail/eval subsystem | P3 |
| TLS stealth | OmniRoute | Missing | No clear TLS fingerprint/stealth transport implementation | P3 |

## 9router-specific local features

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| Next.js dashboard clone | 9router | Missing | XRouter has embedded minimal dashboard only | P3 |
| MITM server | 9router | Missing | No cert/root CA/MITM handlers in XRouter | P3 |
| Cursor MITM handler | 9router | Missing | No equivalent handler found | P3 |
| Copilot MITM handler | 9router | Missing | No equivalent handler found | P3 |
| Kiro MITM handler | 9router | Missing | No equivalent handler found | P3 |
| Antigravity MITM handler | 9router | Missing | No equivalent handler found | P3 |
| Local app updater | 9router | Missing | No updater subsystem seen | P3 |
| i18n website/docs | 9router/OmniRoute | Missing | Not needed for core router | P4 |

## CLIProxyAPI-specific advanced features

| Feature | Reference source | XRouter status | Evidence / note | Priority |
|---|---|---:|---|---:|
| Go SDK packages | CLIProxyAPI | Missing | XRouter is app-oriented internal packages | P3 |
| Translator registry | CLIProxyAPI | Partial/Missing | Forwarder has normalization helpers, not a public registry | P2 |
| Pipeline middleware | CLIProxyAPI | Missing/Partial | No comparable SDK pipeline abstraction | P3 |
| WebSocket downstream transport | CLIProxyAPI | Done/Partial | `/api/v1/ws` supports downstream HTTP/stream relay, but still lacks a fuller dedicated wsrelay subsystem | P1 |
| Amp provider aliases `/api/provider/{provider}/...` | CLIProxyAPI | Done/Partial | Core provider-scoped aliases now work for major chat/media/model paths, but full CLIProxyAPI breadth still needs more audit | P1 |
| Redis protocol integration | CLIProxyAPI | Missing | No Redis protocol endpoints found | P3 |
| Usage plugin manager | CLIProxyAPI | Partial/Missing | XRouter has local usage DB/stats, not plugin manager | P3 |

## Recommended implementation order

1. P0 hardening: add/expand integration tests for Claude/Gemini/OpenAI streaming, tool calls, multimodal chat, and management localhost auth.
2. P1 parity: WebSocket relay, fuller translator coverage OpenAI/Claude/Gemini, live OAuth validation flows, model lockout/account backoff parity.
3. P2 routing: Auto routing, cost optimizer, latency/success/freshness scoring, provider health-driven routing, LKGP strategy.
4. P3 breadth: RTK/Caveman compression, MITM handlers, SDK extraction, Redis/usage plugins, Docker/self-host docs.
5. P4 polish: i18n/large website/dashboard parity only if product direction requires it.

## Local validation performed

- `gofmt -l .` clean.
- `go vet ./...` pass.
- `go test ./...` pass.
- `go build ./cmd/xrouter` pass.
- Smoke server test pass: `/api/health`, `/api/version`, `/api/settings`, `/api/models`, `/api/usage/stats`, `/dashboard`.







