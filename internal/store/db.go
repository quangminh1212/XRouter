package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ProviderCatalogEntry struct {
	Provider       string   `json:"provider"`
	AuthType       string   `json:"authType,omitempty"`
	APIType        string   `json:"apiType"`
	BaseURL        string   `json:"baseUrl"`
	AuthorizeURL   string   `json:"authorizeUrl,omitempty"`
	TokenURL       string   `json:"tokenUrl,omitempty"`
	ClientID       string   `json:"clientId,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	FallbackModels []string `json:"fallbackModels,omitempty"`
}

var providerCatalog = map[string]ProviderCatalogEntry{
	"openai":               {Provider: "openai", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.openai.com"},
	"anthropic":            {Provider: "anthropic", AuthType: "apikey", APIType: "anthropic", BaseURL: "https://api.anthropic.com"},
	"openrouter":           {Provider: "openrouter", AuthType: "apikey", APIType: "openai", BaseURL: "https://openrouter.ai/api"},
	"azure":                {Provider: "azure", AuthType: "apikey", APIType: "openai", FallbackModels: []string{"azure/gpt-4o-mini"}},
	"openai-compatible":    {Provider: "openai-compatible", AuthType: "apikey", APIType: "openai"},
	"anthropic-compatible": {Provider: "anthropic-compatible", AuthType: "apikey", APIType: "anthropic"},
	"gemini-compatible":    {Provider: "gemini-compatible", AuthType: "apikey", APIType: "gemini"},
	"deepseek":             {Provider: "deepseek", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.deepseek.com", FallbackModels: []string{"deepseek/deepseek-chat"}},
	"groq":                 {Provider: "groq", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.groq.com/openai", FallbackModels: []string{"groq/llama-3.1-70b-versatile"}},
	"mistral":              {Provider: "mistral", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.mistral.ai", FallbackModels: []string{"mistral/mistral-large-latest"}},
	"cerebras":             {Provider: "cerebras", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.cerebras.ai", FallbackModels: []string{"cerebras/llama3.1-70b"}},
	"fireworks":            {Provider: "fireworks", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.fireworks.ai/inference/v1", FallbackModels: []string{"fireworks/accounts/fireworks/models/llama-v3p1-70b-instruct"}},
	"together":             {Provider: "together", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.together.xyz/v1", FallbackModels: []string{"together/meta-llama/Llama-3.1-70B-Instruct-Turbo"}},
	"siliconflow":          {Provider: "siliconflow", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.siliconflow.cn/v1", FallbackModels: []string{"siliconflow/Qwen/Qwen2.5-Coder-32B-Instruct"}},
	"vercel-ai-gateway":    {Provider: "vercel-ai-gateway", AuthType: "apikey", APIType: "openai", BaseURL: "https://ai-gateway.vercel.sh/v1", FallbackModels: []string{"vercel-ai-gateway/openai/gpt-4o-mini"}},
	"cohere":               {Provider: "cohere", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.cohere.com/compatibility/v1", FallbackModels: []string{"cohere/command-r-plus"}},
	"perplexity":           {Provider: "perplexity", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.perplexity.ai", FallbackModels: []string{"perplexity/sonar-pro"}},
	"nvidia":               {Provider: "nvidia", AuthType: "apikey", APIType: "openai", BaseURL: "https://integrate.api.nvidia.com/v1", FallbackModels: []string{"nvidia/deepseek-ai/deepseek-v4-flash"}},
	"huggingface":          {Provider: "huggingface", AuthType: "apikey", APIType: "openai", BaseURL: "https://router.huggingface.co/v1", FallbackModels: []string{"huggingface/deepseek-ai/DeepSeek-V3-0324:fastest"}},
	"minimax":              {Provider: "minimax", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.minimax.io/v1", FallbackModels: []string{"minimax/MiniMax-M2.7"}},
	"glm":                  {Provider: "glm", AuthType: "apikey", APIType: "openai", BaseURL: "https://open.bigmodel.cn/api/paas/v4", FallbackModels: []string{"glm/glm-4.7"}},
	"glm-cn":               {Provider: "glm-cn", AuthType: "apikey", APIType: "openai", BaseURL: "https://open.bigmodel.cn/api/paas/v4", FallbackModels: []string{"glm-cn/glm-4.7"}},
	"minimax-cn":           {Provider: "minimax-cn", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.minimax.chat/v1", FallbackModels: []string{"minimax-cn/MiniMax-Text-01"}},
	"moonshot":             {Provider: "moonshot", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.moonshot.ai/v1", FallbackModels: []string{"moonshot/moonshot-v1-8k"}},
	"hyperbolic":           {Provider: "hyperbolic", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.hyperbolic.xyz/v1", FallbackModels: []string{"hyperbolic/meta-llama/Meta-Llama-3.1-70B-Instruct"}},
	"byteplus":             {Provider: "byteplus", AuthType: "apikey", APIType: "openai", BaseURL: "https://ark.ap-southeast.bytepluses.com/api/coding/v3", FallbackModels: []string{"byteplus/deepseek-v3-1-250821"}},
	"novita":               {Provider: "novita", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.novita.ai/v3/openai", FallbackModels: []string{"novita/deepseek/deepseek-v3"}},
	"sambanova":            {Provider: "sambanova", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.sambanova.ai/v1", FallbackModels: []string{"sambanova/Meta-Llama-3.1-70B-Instruct"}},
	"chutes":               {Provider: "chutes", AuthType: "apikey", APIType: "openai", BaseURL: "https://llm.chutes.ai/v1", FallbackModels: []string{"chutes/deepseek-ai/DeepSeek-V3-0324"}},
	"lambda-ai":            {Provider: "lambda-ai", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.lambda.ai/v1", FallbackModels: []string{"lambda-ai/hermes-3-llama-3.1-405b-fp8"}},
	"featherless-ai":       {Provider: "featherless-ai", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.featherless.ai/v1", FallbackModels: []string{"featherless-ai/Qwen/Qwen2.5-Coder-32B-Instruct"}},
	"kluster":              {Provider: "kluster", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.kluster.ai/v1", FallbackModels: []string{"kluster/meta-llama/Meta-Llama-3.1-70B-Instruct"}},
	"nebius":               {Provider: "nebius", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.studio.nebius.ai/v1", FallbackModels: []string{"nebius/Qwen/Qwen3-Coder-480B-A35B-Instruct"}},
	"clarifai":             {Provider: "clarifai", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.clarifai.com/v2/ext/openai/v1", FallbackModels: []string{"clarifai/openai/chat-completion/models/gpt-4o-mini"}},
	"reka":                 {Provider: "reka", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.reka.ai/v1", FallbackModels: []string{"reka/reka-core"}},
	"zai":                  {Provider: "zai", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.z.ai/api/paas/v4", FallbackModels: []string{"zai/glm-4.7"}},
	"qwen":                 {Provider: "qwen", AuthType: "apikey", APIType: "openai", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", FallbackModels: []string{"qwen/qwen-plus"}},
	"opencode":             {Provider: "opencode", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.opencode.ai/v1", FallbackModels: []string{"opencode/gpt-4o-mini"}},
	"opencode-go":          {Provider: "opencode-go", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.opencode.ai/v1", FallbackModels: []string{"opencode-go/gpt-4o-mini"}},
	"opencode-zen":         {Provider: "opencode-zen", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.opencode.ai/v1", FallbackModels: []string{"opencode-zen/gpt-4o-mini"}},
	"kiro":                 {Provider: "kiro", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.kiro.dev/v1", FallbackModels: []string{"kiro/kiro-pro"}},
	"voyage-ai":            {Provider: "voyage-ai", AuthType: "apikey", APIType: "embedding", BaseURL: "https://api.voyageai.com/v1"},
	"jina-ai":              {Provider: "jina-ai", AuthType: "apikey", APIType: "embedding", BaseURL: "https://api.jina.ai/v1"},
	"openai-tts":           {Provider: "openai-tts", AuthType: "apikey", APIType: "tts", BaseURL: "https://api.openai.com/v1"},
	"elevenlabs":           {Provider: "elevenlabs", AuthType: "apikey", APIType: "tts", BaseURL: "https://api.elevenlabs.io/v1/text-to-speech", FallbackModels: []string{"elevenlabs/eleven_multilingual_v2"}},
	"cartesia":             {Provider: "cartesia", AuthType: "apikey", APIType: "tts", BaseURL: "https://api.cartesia.ai/tts/bytes", FallbackModels: []string{"cartesia/sonic-2"}},
	"assemblyai":           {Provider: "assemblyai", AuthType: "apikey", APIType: "stt", BaseURL: "https://api.assemblyai.com/v2/transcript", FallbackModels: []string{"assemblyai/universal-3-pro"}},
	"deepgram":             {Provider: "deepgram", AuthType: "apikey", APIType: "stt", BaseURL: "https://api.deepgram.com/v1"},
	"brave-search":         {Provider: "brave-search", AuthType: "apikey", APIType: "search", BaseURL: "https://api.search.brave.com/res/v1"},
	"serper":               {Provider: "serper", AuthType: "apikey", APIType: "search", BaseURL: "https://google.serper.dev"},
	"serper-search":        {Provider: "serper-search", AuthType: "apikey", APIType: "search", BaseURL: "https://google.serper.dev"},
	"tavily":               {Provider: "tavily", AuthType: "apikey", APIType: "search", BaseURL: "https://api.tavily.com"},
	"tavily-search":        {Provider: "tavily-search", AuthType: "apikey", APIType: "search", BaseURL: "https://api.tavily.com"},
	"exa":                  {Provider: "exa", AuthType: "apikey", APIType: "search", BaseURL: "https://api.exa.ai"},
	"exa-search":           {Provider: "exa-search", AuthType: "apikey", APIType: "search", BaseURL: "https://api.exa.ai"},
	"perplexity-search":    {Provider: "perplexity-search", AuthType: "apikey", APIType: "search", BaseURL: "https://api.perplexity.ai"},
	"google-pse-search":    {Provider: "google-pse-search", AuthType: "apikey", APIType: "search", BaseURL: "https://customsearch.googleapis.com/customsearch/v1"},
	"cloudflare-ai":        {Provider: "cloudflare-ai", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.cloudflare.com/client/v4/accounts/{accountId}/ai/v1"},
	"black-forest-labs":    {Provider: "black-forest-labs", AuthType: "apikey", APIType: "image", BaseURL: "https://api.bfl.ai", FallbackModels: []string{"black-forest-labs/flux-2-pro"}},
	"chatgpt-web":          {Provider: "chatgpt-web", AuthType: "web_cookie", APIType: "openai", BaseURL: "https://chatgpt.com/backend-api", FallbackModels: []string{"chatgpt-web/gpt-4o-mini"}},
	"gemini-web":           {Provider: "gemini-web", AuthType: "web_cookie", APIType: "openai", BaseURL: "https://gemini.google.com", FallbackModels: []string{"gemini-web/gemini-1.5-flash"}},
	"deepseek-web":         {Provider: "deepseek-web", AuthType: "web_cookie", APIType: "openai", BaseURL: "https://chat.deepseek.com", FallbackModels: []string{"deepseek-web/deepseek-chat"}},
	"grok-web":             {Provider: "grok-web", AuthType: "web_cookie", APIType: "openai", BaseURL: "https://grok.com", FallbackModels: []string{"grok-web/grok-2-latest"}},
	"perplexity-web":       {Provider: "perplexity-web", AuthType: "web_cookie", APIType: "openai", BaseURL: "https://www.perplexity.ai", FallbackModels: []string{"perplexity-web/sonar-pro"}},
	"copilot-web":          {Provider: "copilot-web", AuthType: "web_cookie", APIType: "openai", BaseURL: "https://copilot.microsoft.com", FallbackModels: []string{"copilot-web/gpt-4o-mini"}},
	"claude":               {Provider: "claude", AuthType: "oauth", APIType: "anthropic", BaseURL: "https://api.anthropic.com", AuthorizeURL: "https://console.anthropic.com/v1/oauth/authorize", TokenURL: "https://console.anthropic.com/v1/oauth/token", ClientID: "claude-cli", Scopes: []string{"openid", "profile", "email", "offline_access"}, FallbackModels: []string{"claude/claude-3-5-sonnet-latest"}},
	"codex":                {Provider: "codex", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.openai.com", AuthorizeURL: "https://auth.openai.com/oauth/authorize", TokenURL: "https://auth.openai.com/oauth/token", ClientID: "codex-cli", Scopes: []string{"openid", "profile", "email", "offline_access"}, FallbackModels: []string{"codex/gpt-4o-mini"}},
	"github":               {Provider: "github", AuthType: "oauth", APIType: "openai", BaseURL: "https://models.inference.ai.azure.com", FallbackModels: []string{"github/gpt-4o-mini"}},
	"gemini":               {Provider: "gemini", AuthType: "oauth", APIType: "openai", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", ClientID: "gemini-cli", Scopes: []string{"https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/userinfo.email"}, FallbackModels: []string{"gemini/gemini-1.5-flash"}},
	"gemini-cli":           {Provider: "gemini-cli", AuthType: "oauth", APIType: "openai", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", ClientID: "gemini-cli", Scopes: []string{"https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/userinfo.email"}, FallbackModels: []string{"gemini-cli/gemini-1.5-flash"}},
	"vertex":               {Provider: "vertex", AuthType: "oauth", APIType: "openai", BaseURL: "https://aiplatform.googleapis.com/v1beta1", FallbackModels: []string{"vertex/gemini-1.5-flash"}},
	"xai":                  {Provider: "xai", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.x.ai", TokenURL: "https://accounts.x.ai/oauth/token", FallbackModels: []string{"xai/grok-2-latest"}},
	"grok":                 {Provider: "grok", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.x.ai", TokenURL: "https://accounts.x.ai/oauth/token", FallbackModels: []string{"grok/grok-2-latest"}},
	"antigravity":          {Provider: "antigravity", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.antigravity.ai/v1", AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", Scopes: []string{"openid", "profile", "email", "offline_access"}},
	"kimi":                 {Provider: "kimi", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.moonshot.ai", TokenURL: "https://www.kimi.com/api/oauth/token", FallbackModels: []string{"kimi/moonshot-v1-8k"}},
}

type ProviderConnection struct {
	ID                   string                 `json:"id"`
	Provider             string                 `json:"provider"`
	Name                 string                 `json:"name"`
	AuthType             string                 `json:"authType"`
	APIKey               string                 `json:"apiKey,omitempty"`
	AccessToken          string                 `json:"accessToken,omitempty"`
	RefreshToken         string                 `json:"refreshToken,omitempty"`
	TokenExpiry          string                 `json:"tokenExpiry,omitempty"`
	IsActive             bool                   `json:"isActive"`
	Priority             int                    `json:"priority"`
	GlobalPriority       *int                   `json:"globalPriority,omitempty"`
	AccountWeight        int                    `json:"accountWeight,omitempty"`
	RequestsPerMinute    int                    `json:"requestsPerMinute,omitempty"`
	DefaultModel         string                 `json:"defaultModel,omitempty"`
	ExcludedModels       []string               `json:"excludedModels,omitempty"`
	ModelAliases         map[string]string      `json:"modelAliases,omitempty"`
	ProviderSpecificData map[string]interface{} `json:"providerSpecificData,omitempty"`
	AccountName          string                 `json:"accountName,omitempty"`
	AccountEmail         string                 `json:"accountEmail,omitempty"`
	RateLimitedUntil     string                 `json:"rateLimitedUntil,omitempty"`
	BackoffLevel         int                    `json:"backoffLevel,omitempty"`
	ConsecutiveFailures  int                    `json:"consecutiveFailures,omitempty"`
	CircuitOpenUntil     string                 `json:"circuitOpenUntil,omitempty"`
	LastError            string                 `json:"lastError,omitempty"`
	ErrorCode            int                    `json:"errorCode,omitempty"`
	TestStatus           string                 `json:"testStatus,omitempty"`
	CreatedAt            string                 `json:"createdAt,omitempty"`
	UpdatedAt            string                 `json:"updatedAt,omitempty"`
}

type APIKey struct {
	ID                string `json:"id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	RequestsPerMinute int    `json:"requestsPerMinute,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
}

type UsageEntry struct {
	Timestamp        string  `json:"timestamp,omitempty"`
	Provider         string  `json:"provider,omitempty"`
	Model            string  `json:"model,omitempty"`
	TotalCost        float64 `json:"totalCost,omitempty"`
	PromptTokens     int64   `json:"promptTokens,omitempty"`
	CompletionTokens int64   `json:"completionTokens,omitempty"`
}

type UsageData struct {
	History               []UsageEntry            `json:"history"`
	TotalRequestsLifetime int64                   `json:"totalRequestsLifetime"`
	DailySummary          map[string]DailySummary `json:"dailySummary"`
}

type RequestLog struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp,omitempty"`
	Path          string `json:"path,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	APIKeyID      string `json:"apiKeyId,omitempty"`
	StatusCode    int    `json:"statusCode,omitempty"`
	LatencyMs     int64  `json:"latencyMs,omitempty"`
	RequestBytes  int    `json:"requestBytes,omitempty"`
	ResponseBytes int    `json:"responseBytes,omitempty"`
	Error         string `json:"error,omitempty"`
}

type AuthFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider,omitempty"`
	AccountName  string `json:"accountName,omitempty"`
	AccountEmail string `json:"accountEmail,omitempty"`
	ContentB64   string `json:"contentB64,omitempty"`
	Size         int    `json:"size,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

type ProxyPool struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ConnectionIDs []string `json:"connectionIds,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
	UpdatedAt     string   `json:"updatedAt,omitempty"`
}

type ProviderNode struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Provider      string   `json:"provider,omitempty"`
	ConnectionIDs []string `json:"connectionIds,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
	UpdatedAt     string   `json:"updatedAt,omitempty"`
}

type ComboModel struct {
	Alias     string   `json:"alias"`
	Targets   []string `json:"targets,omitempty"`
	CreatedAt string   `json:"createdAt,omitempty"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
}

type RoutePolicy struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ModelPrefix  string   `json:"modelPrefix,omitempty"`
	Providers    []string `json:"providers,omitempty"`
	Accounts     []string `json:"accounts,omitempty"`
	TargetPoolID string   `json:"targetPoolId,omitempty"`
	TargetNodeID string   `json:"targetNodeId,omitempty"`
	ForceModel   string   `json:"forceModel,omitempty"`
	CreatedAt    string   `json:"createdAt,omitempty"`
	UpdatedAt    string   `json:"updatedAt,omitempty"`
}

type MCPServer struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Transport string            `json:"transport,omitempty"`
	Command   string            `json:"command,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Enabled   bool              `json:"enabled"`
	CreatedAt string            `json:"createdAt,omitempty"`
	UpdatedAt string            `json:"updatedAt,omitempty"`
}

type A2AAgent struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	URL          string            `json:"url"`
	Protocol     string            `json:"protocol,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Enabled      bool              `json:"enabled"`
	CreatedAt    string            `json:"createdAt,omitempty"`
	UpdatedAt    string            `json:"updatedAt,omitempty"`
}

type TunnelEndpoint struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Provider    string            `json:"provider,omitempty"`
	PublicURL   string            `json:"publicUrl"`
	LocalTarget string            `json:"localTarget,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	Region      string            `json:"region,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   string            `json:"createdAt,omitempty"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
}

type DailySummary struct {
	Requests int64   `json:"requests"`
	Cost     float64 `json:"cost"`
}

type Settings struct {
	RequireAPIKey              bool              `json:"requireApiKey"`
	RequireLogin               bool              `json:"requireLogin"`
	StickyRoundRobinLimit      int               `json:"stickyRoundRobinLimit"`
	ComboStrategy              string            `json:"comboStrategy"`
	ComboStickyRoundRobinLimit int               `json:"comboStickyRoundRobinLimit"`
	OutboundProxyEnabled       bool              `json:"outboundProxyEnabled"`
	OutboundProxyURL           string            `json:"outboundProxyUrl"`
	OutboundNoProxy            string            `json:"outboundNoProxy"`
	ObservabilityEnabled       bool              `json:"observabilityEnabled"`
	ObservabilityMaxRecords    int               `json:"observabilityMaxRecords"`
	ForcedModelMappings        map[string]string `json:"forcedModelMappings,omitempty"`
	DisabledModels             []string          `json:"disabledModels,omitempty"`
	ModelAvailability          map[string]string `json:"modelAvailability,omitempty"`
	MaxRetries                 int               `json:"maxRetries,omitempty"`
	MaxCooldownSeconds         int               `json:"maxCooldownSeconds,omitempty"`
	DefaultRequestsPerMinute   int               `json:"defaultRequestsPerMinute,omitempty"`
}

type DB struct {
	ProviderConnections []ProviderConnection   `json:"providerConnections"`
	APIKeys             []APIKey               `json:"apiKeys"`
	Settings            Settings               `json:"settings"`
	ModelAliases        map[string]string      `json:"modelAliases"`
	Pricing             map[string]interface{} `json:"pricing"`
	UsageData           UsageData              `json:"usageData"`
	RequestLogs         []RequestLog           `json:"requestLogs"`
	AuthFiles           []AuthFile             `json:"authFiles"`
	ProxyPools          []ProxyPool            `json:"proxyPools"`
	ProviderNodes       []ProviderNode         `json:"providerNodes"`
	ComboModels         []ComboModel           `json:"comboModels"`
	RoutePolicies       []RoutePolicy          `json:"routePolicies"`
	MCPServers          []MCPServer            `json:"mcpServers"`
	A2AAgents           []A2AAgent             `json:"a2aAgents"`
	TunnelEndpoints     []TunnelEndpoint       `json:"tunnelEndpoints"`
}

type Store struct {
	mu       sync.RWMutex
	db       DB
	rawRoot  map[string]json.RawMessage
	dbPath   string
	loadedAt time.Time
}

func DataDir() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "xlabrouter")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".xlabrouter")
}

func NewStore() (*Store, error) {
	dir := DataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{dbPath: filepath.Join(dir, "db.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.dbPath)
	if os.IsNotExist(err) {
		s.db = defaultDB()
		s.rawRoot = map[string]json.RawMessage{}
		s.loadedAt = time.Now()
		return nil
	}
	if err != nil {
		return fmt.Errorf("read db.json: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse db.json root: %w", err)
	}
	var db DB
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("parse db.json typed: %w", err)
	}
	if db.ModelAliases == nil {
		db.ModelAliases = map[string]string{}
	}
	if db.Pricing == nil {
		db.Pricing = map[string]interface{}{}
	}
	if db.UsageData.DailySummary == nil {
		db.UsageData.DailySummary = map[string]DailySummary{}
	}
	if db.Settings.ForcedModelMappings == nil {
		db.Settings.ForcedModelMappings = map[string]string{}
	}
	if db.Settings.DisabledModels == nil {
		db.Settings.DisabledModels = []string{}
	}
	if db.Settings.ModelAvailability == nil {
		db.Settings.ModelAvailability = map[string]string{}
	}
	if db.RequestLogs == nil {
		db.RequestLogs = []RequestLog{}
	}
	if db.AuthFiles == nil {
		db.AuthFiles = []AuthFile{}
	}
	if db.ProxyPools == nil {
		db.ProxyPools = []ProxyPool{}
	}
	if db.ProviderNodes == nil {
		db.ProviderNodes = []ProviderNode{}
	}
	if db.ComboModels == nil {
		db.ComboModels = []ComboModel{}
	}
	if db.RoutePolicies == nil {
		db.RoutePolicies = []RoutePolicy{}
	}
	if db.MCPServers == nil {
		db.MCPServers = []MCPServer{}
	}
	if db.A2AAgents == nil {
		db.A2AAgents = []A2AAgent{}
	}
	if db.TunnelEndpoints == nil {
		db.TunnelEndpoints = []TunnelEndpoint{}
	}
	s.db = db
	s.rawRoot = root
	s.loadedAt = time.Now()
	return nil
}

func defaultDB() DB {
	return DB{
		ProviderConnections: []ProviderConnection{},
		APIKeys:             []APIKey{},
		Settings: Settings{
			RequireLogin:               true,
			StickyRoundRobinLimit:      3,
			ComboStrategy:              "fallback",
			ComboStickyRoundRobinLimit: 1,
			ObservabilityEnabled:       true,
			ObservabilityMaxRecords:    1000,
			ForcedModelMappings:        map[string]string{},
			DisabledModels:             []string{},
			ModelAvailability:          map[string]string{},
			MaxRetries:                 0,
			MaxCooldownSeconds:         90,
		},
		ModelAliases:    map[string]string{},
		Pricing:         map[string]interface{}{},
		UsageData:       UsageData{History: []UsageEntry{}, TotalRequestsLifetime: 0, DailySummary: map[string]DailySummary{}},
		RequestLogs:     []RequestLog{},
		AuthFiles:       []AuthFile{},
		ProxyPools:      []ProxyPool{},
		ProviderNodes:   []ProviderNode{},
		ComboModels:     []ComboModel{},
		RoutePolicies:   []RoutePolicy{},
		MCPServers:      []MCPServer{},
		A2AAgents:       []A2AAgent{},
		TunnelEndpoints: []TunnelEndpoint{},
	}
}

func GetProviderCatalogEntry(provider string) (ProviderCatalogEntry, bool) {
	entry, ok := providerCatalog[strings.ToLower(strings.TrimSpace(provider))]
	return entry, ok
}

func GetFallbackModels() []map[string]string {
	out := make([]map[string]string, 0, 16)
	for _, entry := range providerCatalog {
		for _, model := range entry.FallbackModels {
			out = append(out, map[string]string{"fullModel": model, "alias": ""})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["fullModel"] < out[j]["fullModel"]
	})
	return out
}

func ListProviderCatalogEntries() []ProviderCatalogEntry {
	out := make([]ProviderCatalogEntry, 0, len(providerCatalog))
	for _, entry := range providerCatalog {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Provider < out[j].Provider
	})
	return out
}

func applyProviderDefaults(c ProviderConnection) ProviderConnection {
	c.AccountName = strings.TrimSpace(c.AccountName)
	c.AccountEmail = strings.TrimSpace(c.AccountEmail)
	entry, ok := GetProviderCatalogEntry(c.Provider)
	if !ok {
		return c
	}
	if c.ProviderSpecificData == nil {
		c.ProviderSpecificData = map[string]interface{}{}
	}
	if strings.TrimSpace(c.AuthType) == "" {
		c.AuthType = entry.AuthType
		if c.AuthType == "" {
			c.AuthType = "apikey"
		}
	}
	if _, ok := c.ProviderSpecificData["baseUrl"]; !ok || strings.TrimSpace(fmt.Sprint(c.ProviderSpecificData["baseUrl"])) == "" {
		c.ProviderSpecificData["baseUrl"] = entry.BaseURL
	}
	if _, ok := c.ProviderSpecificData["apiType"]; !ok || strings.TrimSpace(fmt.Sprint(c.ProviderSpecificData["apiType"])) == "" {
		c.ProviderSpecificData["apiType"] = entry.APIType
	}
	if strings.TrimSpace(c.DefaultModel) == "" && len(entry.FallbackModels) > 0 {
		c.DefaultModel = entry.FallbackModels[0]
	}
	return c
}

func mustJSON(v interface{}) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func (s *Store) persistLocked() error {
	if s.rawRoot == nil {
		s.rawRoot = map[string]json.RawMessage{}
	}
	var err error
	if s.rawRoot["providerConnections"], err = mustJSON(s.db.ProviderConnections); err != nil {
		return err
	}
	if s.rawRoot["apiKeys"], err = mustJSON(s.db.APIKeys); err != nil {
		return err
	}
	if s.rawRoot["settings"], err = mustJSON(s.db.Settings); err != nil {
		return err
	}
	if s.rawRoot["modelAliases"], err = mustJSON(s.db.ModelAliases); err != nil {
		return err
	}
	if s.rawRoot["pricing"], err = mustJSON(s.db.Pricing); err != nil {
		return err
	}
	if s.rawRoot["usageData"], err = mustJSON(s.db.UsageData); err != nil {
		return err
	}
	if s.rawRoot["requestLogs"], err = mustJSON(s.db.RequestLogs); err != nil {
		return err
	}
	if s.rawRoot["authFiles"], err = mustJSON(s.db.AuthFiles); err != nil {
		return err
	}
	if s.rawRoot["proxyPools"], err = mustJSON(s.db.ProxyPools); err != nil {
		return err
	}
	if s.rawRoot["providerNodes"], err = mustJSON(s.db.ProviderNodes); err != nil {
		return err
	}
	if s.rawRoot["comboModels"], err = mustJSON(s.db.ComboModels); err != nil {
		return err
	}
	if s.rawRoot["routePolicies"], err = mustJSON(s.db.RoutePolicies); err != nil {
		return err
	}
	if s.rawRoot["mcpServers"], err = mustJSON(s.db.MCPServers); err != nil {
		return err
	}
	if s.rawRoot["a2aAgents"], err = mustJSON(s.db.A2AAgents); err != nil {
		return err
	}
	if s.rawRoot["tunnelEndpoints"], err = mustJSON(s.db.TunnelEndpoints); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(s.rawRoot, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.dbPath + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.dbPath)
}

func (s *Store) Reload() error         { s.mu.Lock(); defer s.mu.Unlock(); return s.load() }
func (s *Store) GetSettings() Settings { s.mu.RLock(); defer s.mu.RUnlock(); return s.db.Settings }

func (s *Store) UpdateSettings(patch map[string]interface{}) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(s.db.Settings)
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	nextRaw, _ := json.Marshal(merged)
	var next Settings
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return Settings{}, err
	}
	s.db.Settings = next
	return next, s.persistLocked()
}

func sortConnections(conns []ProviderConnection) {
	sort.SliceStable(conns, func(i, j int) bool { return conns[i].Priority < conns[j].Priority })
}

func (s *Store) GetActiveConnections(provider string) []ProviderConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ProviderConnection
	for _, c := range s.db.ProviderConnections {
		if !c.IsActive {
			continue
		}
		if provider != "" && c.Provider != provider {
			continue
		}
		if c.RateLimitedUntil != "" {
			until, err := time.Parse(time.RFC3339, c.RateLimitedUntil)
			if err == nil && until.After(time.Now()) {
				continue
			}
		}
		if c.CircuitOpenUntil != "" {
			until, err := time.Parse(time.RFC3339, c.CircuitOpenUntil)
			if err == nil && until.After(time.Now()) {
				continue
			}
		}
		out = append(out, c)
	}
	sortConnections(out)
	return out
}

func (s *Store) MarkConnectionCooldown(id string, until time.Time, status int, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	until = clampCooldownUntil(time.Now().UTC(), until, s.db.Settings.MaxCooldownSeconds)
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		s.db.ProviderConnections[i].RateLimitedUntil = until.UTC().Format(time.RFC3339)
		s.db.ProviderConnections[i].BackoffLevel++
		s.db.ProviderConnections[i].ConsecutiveFailures++
		s.db.ProviderConnections[i].LastError = message
		s.db.ProviderConnections[i].ErrorCode = status
		s.db.ProviderConnections[i].TestStatus = "unavailable"
		if shouldOpenCircuit(status, s.db.ProviderConnections[i].ConsecutiveFailures) {
			circuitUntil := until.UTC().Add(getCircuitBreakDuration(s.db.ProviderConnections[i].ConsecutiveFailures))
			circuitUntil = clampCooldownUntil(time.Now().UTC(), circuitUntil, s.db.Settings.MaxCooldownSeconds)
			s.db.ProviderConnections[i].CircuitOpenUntil = circuitUntil.Format(time.RFC3339)
			s.db.ProviderConnections[i].TestStatus = "circuit-open"
		}
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return s.persistLocked()
	}
	return fmt.Errorf("provider connection not found")
}

func clampCooldownUntil(now, until time.Time, maxSeconds int) time.Time {
	if maxSeconds <= 0 {
		return until
	}
	maxUntil := now.Add(time.Duration(maxSeconds) * time.Second)
	if until.After(maxUntil) {
		return maxUntil
	}
	return until
}

func (s *Store) ClearConnectionCooldown(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		s.db.ProviderConnections[i].RateLimitedUntil = ""
		s.db.ProviderConnections[i].BackoffLevel = 0
		s.db.ProviderConnections[i].ConsecutiveFailures = 0
		s.db.ProviderConnections[i].CircuitOpenUntil = ""
		s.db.ProviderConnections[i].LastError = ""
		s.db.ProviderConnections[i].ErrorCode = 0
		s.db.ProviderConnections[i].TestStatus = "active"
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return s.persistLocked()
	}
	return fmt.Errorf("provider connection not found")
}

func (s *Store) GetAllConnections() []ProviderConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProviderConnection, len(s.db.ProviderConnections))
	for i, c := range s.db.ProviderConnections {
		c.APIKey = ""
		c.AccessToken = ""
		c.RefreshToken = ""
		out[i] = c
	}
	sortConnections(out)
	return out
}

func (s *Store) GetAllConnectionsRaw() []ProviderConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProviderConnection, len(s.db.ProviderConnections))
	copy(out, s.db.ProviderConnections)
	sortConnections(out)
	return out
}

func (s *Store) GetConnectionByIDRaw(id string) (ProviderConnection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.db.ProviderConnections {
		if c.ID == id {
			return c, true
		}
	}
	return ProviderConnection{}, false
}

func (s *Store) UpdateConnectionTestStatus(id, status, message string, code int) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		s.db.ProviderConnections[i].TestStatus = strings.TrimSpace(status)
		s.db.ProviderConnections[i].LastError = strings.TrimSpace(message)
		s.db.ProviderConnections[i].ErrorCode = code
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.persistLocked(); err != nil {
			return ProviderConnection{}, err
		}
		return s.db.ProviderConnections[i], nil
	}
	return ProviderConnection{}, fmt.Errorf("provider connection not found")
}

func (s *Store) ClearAllCooldowns() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := 0
	for i := range s.db.ProviderConnections {
		conn := &s.db.ProviderConnections[i]
		if conn.RateLimitedUntil == "" && conn.BackoffLevel == 0 && conn.ConsecutiveFailures == 0 && conn.CircuitOpenUntil == "" && conn.LastError == "" && conn.ErrorCode == 0 && conn.TestStatus == "" {
			continue
		}
		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.ConsecutiveFailures = 0
		conn.CircuitOpenUntil = ""
		conn.LastError = ""
		conn.ErrorCode = 0
		conn.TestStatus = "active"
		conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		cleared++
	}
	if cleared == 0 {
		return 0, nil
	}
	return cleared, s.persistLocked()
}

func shouldOpenCircuit(status, failures int) bool {
	if failures < 3 {
		return false
	}
	return status == 0 || status >= 500
}

func getCircuitBreakDuration(failures int) time.Duration {
	switch {
	case failures >= 6:
		return 90 * time.Second
	case failures >= 4:
		return 45 * time.Second
	default:
		return 20 * time.Second
	}
}

func (s *Store) ClearHealthState(connectionID, provider string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := 0
	for i := range s.db.ProviderConnections {
		conn := &s.db.ProviderConnections[i]
		if connectionID != "" && conn.ID != connectionID {
			continue
		}
		if provider != "" && conn.Provider != provider {
			continue
		}
		if conn.RateLimitedUntil == "" && conn.BackoffLevel == 0 && conn.ConsecutiveFailures == 0 && conn.CircuitOpenUntil == "" && conn.LastError == "" && conn.ErrorCode == 0 && conn.TestStatus == "" {
			continue
		}
		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.ConsecutiveFailures = 0
		conn.CircuitOpenUntil = ""
		conn.LastError = ""
		conn.ErrorCode = 0
		conn.TestStatus = "active"
		conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		cleared++
	}
	if cleared == 0 {
		return 0, nil
	}
	return cleared, s.persistLocked()
}

func randID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return prefix + hex.EncodeToString(buf)
}

func (s *Store) CreateProviderConnection(c ProviderConnection) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	c = applyProviderDefaults(c)
	if c.ID == "" {
		c.ID = randID("pc_")
	}
	if c.Priority <= 0 {
		c.Priority = 1
	}
	c.CreatedAt = now
	c.UpdatedAt = now
	s.db.ProviderConnections = append(s.db.ProviderConnections, c)
	return c, s.persistLocked()
}

func (s *Store) UpdateProviderConnection(id string, patch map[string]interface{}) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, c := range s.db.ProviderConnections {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ProviderConnection{}, fmt.Errorf("provider connection not found")
	}
	raw, _ := json.Marshal(s.db.ProviderConnections[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next ProviderConnection
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return ProviderConnection{}, err
	}
	next = applyProviderDefaults(next)
	s.db.ProviderConnections[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteProviderConnection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.ProviderConnections[:0]
	found := false
	for _, c := range s.db.ProviderConnections {
		if c.ID == id {
			found = true
			continue
		}
		next = append(next, c)
	}
	if !found {
		return fmt.Errorf("provider connection not found")
	}
	s.db.ProviderConnections = next
	return s.persistLocked()
}

func (s *Store) UpdateOAuthTokens(id, accessToken, refreshToken, tokenExpiry string) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		if strings.TrimSpace(accessToken) != "" {
			s.db.ProviderConnections[i].AccessToken = strings.TrimSpace(accessToken)
		}
		if strings.TrimSpace(refreshToken) != "" {
			s.db.ProviderConnections[i].RefreshToken = strings.TrimSpace(refreshToken)
		}
		if tokenExpiry != "" {
			s.db.ProviderConnections[i].TokenExpiry = tokenExpiry
		}
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.persistLocked(); err != nil {
			return ProviderConnection{}, err
		}
		return s.db.ProviderConnections[i], nil
	}
	return ProviderConnection{}, fmt.Errorf("provider connection not found")
}

func (s *Store) ValidateAPIKey(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.db.APIKeys {
		if k.Key == key {
			return true
		}
	}
	return false
}

func (s *Store) GetAPIKeyByValue(key string) (APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.db.APIKeys {
		if item.Key == key {
			return item, true
		}
	}
	return APIKey{}, false
}

func maskAPIKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= 8 {
		return "********"
	}
	return trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
}

func (s *Store) GetAPIKeys() []APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]APIKey, len(s.db.APIKeys))
	for i, key := range s.db.APIKeys {
		key.Key = maskAPIKey(key.Key)
		out[i] = key
	}
	return out
}

func (s *Store) GetAPIKeysRaw() []APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]APIKey, len(s.db.APIKeys))
	copy(out, s.db.APIKeys)
	return out
}

func (s *Store) CreateAPIKey(name, key string, requestsPerMinute int) (APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	key = strings.TrimSpace(key)
	if key == "" {
		return APIKey{}, fmt.Errorf("api key is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	item := APIKey{
		ID:                randID("key_"),
		Key:               key,
		Name:              name,
		RequestsPerMinute: requestsPerMinute,
		CreatedAt:         now,
	}
	s.db.APIKeys = append(s.db.APIKeys, item)
	return item, s.persistLocked()
}

func (s *Store) DeleteAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.APIKeys[:0]
	found := false
	for _, item := range s.db.APIKeys {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("api key not found")
	}
	s.db.APIKeys = next
	return s.persistLocked()
}

func (s *Store) RotateAPIKey(id string, newKey string) (APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	newKey = strings.TrimSpace(newKey)
	if newKey == "" {
		return APIKey{}, fmt.Errorf("new api key is required")
	}
	for i := range s.db.APIKeys {
		if s.db.APIKeys[i].ID != id {
			continue
		}
		s.db.APIKeys[i].Key = newKey
		if err := s.persistLocked(); err != nil {
			return APIKey{}, err
		}
		return s.db.APIKeys[i], nil
	}
	return APIKey{}, fmt.Errorf("api key not found")
}

func (s *Store) ListAuthFiles(provider, account string) []AuthFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuthFile, 0, len(s.db.AuthFiles))
	provider = strings.ToLower(strings.TrimSpace(provider))
	account = strings.ToLower(strings.TrimSpace(account))
	for _, item := range s.db.AuthFiles {
		if provider != "" && strings.ToLower(strings.TrimSpace(item.Provider)) != provider {
			continue
		}
		if account != "" {
			name := strings.ToLower(strings.TrimSpace(item.AccountName))
			email := strings.ToLower(strings.TrimSpace(item.AccountEmail))
			if name != account && email != account {
				continue
			}
		}
		next := item
		next.ContentB64 = ""
		out = append(out, next)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

func (s *Store) CreateAuthFile(item AuthFile) (AuthFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = randID("af_")
	item.Name = strings.TrimSpace(item.Name)
	item.Provider = strings.TrimSpace(item.Provider)
	item.AccountName = strings.TrimSpace(item.AccountName)
	item.AccountEmail = strings.TrimSpace(item.AccountEmail)
	item.CreatedAt = now
	item.UpdatedAt = now
	item.Size = len(item.ContentB64)
	s.db.AuthFiles = append(s.db.AuthFiles, item)
	return item, s.persistLocked()
}

func (s *Store) GetAuthFile(id string) (AuthFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.db.AuthFiles {
		if item.ID == id {
			return item, true
		}
	}
	return AuthFile{}, false
}

func (s *Store) DeleteAuthFile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.AuthFiles[:0]
	found := false
	for _, item := range s.db.AuthFiles {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("auth file not found")
	}
	s.db.AuthFiles = next
	return s.persistLocked()
}

func sanitizeConnectionIDs(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func (s *Store) ListProxyPools() []ProxyPool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProxyPool, len(s.db.ProxyPools))
	copy(out, s.db.ProxyPools)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) GetProxyPool(id string) (ProxyPool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.db.ProxyPools {
		if item.ID == id {
			return item, true
		}
	}
	return ProxyPool{}, false
}

func (s *Store) CreateProxyPool(item ProxyPool) (ProxyPool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = randID("pool_")
	item.Name = strings.TrimSpace(item.Name)
	item.ConnectionIDs = sanitizeConnectionIDs(item.ConnectionIDs)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.db.ProxyPools = append(s.db.ProxyPools, item)
	return item, s.persistLocked()
}

func (s *Store) UpdateProxyPool(id string, patch map[string]interface{}) (ProxyPool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.db.ProxyPools {
		if item.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ProxyPool{}, fmt.Errorf("proxy pool not found")
	}
	raw, _ := json.Marshal(s.db.ProxyPools[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next ProxyPool
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return ProxyPool{}, err
	}
	next.Name = strings.TrimSpace(next.Name)
	next.ConnectionIDs = sanitizeConnectionIDs(next.ConnectionIDs)
	s.db.ProxyPools[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteProxyPool(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.ProxyPools[:0]
	found := false
	for _, item := range s.db.ProxyPools {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("proxy pool not found")
	}
	s.db.ProxyPools = next
	return s.persistLocked()
}

func (s *Store) ListProviderNodes() []ProviderNode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProviderNode, len(s.db.ProviderNodes))
	copy(out, s.db.ProviderNodes)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) GetProviderNode(id string) (ProviderNode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.db.ProviderNodes {
		if item.ID == id {
			return item, true
		}
	}
	return ProviderNode{}, false
}

func (s *Store) CreateProviderNode(item ProviderNode) (ProviderNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = randID("node_")
	item.Name = strings.TrimSpace(item.Name)
	item.Provider = strings.TrimSpace(item.Provider)
	item.ConnectionIDs = sanitizeConnectionIDs(item.ConnectionIDs)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.db.ProviderNodes = append(s.db.ProviderNodes, item)
	return item, s.persistLocked()
}

func (s *Store) UpdateProviderNode(id string, patch map[string]interface{}) (ProviderNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.db.ProviderNodes {
		if item.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ProviderNode{}, fmt.Errorf("provider node not found")
	}
	raw, _ := json.Marshal(s.db.ProviderNodes[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next ProviderNode
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return ProviderNode{}, err
	}
	next.Name = strings.TrimSpace(next.Name)
	next.Provider = strings.TrimSpace(next.Provider)
	next.ConnectionIDs = sanitizeConnectionIDs(next.ConnectionIDs)
	s.db.ProviderNodes[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteProviderNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.ProviderNodes[:0]
	found := false
	for _, item := range s.db.ProviderNodes {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("provider node not found")
	}
	s.db.ProviderNodes = next
	return s.persistLocked()
}

func sanitizeComboTargets(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range sanitizeConnectionIDs(items) {
		if strings.Contains(item, "/") {
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) ListComboModels() []ComboModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ComboModel, len(s.db.ComboModels))
	copy(out, s.db.ComboModels)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) GetComboModelsMap() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]string, len(s.db.ComboModels))
	for _, item := range s.db.ComboModels {
		out[item.Alias] = append([]string(nil), item.Targets...)
	}
	return out
}

func (s *Store) CreateComboModel(item ComboModel) (ComboModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.Alias = strings.TrimSpace(item.Alias)
	item.Targets = sanitizeComboTargets(item.Targets)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.db.ComboModels = append(s.db.ComboModels, item)
	return item, s.persistLocked()
}

func (s *Store) UpdateComboModel(alias string, patch map[string]interface{}) (ComboModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.db.ComboModels {
		if item.Alias == alias {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ComboModel{}, fmt.Errorf("combo model not found")
	}
	raw, _ := json.Marshal(s.db.ComboModels[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next ComboModel
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return ComboModel{}, err
	}
	next.Alias = strings.TrimSpace(next.Alias)
	next.Targets = sanitizeComboTargets(next.Targets)
	s.db.ComboModels[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteComboModel(alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.ComboModels[:0]
	found := false
	for _, item := range s.db.ComboModels {
		if item.Alias == alias {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("combo model not found")
	}
	s.db.ComboModels = next
	return s.persistLocked()
}

func sanitizePolicyProviders(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range sanitizeConnectionIDs(items) {
		out = append(out, strings.ToLower(strings.TrimSpace(item)))
	}
	return out
}

func sanitizePolicyAccounts(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func (s *Store) ListRoutePolicies() []RoutePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RoutePolicy, len(s.db.RoutePolicies))
	copy(out, s.db.RoutePolicies)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) CreateRoutePolicy(item RoutePolicy) (RoutePolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = randID("rp_")
	item.Name = strings.TrimSpace(item.Name)
	item.ModelPrefix = strings.TrimSpace(item.ModelPrefix)
	item.Providers = sanitizePolicyProviders(item.Providers)
	item.Accounts = sanitizePolicyAccounts(item.Accounts)
	item.TargetPoolID = strings.TrimSpace(item.TargetPoolID)
	item.TargetNodeID = strings.TrimSpace(item.TargetNodeID)
	item.ForceModel = strings.TrimSpace(item.ForceModel)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.db.RoutePolicies = append(s.db.RoutePolicies, item)
	return item, s.persistLocked()
}

func (s *Store) UpdateRoutePolicy(id string, patch map[string]interface{}) (RoutePolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.db.RoutePolicies {
		if item.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return RoutePolicy{}, fmt.Errorf("route policy not found")
	}
	raw, _ := json.Marshal(s.db.RoutePolicies[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next RoutePolicy
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return RoutePolicy{}, err
	}
	next.Name = strings.TrimSpace(next.Name)
	next.ModelPrefix = strings.TrimSpace(next.ModelPrefix)
	next.Providers = sanitizePolicyProviders(next.Providers)
	next.Accounts = sanitizePolicyAccounts(next.Accounts)
	next.TargetPoolID = strings.TrimSpace(next.TargetPoolID)
	next.TargetNodeID = strings.TrimSpace(next.TargetNodeID)
	next.ForceModel = strings.TrimSpace(next.ForceModel)
	s.db.RoutePolicies[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteRoutePolicy(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.RoutePolicies[:0]
	found := false
	for _, item := range s.db.RoutePolicies {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("route policy not found")
	}
	s.db.RoutePolicies = next
	return s.persistLocked()
}

func (s *Store) ListMCPServers(includeDisabled bool) []MCPServer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MCPServer, 0, len(s.db.MCPServers))
	for _, item := range s.db.MCPServers {
		if !includeDisabled && !item.Enabled {
			continue
		}
		clean := item
		clean.Env = nil
		clean.Headers = nil
		out = append(out, clean)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) CreateMCPServer(item MCPServer) (MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = randID("mcp_")
	item.Name = strings.TrimSpace(item.Name)
	item.Transport = strings.ToLower(strings.TrimSpace(item.Transport))
	item.Command = strings.TrimSpace(item.Command)
	item.URL = strings.TrimSpace(item.URL)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.db.MCPServers = append(s.db.MCPServers, item)
	return item, s.persistLocked()
}

func (s *Store) UpdateMCPServer(id string, patch map[string]interface{}) (MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.db.MCPServers {
		if item.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return MCPServer{}, fmt.Errorf("mcp server not found")
	}
	raw, _ := json.Marshal(s.db.MCPServers[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next MCPServer
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return MCPServer{}, err
	}
	next.Name = strings.TrimSpace(next.Name)
	next.Transport = strings.ToLower(strings.TrimSpace(next.Transport))
	next.Command = strings.TrimSpace(next.Command)
	next.URL = strings.TrimSpace(next.URL)
	s.db.MCPServers[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteMCPServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.MCPServers[:0]
	found := false
	for _, item := range s.db.MCPServers {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("mcp server not found")
	}
	s.db.MCPServers = next
	return s.persistLocked()
}

func sanitizeCapabilities(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		k := strings.ToLower(v)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Store) ListA2AAgents(includeDisabled bool) []A2AAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]A2AAgent, 0, len(s.db.A2AAgents))
	for _, item := range s.db.A2AAgents {
		if !includeDisabled && !item.Enabled {
			continue
		}
		clean := item
		clean.Env = nil
		clean.Headers = nil
		out = append(out, clean)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) CreateA2AAgent(item A2AAgent) (A2AAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = randID("a2a_")
	item.Name = strings.TrimSpace(item.Name)
	item.URL = strings.TrimSpace(item.URL)
	item.Protocol = strings.ToLower(strings.TrimSpace(item.Protocol))
	item.Capabilities = sanitizeCapabilities(item.Capabilities)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.db.A2AAgents = append(s.db.A2AAgents, item)
	return item, s.persistLocked()
}

func (s *Store) UpdateA2AAgent(id string, patch map[string]interface{}) (A2AAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.db.A2AAgents {
		if item.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return A2AAgent{}, fmt.Errorf("a2a agent not found")
	}
	raw, _ := json.Marshal(s.db.A2AAgents[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next A2AAgent
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return A2AAgent{}, err
	}
	next.Name = strings.TrimSpace(next.Name)
	next.URL = strings.TrimSpace(next.URL)
	next.Protocol = strings.ToLower(strings.TrimSpace(next.Protocol))
	next.Capabilities = sanitizeCapabilities(next.Capabilities)
	s.db.A2AAgents[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteA2AAgent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.A2AAgents[:0]
	found := false
	for _, item := range s.db.A2AAgents {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("a2a agent not found")
	}
	s.db.A2AAgents = next
	return s.persistLocked()
}

func (s *Store) ListTunnelEndpoints(includeDisabled bool) []TunnelEndpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TunnelEndpoint, 0, len(s.db.TunnelEndpoints))
	for _, item := range s.db.TunnelEndpoints {
		if !includeDisabled && !item.Enabled {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) CreateTunnelEndpoint(item TunnelEndpoint) (TunnelEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = randID("tun_")
	item.Name = strings.TrimSpace(item.Name)
	item.Provider = strings.ToLower(strings.TrimSpace(item.Provider))
	item.PublicURL = strings.TrimSpace(item.PublicURL)
	item.LocalTarget = strings.TrimSpace(item.LocalTarget)
	item.Protocol = strings.ToLower(strings.TrimSpace(item.Protocol))
	item.Region = strings.TrimSpace(item.Region)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.db.TunnelEndpoints = append(s.db.TunnelEndpoints, item)
	return item, s.persistLocked()
}

func (s *Store) UpdateTunnelEndpoint(id string, patch map[string]interface{}) (TunnelEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.db.TunnelEndpoints {
		if item.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return TunnelEndpoint{}, fmt.Errorf("tunnel endpoint not found")
	}
	raw, _ := json.Marshal(s.db.TunnelEndpoints[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next TunnelEndpoint
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return TunnelEndpoint{}, err
	}
	next.Name = strings.TrimSpace(next.Name)
	next.Provider = strings.ToLower(strings.TrimSpace(next.Provider))
	next.PublicURL = strings.TrimSpace(next.PublicURL)
	next.LocalTarget = strings.TrimSpace(next.LocalTarget)
	next.Protocol = strings.ToLower(strings.TrimSpace(next.Protocol))
	next.Region = strings.TrimSpace(next.Region)
	s.db.TunnelEndpoints[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteTunnelEndpoint(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.TunnelEndpoints[:0]
	found := false
	for _, item := range s.db.TunnelEndpoints {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("tunnel endpoint not found")
	}
	s.db.TunnelEndpoints = next
	return s.persistLocked()
}

func (s *Store) GetModelAliases() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.db.ModelAliases))
	for k, v := range s.db.ModelAliases {
		out[k] = v
	}
	return out
}

func sanitizeAliases(input map[string]string) map[string]string {
	out := map[string]string{}
	for fullModel, alias := range input {
		k := strings.TrimSpace(fullModel)
		v := strings.TrimSpace(alias)
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func (s *Store) ReplaceModelAliases(aliases map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.ModelAliases = sanitizeAliases(aliases)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.ModelAliases))
	for k, v := range s.db.ModelAliases {
		out[k] = v
	}
	return out, nil
}

func (s *Store) PatchModelAliases(aliases map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.ModelAliases == nil {
		s.db.ModelAliases = map[string]string{}
	}
	for k, v := range sanitizeAliases(aliases) {
		s.db.ModelAliases[k] = v
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.ModelAliases))
	for k, v := range s.db.ModelAliases {
		out[k] = v
	}
	return out, nil
}

func (s *Store) DeleteModelAliasKeys(fullModels []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.ModelAliases == nil {
		s.db.ModelAliases = map[string]string{}
	}
	for _, key := range fullModels {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		delete(s.db.ModelAliases, trimmed)
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.ModelAliases))
	for k, v := range s.db.ModelAliases {
		out[k] = v
	}
	return out, nil
}

func sanitizeMappings(input map[string]string) map[string]string {
	out := map[string]string{}
	for source, target := range input {
		k := strings.TrimSpace(source)
		v := strings.TrimSpace(target)
		if k == "" || v == "" || !strings.Contains(v, "/") {
			continue
		}
		out[k] = v
	}
	return out
}

func (s *Store) GetForcedModelMappings() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return sanitizeMappings(out)
}

func (s *Store) ReplaceForcedModelMappings(mappings map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Settings.ForcedModelMappings = sanitizeMappings(mappings)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return out, nil
}

func (s *Store) PatchForcedModelMappings(mappings map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.Settings.ForcedModelMappings == nil {
		s.db.Settings.ForcedModelMappings = map[string]string{}
	}
	for k, v := range sanitizeMappings(mappings) {
		s.db.Settings.ForcedModelMappings[k] = v
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return out, nil
}

func (s *Store) DeleteForcedModelMappingKeys(keys []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.Settings.ForcedModelMappings == nil {
		s.db.Settings.ForcedModelMappings = map[string]string{}
	}
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		delete(s.db.Settings.ForcedModelMappings, trimmed)
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return out, nil
}

func sanitizeModelList(input []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(input))
	for _, item := range input {
		model := strings.TrimSpace(item)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}

func (s *Store) GetDisabledModels() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sanitizeModelList(s.db.Settings.DisabledModels)
}

func (s *Store) ReplaceDisabledModels(models []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Settings.DisabledModels = sanitizeModelList(models)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return append([]string(nil), s.db.Settings.DisabledModels...), nil
}

func (s *Store) PatchDisabledModels(models []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	merged := append([]string(nil), s.db.Settings.DisabledModels...)
	merged = append(merged, models...)
	s.db.Settings.DisabledModels = sanitizeModelList(merged)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return append([]string(nil), s.db.Settings.DisabledModels...), nil
}

func (s *Store) DeleteDisabledModels(models []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	remove := map[string]bool{}
	for _, item := range models {
		item = strings.TrimSpace(item)
		if item != "" {
			remove[item] = true
		}
	}
	next := make([]string, 0, len(s.db.Settings.DisabledModels))
	for _, item := range s.db.Settings.DisabledModels {
		if !remove[item] {
			next = append(next, item)
		}
	}
	s.db.Settings.DisabledModels = sanitizeModelList(next)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return append([]string(nil), s.db.Settings.DisabledModels...), nil
}

func (s *Store) IsModelDisabled(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.db.Settings.DisabledModels {
		if item == model {
			return true
		}
	}
	return false
}

func sanitizeAvailability(input map[string]string) map[string]string {
	out := map[string]string{}
	for model, status := range input {
		k := strings.TrimSpace(model)
		v := strings.ToLower(strings.TrimSpace(status))
		if k == "" {
			continue
		}
		switch v {
		case "available", "unavailable", "unknown":
			out[k] = v
		}
	}
	return out
}

func (s *Store) GetModelAvailability() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.db.Settings.ModelAvailability))
	for k, v := range s.db.Settings.ModelAvailability {
		out[k] = v
	}
	return sanitizeAvailability(out)
}

func (s *Store) ReplaceModelAvailability(input map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Settings.ModelAvailability = sanitizeAvailability(input)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ModelAvailability))
	for k, v := range s.db.Settings.ModelAvailability {
		out[k] = v
	}
	return out, nil
}

func (s *Store) PatchModelAvailability(input map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.Settings.ModelAvailability == nil {
		s.db.Settings.ModelAvailability = map[string]string{}
	}
	for k, v := range sanitizeAvailability(input) {
		s.db.Settings.ModelAvailability[k] = v
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ModelAvailability))
	for k, v := range s.db.Settings.ModelAvailability {
		out[k] = v
	}
	return out, nil
}

func (s *Store) DeleteModelAvailabilityKeys(models []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.Settings.ModelAvailability == nil {
		s.db.Settings.ModelAvailability = map[string]string{}
	}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			delete(s.db.Settings.ModelAvailability, model)
		}
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ModelAvailability))
	for k, v := range s.db.Settings.ModelAvailability {
		out[k] = v
	}
	return out, nil
}

func (s *Store) GetUsageSummary() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	totalCost := 0.0
	for _, item := range s.db.UsageData.History {
		totalCost += item.TotalCost
	}
	providers := map[string]DailySummary{}
	for _, item := range s.db.UsageData.History {
		cur := providers[item.Provider]
		cur.Requests++
		cur.Cost += item.TotalCost
		providers[item.Provider] = cur
	}
	return map[string]interface{}{"totalRequests": s.db.UsageData.TotalRequestsLifetime, "totalCost": totalCost, "providers": providers, "days": s.db.UsageData.DailySummary, "historySize": len(s.db.UsageData.History)}
}

func (s *Store) RecordUsage(entry UsageEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if entry.Timestamp == "" {
		entry.Timestamp = now.Format(time.RFC3339)
	}
	if entry.TotalCost == 0 {
		entry.TotalCost = estimateCost(s.db.Pricing, entry.Model, entry.PromptTokens, entry.CompletionTokens)
	}
	s.db.UsageData.TotalRequestsLifetime++
	s.db.UsageData.History = append(s.db.UsageData.History, entry)
	limit := s.db.Settings.ObservabilityMaxRecords
	if limit <= 0 {
		limit = 1000
	}
	if len(s.db.UsageData.History) > limit {
		s.db.UsageData.History = s.db.UsageData.History[len(s.db.UsageData.History)-limit:]
	}
	day := now.Format("2006-01-02")
	daily := s.db.UsageData.DailySummary[day]
	daily.Requests++
	daily.Cost += entry.TotalCost
	s.db.UsageData.DailySummary[day] = daily
	return s.persistLocked()
}

func (s *Store) RecordRequestLog(entry RequestLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = randID("rlog_")
	}
	if strings.TrimSpace(entry.Timestamp) == "" {
		entry.Timestamp = now.Format(time.RFC3339)
	}
	s.db.RequestLogs = append(s.db.RequestLogs, entry)
	limit := s.db.Settings.ObservabilityMaxRecords
	if limit <= 0 {
		limit = 1000
	}
	if len(s.db.RequestLogs) > limit {
		s.db.RequestLogs = s.db.RequestLogs[len(s.db.RequestLogs)-limit:]
	}
	return s.persistLocked()
}

func (s *Store) GetRequestLogs(limit int) []RequestLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.db.RequestLogs) {
		limit = len(s.db.RequestLogs)
	}
	out := make([]RequestLog, 0, limit)
	for i := len(s.db.RequestLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.db.RequestLogs[i])
	}
	return out
}

func (s *Store) GetRequestLogByID(id string) (RequestLog, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = strings.TrimSpace(id)
	for _, item := range s.db.RequestLogs {
		if item.ID == id {
			return item, true
		}
	}
	return RequestLog{}, false
}

func (s *Store) GetUsageHistory(limit int, provider string) []UsageEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UsageEntry, 0, len(s.db.UsageData.History))
	provider = strings.TrimSpace(provider)
	for i := len(s.db.UsageData.History) - 1; i >= 0; i-- {
		item := s.db.UsageData.History[i]
		if provider != "" && item.Provider != provider {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Store) GetUsageStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byProvider := map[string]DailySummary{}
	byModel := map[string]DailySummary{}
	byDay := map[string]DailySummary{}
	var totalRequests int64
	totalCost := 0.0
	var promptTokens int64
	var completionTokens int64
	for _, item := range s.db.UsageData.History {
		totalRequests++
		totalCost += item.TotalCost
		promptTokens += item.PromptTokens
		completionTokens += item.CompletionTokens
		addUsageStat(byProvider, item.Provider, item.TotalCost)
		addUsageStat(byModel, item.Model, item.TotalCost)
		day := usageDay(item.Timestamp)
		if day != "" {
			addUsageStat(byDay, day, item.TotalCost)
		}
	}
	return map[string]interface{}{
		"totalRequests":    totalRequests,
		"totalCost":        totalCost,
		"promptTokens":     promptTokens,
		"completionTokens": completionTokens,
		"totalTokens":      promptTokens + completionTokens,
		"byProvider":       byProvider,
		"byModel":          byModel,
		"byDay":            byDay,
	}
}

func addUsageStat(bucket map[string]DailySummary, key string, cost float64) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	cur := bucket[key]
	cur.Requests++
	cur.Cost += cost
	bucket[key] = cur
}

func usageDay(timestamp string) string {
	timestamp = strings.TrimSpace(timestamp)
	if timestamp == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, timestamp); err == nil {
		return parsed.UTC().Format("2006-01-02")
	}
	if len(timestamp) >= len("2006-01-02") {
		return timestamp[:len("2006-01-02")]
	}
	return timestamp
}

func pricingNumber(node map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := node[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case float64:
			return v, true
		case int64:
			return float64(v), true
		case int:
			return float64(v), true
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func estimateCost(pricing map[string]interface{}, model string, promptTokens, completionTokens int64) float64 {
	if pricing == nil || strings.TrimSpace(model) == "" {
		return 0
	}
	candidates := []string{model}
	if idx := strings.Index(model, "/"); idx >= 0 && idx+1 < len(model) {
		candidates = append(candidates, model[idx+1:])
	}
	for _, key := range candidates {
		raw, ok := pricing[key]
		if !ok {
			continue
		}
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		promptRate, _ := pricingNumber(entry, "prompt", "promptCostPer1k", "promptCostPer1K", "input", "inputCostPer1k", "inputCostPer1K")
		completionRate, _ := pricingNumber(entry, "completion", "completionCostPer1k", "completionCostPer1K", "output", "outputCostPer1k", "outputCostPer1K")
		cost := float64(promptTokens)/1000.0*promptRate + float64(completionTokens)/1000.0*completionRate
		if cost < 0 {
			return 0
		}
		return cost
	}
	return 0
}

func (s *Store) DBSnapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	safe := s.db
	conns := make([]ProviderConnection, len(safe.ProviderConnections))
	for i, c := range safe.ProviderConnections {
		c.APIKey = ""
		c.AccessToken = ""
		c.RefreshToken = ""
		conns[i] = c
	}
	safe.ProviderConnections = conns
	safe.APIKeys = nil
	return json.Marshal(safe)
}
