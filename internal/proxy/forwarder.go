package proxy

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"xrouter/internal/store"
)

type Forwarder struct {
	client          *http.Client
	store           *store.Store
	mu              sync.Mutex
	proxyConfigHash string
	dedupMu         sync.Mutex
	dedup           map[string]*dedupEntry
	modelsMu        sync.Mutex
	modelsCache     map[string]modelsCacheEntry
	rrMu            sync.Mutex
	rrIndex         map[string]int
}

type dedupEntry struct {
	expiresAt time.Time
	waitCh    chan struct{}
	ready     bool
	status    int
	headers   http.Header
	body      []byte
	err       error
}

type modelsCacheEntry struct {
	expiresAt time.Time
	models    []string
	source    string
}

const dedupTTL = 5 * time.Second
const modelsCacheTTL = 60 * time.Second

func NewForwarder(st *store.Store) *Forwarder {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 256
	transport.MaxIdleConnsPerHost = 64
	transport.MaxConnsPerHost = 128
	transport.IdleConnTimeout = 90 * time.Second

	return &Forwarder{
		client:      &http.Client{Transport: transport, Timeout: 0},
		store:       st,
		dedup:       map[string]*dedupEntry{},
		modelsCache: map[string]modelsCacheEntry{},
		rrIndex:     map[string]int{},
	}
}

func (f *Forwarder) HTTPClient() *http.Client {
	return f.client
}

func isStreaming(body map[string]interface{}) bool {
	v, ok := body["stream"]
	if !ok {
		return false
	}
	stream, ok := v.(bool)
	return ok && stream
}

func dedupKey(scope, path string, body []byte) string {
	sum := sha1.Sum(append([]byte(scope+"|"+path+"|"), body...))
	return fmt.Sprintf("%x", sum[:])
}

func cloneHeader(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for k, values := range in {
		next := make([]string, len(values))
		copy(next, values)
		out[k] = next
	}
	return out
}

func cloneResponse(status int, headers http.Header, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     cloneHeader(headers),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func (f *Forwarder) refreshTransport() error {
	settings := f.store.GetSettings()
	hash := fmt.Sprintf("%t|%s|%s", settings.OutboundProxyEnabled, settings.OutboundProxyURL, settings.OutboundNoProxy)

	f.mu.Lock()
	defer f.mu.Unlock()
	if hash == f.proxyConfigHash {
		return nil
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 256
	transport.MaxIdleConnsPerHost = 64
	transport.MaxConnsPerHost = 128
	transport.IdleConnTimeout = 90 * time.Second
	if settings.OutboundProxyEnabled && strings.TrimSpace(settings.OutboundProxyURL) != "" {
		proxyURL, err := url.Parse(strings.TrimSpace(settings.OutboundProxyURL))
		if err != nil {
			return fmt.Errorf("invalid outbound proxy url: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	f.client.Transport = transport
	f.proxyConfigHash = hash
	return nil
}

func resolveEndpoint(c store.ProviderConnection, model, path string) (string, string, error) {
	baseURL := ""
	apiType := ""
	if c.ProviderSpecificData != nil {
		if v, ok := c.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			baseURL = strings.TrimRight(strings.TrimSpace(v), "/")
		}
		if v, ok := c.ProviderSpecificData["apiType"].(string); ok {
			apiType = strings.ToLower(strings.TrimSpace(v))
		}
	}
	if c.Provider == "azure" {
		return resolveAzureEndpoint(c, model, path)
	}
	if baseURL == "" {
		if entry, ok := store.GetProviderCatalogEntry(c.Provider); ok {
			baseURL = strings.TrimRight(entry.BaseURL, "/")
			if apiType == "" {
				apiType = entry.APIType
			}
		} else {
			return "", "", fmt.Errorf("provider %s missing baseUrl", c.Provider)
		}
	}
	if baseURL == "" {
		return "", "", fmt.Errorf("provider %s missing baseUrl", c.Provider)
	}
	baseURL = resolveBaseURL(c, baseURL)
	if strings.Contains(baseURL, "{") {
		return "", "", fmt.Errorf("provider %s missing baseUrl variables", c.Provider)
	}

	if path == "/v1/responses" || path == "/v1/responses/compact" || path == "/v1/responses/stream" || path == "/backend-api/codex/responses" || apiType == "responses" {
		return joinOpenAIEndpoint(baseURL, "/responses"), "openai", nil
	}
	if path == "/v1/completions" {
		return joinOpenAIEndpoint(baseURL, "/v1/completions"), "openai", nil
	}
	if path == "/v1/messages/count_tokens" {
		return joinOpenAIEndpoint(baseURL, "/v1/messages/count_tokens"), "anthropic", nil
	}
	if apiType == "gemini" {
		return buildGeminiEndpoint(baseURL, model, path)
	}
	if apiType == "anthropic" {
		return joinOpenAIEndpoint(baseURL, "/v1/messages"), "anthropic", nil
	}
	if strings.Contains(model, "claude") || c.Provider == "anthropic" || strings.HasPrefix(c.Provider, "anthropic-compatible-") {
		return joinOpenAIEndpoint(baseURL, "/v1/messages"), "anthropic", nil
	}
	return joinOpenAIEndpoint(baseURL, "/v1/chat/completions"), "openai", nil
}

func resolveAzureEndpoint(c store.ProviderConnection, model, path string) (string, string, error) {
	endpoint := providerDataString(c, "azureEndpoint")
	if endpoint == "" {
		endpoint = providerDataString(c, "baseUrl")
	}
	if endpoint == "" {
		return "", "", fmt.Errorf("provider azure missing azureEndpoint")
	}
	deployment := providerDataString(c, "deployment")
	if deployment == "" {
		deployment = providerDataString(c, "azureDeployment")
	}
	if deployment == "" {
		deployment = strings.TrimSpace(model)
		if strings.Contains(deployment, "/") {
			parts := strings.SplitN(deployment, "/", 2)
			deployment = strings.TrimSpace(parts[1])
		}
	}
	if deployment == "" {
		return "", "", fmt.Errorf("provider azure missing deployment")
	}
	apiVersion := providerDataString(c, "apiVersion")
	if apiVersion == "" {
		apiVersion = providerDataString(c, "azureApiVersion")
	}
	if apiVersion == "" {
		apiVersion = "2024-10-21"
	}
	operation := "/chat/completions"
	switch path {
	case "/v1/completions":
		operation = "/completions"
	case "/v1/embeddings":
		operation = "/embeddings"
	}
	return strings.TrimRight(endpoint, "/") + "/openai/deployments/" + url.PathEscape(deployment) + operation + "?api-version=" + url.QueryEscape(apiVersion), "openai", nil
}

func providerDataString(c store.ProviderConnection, key string) string {
	if c.ProviderSpecificData == nil {
		return ""
	}
	value, ok := c.ProviderSpecificData[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func resolveBaseURL(c store.ProviderConnection, baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if !strings.Contains(baseURL, "{") {
		return baseURL
	}
	if c.ProviderSpecificData == nil {
		return baseURL
	}
	for _, key := range []string{"accountId", "azureEndpoint", "deployment"} {
		value := providerDataString(c, key)
		if value == "" {
			continue
		}
		baseURL = strings.ReplaceAll(baseURL, "{"+key+"}", value)
	}
	return baseURL
}

func buildGeminiEndpoint(baseURL, model, path string) (string, string, error) {
	if path != "/v1/chat/completions" {
		return "", "", fmt.Errorf("gemini-compatible adapter only supports /v1/chat/completions")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gemini-1.5-flash"
	}
	if strings.Contains(model, "/") {
		parts := strings.SplitN(model, "/", 2)
		model = strings.TrimSpace(parts[1])
	}
	return strings.TrimRight(baseURL, "/") + "/v1beta/models/" + model + ":generateContent", "gemini", nil
}

func joinOpenAIEndpoint(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasPrefix(path, "/v1/") && (strings.HasSuffix(base, "/v1") || endsWithVersionSegment(base)) {
		return base + strings.TrimPrefix(path, "/v1")
	}
	return base + path
}

func endsWithVersionSegment(baseURL string) bool {
	parts := strings.Split(strings.TrimRight(baseURL, "/"), "/")
	if len(parts) == 0 {
		return false
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	if len(last) < 2 || last[0] != 'v' {
		return false
	}
	hasDigit := false
	for _, ch := range last[1:] {
		if ch >= '0' && ch <= '9' {
			hasDigit = true
			continue
		}
		if ch != '.' {
			return false
		}
	}
	return hasDigit
}

func setAuthHeader(req *http.Request, c store.ProviderConnection, mode string) {
	if c.AuthType == "cookie" {
		if c.APIKey != "" {
			req.Header.Set("Cookie", c.APIKey)
		}
		return
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if c.Provider == "azure" && c.APIKey != "" {
		req.Header.Set("api-key", c.APIKey)
		req.Header.Del("Authorization")
	}
	if c.AccessToken != "" && c.APIKey == "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	if mode == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	if mode == "gemini" {
		req.Header.Set("x-goog-api-key", c.APIKey)
		if c.APIKey != "" {
			req.Header.Del("Authorization")
		}
	}
	if mode == "deepgram" && c.APIKey != "" {
		req.Header.Set("Authorization", "Token "+c.APIKey)
	}
}

func extractModel(body map[string]interface{}) string {
	if v, ok := body["model"].(string); ok {
		return v
	}
	return ""
}

func extractPoolHint(body map[string]interface{}) string {
	if v, ok := body["pool"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func extractNodeHint(body map[string]interface{}) string {
	if v, ok := body["node"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func matchRoutePolicy(policy store.RoutePolicy, model string, providerHint string) bool {
	if strings.TrimSpace(policy.ModelPrefix) != "" && !strings.HasPrefix(model, strings.TrimSpace(policy.ModelPrefix)) {
		return false
	}
	if providerHint != "" && len(policy.Providers) > 0 {
		matched := false
		for _, provider := range policy.Providers {
			if provider == strings.ToLower(strings.TrimSpace(providerHint)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func filterCandidatesByPolicy(candidates []store.ProviderConnection, policy store.RoutePolicy) []store.ProviderConnection {
	if len(policy.Providers) == 0 && len(policy.Accounts) == 0 {
		return candidates
	}
	providers := map[string]bool{}
	for _, item := range policy.Providers {
		providers[strings.ToLower(strings.TrimSpace(item))] = true
	}
	accounts := map[string]bool{}
	for _, item := range policy.Accounts {
		accounts[strings.ToLower(strings.TrimSpace(item))] = true
	}
	out := make([]store.ProviderConnection, 0, len(candidates))
	for _, candidate := range candidates {
		if len(providers) > 0 && !providers[strings.ToLower(strings.TrimSpace(candidate.Provider))] {
			continue
		}
		if len(accounts) > 0 {
			accountName := strings.ToLower(strings.TrimSpace(candidate.AccountName))
			accountEmail := strings.ToLower(strings.TrimSpace(candidate.AccountEmail))
			if !accounts[accountName] && !accounts[accountEmail] {
				continue
			}
		}
		out = append(out, candidate)
	}
	return out
}

func normalizeModelForUpstream(body map[string]interface{}, providerHint string) []byte {
	if providerHint != "" {
		if model, ok := body["model"].(string); ok && strings.HasPrefix(model, providerHint+"/") {
			body["model"] = strings.TrimPrefix(model, providerHint+"/")
		}
	}
	raw, _ := json.Marshal(body)
	return raw
}

func normalizeBodyForMode(body map[string]interface{}, providerHint, mode string) []byte {
	if mode == "gemini" {
		return normalizeOpenAIToGeminiBody(body, providerHint)
	}
	return normalizeModelForUpstream(body, providerHint)
}

func normalizeOpenAIToGeminiBody(body map[string]interface{}, providerHint string) []byte {
	if providerHint != "" {
		if model, ok := body["model"].(string); ok && strings.HasPrefix(model, providerHint+"/") {
			body["model"] = strings.TrimPrefix(model, providerHint+"/")
		}
	}
	out := map[string]interface{}{}
	if messages, ok := body["messages"].([]interface{}); ok {
		contents := make([]map[string]interface{}, 0, len(messages))
		for _, raw := range messages {
			item, _ := raw.(map[string]interface{})
			role := "user"
			if v, ok := item["role"].(string); ok && strings.TrimSpace(v) != "" {
				switch strings.ToLower(strings.TrimSpace(v)) {
				case "assistant", "model":
					role = "model"
				default:
					role = "user"
				}
			}
			parts := make([]map[string]string, 0, 1)
			switch content := item["content"].(type) {
			case string:
				if strings.TrimSpace(content) != "" {
					parts = append(parts, map[string]string{"text": content})
				}
			case []interface{}:
				for _, rawPart := range content {
					part, _ := rawPart.(map[string]interface{})
					if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
						parts = append(parts, map[string]string{"text": text})
					}
				}
			}
			if len(parts) == 0 {
				continue
			}
			contents = append(contents, map[string]interface{}{"role": role, "parts": parts})
		}
		out["contents"] = contents
	}
	cfg := map[string]interface{}{}
	if v, ok := body["max_tokens"]; ok {
		cfg["maxOutputTokens"] = v
	}
	if v, ok := body["temperature"]; ok {
		cfg["temperature"] = v
	}
	if len(cfg) > 0 {
		out["generationConfig"] = cfg
	}
	raw, _ := json.Marshal(out)
	return raw
}

func normalizeResponseForMode(resp *http.Response, mode string) (*http.Response, error) {
	if mode != "gemini" {
		return resp, nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		return resp, nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		return resp, nil
	}
	normalized := normalizeGeminiToOpenAIResponse(payload)
	body, _ := json.Marshal(normalized)
	headers := cloneHeader(resp.Header)
	headers.Set("Content-Type", "application/json")
	return cloneResponse(resp.StatusCode, headers, body), nil
}

func normalizeGeminiToOpenAIResponse(raw map[string]interface{}) map[string]interface{} {
	text := ""
	finishReason := "stop"
	if candidates, ok := raw["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if candidate, ok := candidates[0].(map[string]interface{}); ok {
			if v, ok := candidate["finishReason"].(string); ok && strings.TrimSpace(v) != "" {
				finishReason = strings.ToLower(strings.TrimSpace(v))
			}
			if content, ok := candidate["content"].(map[string]interface{}); ok {
				if parts, ok := content["parts"].([]interface{}); ok {
					chunks := make([]string, 0, len(parts))
					for _, rawPart := range parts {
						part, _ := rawPart.(map[string]interface{})
						if v, ok := part["text"].(string); ok && strings.TrimSpace(v) != "" {
							chunks = append(chunks, v)
						}
					}
					text = strings.Join(chunks, "\n")
				}
			}
		}
	}
	return map[string]interface{}{
		"id":      "chatcmpl-gemini-compatible",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": finishReason,
			},
		},
	}
}

func cloneRequestBody(body map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(body))
	for k, v := range body {
		out[k] = v
	}
	return out
}

func connectionHasModelControls(c store.ProviderConnection) bool {
	return len(c.ExcludedModels) > 0 || len(c.ModelAliases) > 0
}

func anyConnectionHasModelControls(candidates []store.ProviderConnection) bool {
	for _, c := range candidates {
		if connectionHasModelControls(c) {
			return true
		}
	}
	return false
}

func connectionModelExcluded(c store.ProviderConnection, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	for _, item := range c.ExcludedModels {
		item = strings.TrimSpace(item)
		if item == model {
			return true
		}
		if strings.Contains(model, "/") && strings.TrimPrefix(model, c.Provider+"/") == item {
			return true
		}
	}
	return false
}

func applyConnectionModelAlias(c store.ProviderConnection, model string) string {
	model = strings.TrimSpace(model)
	if model == "" || len(c.ModelAliases) == 0 {
		return model
	}
	if target := strings.TrimSpace(c.ModelAliases[model]); target != "" {
		return target
	}
	if strings.Contains(model, "/") {
		unprefixed := strings.TrimPrefix(model, c.Provider+"/")
		if target := strings.TrimSpace(c.ModelAliases[unprefixed]); target != "" {
			return target
		}
	}
	return model
}

func parseRetryAfter(value string, now time.Time) (time.Time, bool) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Time{}, false
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds < 1 {
			seconds = 1
		}
		return now.Add(time.Duration(seconds) * time.Second), true
	}
	if t, err := http.ParseTime(raw); err == nil && t.After(now) {
		return t, true
	}
	return time.Time{}, false
}

func parseRateLimitReset(headers http.Header, now time.Time) (time.Time, bool) {
	if retryAfter, ok := parseRetryAfter(headers.Get("Retry-After"), now); ok {
		return retryAfter, true
	}
	for _, key := range []string{"x-ratelimit-reset", "x-rate-limit-reset", "ratelimit-reset"} {
		value := strings.TrimSpace(headers.Get(key))
		if value == "" {
			continue
		}
		epoch, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		if epoch > 10_000_000_000 {
			t := time.UnixMilli(epoch)
			if t.After(now) {
				return t, true
			}
			continue
		}
		t := time.Unix(epoch, 0)
		if t.After(now) {
			return t, true
		}
	}
	return time.Time{}, false
}

func getFallbackCooldown(status int) time.Duration {
	switch status {
	case 429:
		return 15 * time.Second
	case 502, 503, 504, 520, 521, 522, 524, 529:
		return 8 * time.Second
	default:
		return 5 * time.Second
	}
}

func getCircuitOpenDuration(failures int) time.Duration {
	switch {
	case failures >= 6:
		return 90 * time.Second
	case failures >= 4:
		return 45 * time.Second
	default:
		return 20 * time.Second
	}
}

func maxForwardAttempts(settings store.Settings) int {
	if settings.MaxRetries <= 0 {
		return 3
	}
	return settings.MaxRetries
}

func rotateCandidates(items []store.ProviderConnection, offset int) []store.ProviderConnection {
	if len(items) == 0 {
		return items
	}
	offset = offset % len(items)
	if offset == 0 {
		return append([]store.ProviderConnection(nil), items...)
	}
	out := make([]store.ProviderConnection, 0, len(items))
	out = append(out, items[offset:]...)
	out = append(out, items[:offset]...)
	return out
}

func quotaAwareCandidates(candidates []store.ProviderConnection, defaultRPM int) []store.ProviderConnection {
	out := append([]store.ProviderConnection(nil), candidates...)
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i].RequestsPerMinute
		right := out[j].RequestsPerMinute
		if left <= 0 {
			left = defaultRPM
		}
		if right <= 0 {
			right = defaultRPM
		}
		if left == right {
			if out[i].Priority == out[j].Priority {
				return out[i].CreatedAt > out[j].CreatedAt
			}
			return out[i].Priority < out[j].Priority
		}
		return left > right
	})
	return out
}

func weightedFirstIndex(candidates []store.ProviderConnection, cursor int) int {
	if len(candidates) == 0 {
		return 0
	}
	weighted := make([]int, 0, len(candidates)*2)
	for i, candidate := range candidates {
		weight := candidate.AccountWeight
		if weight <= 0 {
			weight = 1
		}
		for j := 0; j < weight; j++ {
			weighted = append(weighted, i)
		}
	}
	if len(weighted) == 0 {
		return 0
	}
	return weighted[cursor%len(weighted)]
}

func (f *Forwarder) reorderCandidates(scope, model string, candidates []store.ProviderConnection) []store.ProviderConnection {
	settings := f.store.GetSettings()
	candidates = quotaAwareCandidates(candidates, settings.DefaultRequestsPerMinute)
	switch settings.ComboStrategy {
	case "round_robin":
		f.rrMu.Lock()
		defer f.rrMu.Unlock()
		key := "global"
		offset := weightedFirstIndex(candidates, f.rrIndex[key])
		f.rrIndex[key] = f.rrIndex[key] + 1
		return rotateCandidates(candidates, offset)
	case "sticky_round_robin":
		sum := sha1.Sum([]byte(scope + "|" + model))
		offset := weightedFirstIndex(candidates, int(sum[0]))
		return rotateCandidates(candidates, offset)
	default:
		return candidates
	}
}

type SearchRequest struct {
	Query        string `json:"query"`
	MaxResults   int    `json:"maxResults,omitempty"`
	ProviderHint string `json:"provider,omitempty"`
}

type SearchResult struct {
	Title   string `json:"title,omitempty"`
	URL     string `json:"url,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

type SearchResponse struct {
	Provider string         `json:"provider,omitempty"`
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
	Raw      interface{}    `json:"raw,omitempty"`
}

type MediaRequest struct {
	Path     string
	Body     []byte
	Provider string
	Headers  http.Header
}

type ProbeResult struct {
	Provider   string `json:"provider"`
	Model      string `json:"model,omitempty"`
	Healthy    bool   `json:"healthy"`
	StatusCode int    `json:"statusCode,omitempty"`
	Message    string `json:"message,omitempty"`
	LatencyMs  int64  `json:"latencyMs"`
}

type ProviderModelsResult struct {
	Provider string   `json:"provider"`
	Models   []string `json:"models"`
	Source   string   `json:"source"`
}

type OAuthRefreshResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	TokenExpiry  string `json:"tokenExpiry,omitempty"`
	Raw          any    `json:"raw,omitempty"`
}

func (f *Forwarder) Search(ctx context.Context, request SearchRequest) (SearchResponse, error) {
	if err := f.refreshTransport(); err != nil {
		return SearchResponse{}, err
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return SearchResponse{}, fmt.Errorf("query is required")
	}
	if request.MaxResults <= 0 {
		request.MaxResults = 5
	}
	if request.MaxResults > 20 {
		request.MaxResults = 20
	}
	candidates := f.store.GetActiveConnections(strings.TrimSpace(request.ProviderHint))
	if len(candidates) == 0 && strings.TrimSpace(request.ProviderHint) != "" {
		candidates = f.store.GetActiveConnections("")
	}
	filtered := make([]store.ProviderConnection, 0, len(candidates))
	for _, c := range candidates {
		if entry, ok := store.GetProviderCatalogEntry(c.Provider); ok && entry.APIType == "search" {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return SearchResponse{}, fmt.Errorf("no active search provider connections")
	}
	maxAttempts := maxForwardAttempts(f.store.GetSettings())
	attempts := 0
	var lastErr error
	for _, c := range filtered {
		if attempts >= maxAttempts {
			break
		}
		attempts++
		result, err := f.searchWithConnection(ctx, c, query, request.MaxResults)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !strings.HasPrefix(err.Error(), "upstream ") {
			_ = f.store.MarkConnectionCooldown(c.ID, time.Now().Add(getCircuitOpenDuration(c.ConsecutiveFailures+1)), 0, err.Error())
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all search providers failed")
	}
	return SearchResponse{}, lastErr
}

func (f *Forwarder) searchWithConnection(ctx context.Context, c store.ProviderConnection, query string, maxResults int) (SearchResponse, error) {
	entry, ok := store.GetProviderCatalogEntry(c.Provider)
	if !ok || entry.APIType != "search" {
		return SearchResponse{}, fmt.Errorf("provider %s is not a search provider", c.Provider)
	}
	baseURL := resolveBaseURL(c, entry.BaseURL)
	if c.ProviderSpecificData != nil {
		if v, ok := c.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			baseURL = resolveBaseURL(c, v)
		}
	}
	req, err := buildSearchRequest(ctx, c, baseURL, query, maxResults)
	if err != nil {
		return SearchResponse{}, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return SearchResponse{}, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return SearchResponse{}, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		until, ok := parseRateLimitReset(resp.Header, time.Now())
		if !ok {
			until = time.Now().Add(getFallbackCooldown(resp.StatusCode))
		}
		_ = f.store.MarkConnectionCooldown(c.ID, until, resp.StatusCode, string(rawBody))
		return SearchResponse{}, fmt.Errorf("upstream %s status %d", c.Provider, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SearchResponse{}, fmt.Errorf("upstream %s status %d", c.Provider, resp.StatusCode)
	}
	var raw interface{}
	_ = json.Unmarshal(rawBody, &raw)
	results := normalizeSearchResults(c.Provider, raw)
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	if c.RateLimitedUntil != "" || c.CircuitOpenUntil != "" || c.TestStatus == "unavailable" || c.BackoffLevel > 0 || c.ConsecutiveFailures > 0 {
		_ = f.store.ClearConnectionCooldown(c.ID)
	}
	return SearchResponse{Provider: c.Provider, Query: query, Results: results, Raw: raw}, nil
}

func buildSearchRequest(ctx context.Context, c store.ProviderConnection, baseURL, query string, maxResults int) (*http.Request, error) {
	switch c.Provider {
	case "brave-search":
		u, err := url.Parse(baseURL + "/web/search")
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("q", query)
		q.Set("count", strconv.Itoa(maxResults))
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Subscription-Token", c.APIKey)
		return req, nil
	case "serper", "serper-search":
		req, err := postJSONSearch(ctx, baseURL+"/search", c, map[string]interface{}{"q": query, "num": maxResults})
		if err != nil {
			return nil, err
		}
		req.Header.Del("Authorization")
		req.Header.Set("X-API-KEY", c.APIKey)
		return req, nil
	case "tavily", "tavily-search":
		return postJSONSearch(ctx, baseURL+"/search", c, map[string]interface{}{"query": query, "max_results": maxResults})
	case "exa", "exa-search":
		return postJSONSearch(ctx, baseURL+"/search", c, map[string]interface{}{"query": query, "numResults": maxResults})
	case "perplexity-search":
		return postJSONSearch(ctx, baseURL+"/search", c, map[string]interface{}{"query": query, "max_results": maxResults})
	case "google-pse-search":
		u, err := url.Parse(baseURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("key", c.APIKey)
		q.Set("q", query)
		q.Set("num", strconv.Itoa(maxResults))
		if c.ProviderSpecificData != nil {
			if cx, ok := c.ProviderSpecificData["cx"].(string); ok && strings.TrimSpace(cx) != "" {
				q.Set("cx", strings.TrimSpace(cx))
			}
		}
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return req, nil
	default:
		return nil, fmt.Errorf("search provider %s not supported", c.Provider)
	}
}

func postJSONSearch(ctx context.Context, endpoint string, c store.ProviderConnection, body map[string]interface{}) (*http.Request, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	setAuthHeader(req, c, "openai")
	return req, nil
}

func normalizeSearchResults(provider string, raw interface{}) []SearchResult {
	root, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	switch provider {
	case "brave-search":
		if web, ok := root["web"].(map[string]interface{}); ok {
			return normalizeResultArray(web["results"], "title", "url", "description")
		}
	case "serper", "serper-search":
		return normalizeResultArray(root["organic"], "title", "link", "snippet")
	case "tavily", "tavily-search", "exa", "exa-search", "perplexity-search":
		return normalizeResultArray(root["results"], "title", "url", "content")
	case "google-pse-search":
		return normalizeResultArray(root["items"], "title", "link", "snippet")
	}
	return nil
}

func normalizeResultArray(raw interface{}, titleKey, urlKey, snippetKey string) []SearchResult {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	results := make([]SearchResult, 0, len(items))
	for _, item := range items {
		node, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		results = append(results, SearchResult{
			Title:   fmt.Sprint(node[titleKey]),
			URL:     fmt.Sprint(node[urlKey]),
			Snippet: fmt.Sprint(node[snippetKey]),
		})
	}
	return results
}

func (f *Forwarder) GetProviderModels(ctx context.Context, c store.ProviderConnection) (ProviderModelsResult, error) {
	cacheKey := c.ID
	now := time.Now()
	f.modelsMu.Lock()
	if entry, ok := f.modelsCache[cacheKey]; ok && now.Before(entry.expiresAt) {
		out := make([]string, len(entry.models))
		copy(out, entry.models)
		f.modelsMu.Unlock()
		return ProviderModelsResult{Provider: c.Provider, Models: out, Source: entry.source}, nil
	}
	f.modelsMu.Unlock()

	models, source, err := f.fetchProviderModels(ctx, c)
	if err != nil {
		return ProviderModelsResult{}, err
	}
	f.modelsMu.Lock()
	f.modelsCache[cacheKey] = modelsCacheEntry{
		expiresAt: time.Now().Add(modelsCacheTTL),
		models:    append([]string(nil), models...),
		source:    source,
	}
	f.modelsMu.Unlock()
	return ProviderModelsResult{Provider: c.Provider, Models: models, Source: source}, nil
}

func (f *Forwarder) fetchProviderModels(ctx context.Context, c store.ProviderConnection) ([]string, string, error) {
	if err := f.refreshTransport(); err != nil {
		return nil, "", err
	}
	entry, ok := store.GetProviderCatalogEntry(c.Provider)
	if !ok {
		return nil, "", fmt.Errorf("unknown provider %s", c.Provider)
	}
	baseURL := resolveBaseURL(c, entry.BaseURL)
	if c.ProviderSpecificData != nil {
		if v, ok := c.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			baseURL = resolveBaseURL(c, v)
		}
	}
	modelsURL := joinOpenAIEndpoint(baseURL, "/v1/models")
	if entry.APIType == "anthropic" {
		return fallbackProviderModels(c, entry), "fallback", nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	setAuthHeader(req, c, entry.APIType)
	resp, err := f.client.Do(req)
	if err != nil {
		return fallbackProviderModels(c, entry), "fallback", nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fallbackProviderModels(c, entry), "fallback", nil
	}
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return fallbackProviderModels(c, entry), "fallback", nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return fallbackProviderModels(c, entry), "fallback", nil
	}
	items, ok := payload["data"].([]interface{})
	if !ok {
		return fallbackProviderModels(c, entry), "fallback", nil
	}
	models := make([]string, 0, len(items))
	for _, item := range items {
		node, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(node["id"]))
		if id == "" || id == "<nil>" {
			continue
		}
		if !strings.Contains(id, "/") {
			id = c.Provider + "/" + id
		}
		models = append(models, id)
	}
	if len(models) == 0 {
		return fallbackProviderModels(c, entry), "fallback", nil
	}
	slices.Sort(models)
	return dedupeStrings(models), "live", nil
}

func fallbackProviderModels(c store.ProviderConnection, entry store.ProviderCatalogEntry) []string {
	models := make([]string, 0, len(entry.FallbackModels)+1)
	for _, item := range entry.FallbackModels {
		if strings.TrimSpace(item) != "" {
			models = append(models, strings.TrimSpace(item))
		}
	}
	if strings.TrimSpace(c.DefaultModel) != "" {
		models = append(models, strings.TrimSpace(c.DefaultModel))
	}
	if len(models) == 0 && c.Provider != "" {
		models = append(models, c.Provider+"/default")
	}
	slices.Sort(models)
	return dedupeStrings(models)
}

func dedupeStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func (f *Forwarder) ProbeConnection(ctx context.Context, c store.ProviderConnection) (ProbeResult, error) {
	if err := f.refreshTransport(); err != nil {
		return ProbeResult{}, err
	}
	entry, ok := store.GetProviderCatalogEntry(c.Provider)
	if !ok {
		return ProbeResult{}, fmt.Errorf("unknown provider %s", c.Provider)
	}
	baseURL := resolveBaseURL(c, entry.BaseURL)
	if c.ProviderSpecificData != nil {
		if v, ok := c.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			baseURL = resolveBaseURL(c, v)
		}
	}
	targetURL, method, body := probeRequestForProvider(entry, baseURL, c.DefaultModel)
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		return ProbeResult{}, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	setAuthHeader(req, c, entry.APIType)
	resp, err := f.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return ProbeResult{Provider: c.Provider, Healthy: false, Message: err.Error(), LatencyMs: latency}, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1*1024*1024))
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	msg := ""
	if !healthy {
		msg = fmt.Sprintf("status %d", resp.StatusCode)
	}
	result := ProbeResult{
		Provider:   c.Provider,
		Model:      strings.TrimSpace(c.DefaultModel),
		Healthy:    healthy,
		StatusCode: resp.StatusCode,
		Message:    msg,
		LatencyMs:  latency,
	}
	if !healthy {
		return result, fmt.Errorf("provider probe failed: %s", msg)
	}
	return result, nil
}

func (f *Forwarder) ProbeModels(ctx context.Context, c store.ProviderConnection, models []string) ([]ProbeResult, error) {
	if err := f.refreshTransport(); err != nil {
		return nil, err
	}
	if len(models) == 0 {
		if strings.TrimSpace(c.DefaultModel) != "" {
			models = []string{strings.TrimSpace(c.DefaultModel)}
		} else {
			return nil, fmt.Errorf("models are required")
		}
	}
	entry, ok := store.GetProviderCatalogEntry(c.Provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider %s", c.Provider)
	}
	baseURL := resolveBaseURL(c, entry.BaseURL)
	if c.ProviderSpecificData != nil {
		if v, ok := c.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			baseURL = resolveBaseURL(c, v)
		}
	}
	results := make([]ProbeResult, 0, len(models))
	for _, model := range models {
		targetURL, method, body := probeRequestForProvider(entry, baseURL, model)
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
		if err != nil {
			results = append(results, ProbeResult{Provider: c.Provider, Model: model, Healthy: false, Message: err.Error()})
			continue
		}
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		setAuthHeader(req, c, entry.APIType)
		resp, err := f.client.Do(req)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			results = append(results, ProbeResult{Provider: c.Provider, Model: model, Healthy: false, Message: err.Error(), LatencyMs: latency})
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1*1024*1024))
		_ = resp.Body.Close()
		healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
		msg := ""
		if !healthy {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		results = append(results, ProbeResult{
			Provider:   c.Provider,
			Model:      strings.TrimSpace(model),
			Healthy:    healthy,
			StatusCode: resp.StatusCode,
			Message:    msg,
			LatencyMs:  latency,
		})
	}
	return results, nil
}

func probeRequestForProvider(entry store.ProviderCatalogEntry, baseURL, defaultModel string) (url, method string, body []byte) {
	switch entry.APIType {
	case "search":
		switch entry.Provider {
		case "brave-search":
			return baseURL + "/web/search?q=healthcheck&count=1", http.MethodGet, nil
		case "serper":
			raw, _ := json.Marshal(map[string]interface{}{"q": "healthcheck", "num": 1})
			return baseURL + "/search", http.MethodPost, raw
		default:
			raw, _ := json.Marshal(map[string]interface{}{"query": "healthcheck", "max_results": 1})
			return baseURL + "/search", http.MethodPost, raw
		}
	case "embedding":
		model := strings.TrimSpace(defaultModel)
		if model == "" {
			model = "text-embedding-3-small"
		}
		raw, _ := json.Marshal(map[string]interface{}{"model": model, "input": "healthcheck"})
		return joinOpenAIEndpoint(baseURL, "/v1/embeddings"), http.MethodPost, raw
	case "tts":
		model := strings.TrimSpace(defaultModel)
		if model == "" {
			model = "gpt-4o-mini-tts"
		}
		raw, _ := json.Marshal(map[string]interface{}{"model": model, "input": "ok", "voice": "alloy"})
		return joinOpenAIEndpoint(baseURL, "/v1/audio/speech"), http.MethodPost, raw
	case "stt":
		return baseURL, http.MethodGet, nil
	case "anthropic":
		model := strings.TrimSpace(defaultModel)
		if model == "" {
			model = "claude-3-5-sonnet-latest"
		}
		raw, _ := json.Marshal(map[string]interface{}{
			"model":      model,
			"max_tokens": 8,
			"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		})
		return joinOpenAIEndpoint(baseURL, "/v1/messages"), http.MethodPost, raw
	default:
		model := strings.TrimSpace(defaultModel)
		if model == "" {
			model = "gpt-4o-mini"
		}
		raw, _ := json.Marshal(map[string]interface{}{
			"model":      model,
			"max_tokens": 8,
			"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		})
		return joinOpenAIEndpoint(baseURL, "/v1/chat/completions"), http.MethodPost, raw
	}
}

func (f *Forwarder) RefreshOAuthToken(ctx context.Context, c store.ProviderConnection, tokenURL, clientID, clientSecret string) (OAuthRefreshResult, error) {
	if err := f.refreshTransport(); err != nil {
		return OAuthRefreshResult{}, err
	}
	if strings.TrimSpace(c.RefreshToken) == "" {
		return OAuthRefreshResult{}, fmt.Errorf("refresh token is required")
	}
	if strings.TrimSpace(tokenURL) == "" {
		if entry, ok := store.GetProviderCatalogEntry(c.Provider); ok {
			tokenURL = strings.TrimSpace(entry.TokenURL)
		}
	}
	if strings.TrimSpace(tokenURL) == "" {
		return OAuthRefreshResult{}, fmt.Errorf("token url is required")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", c.RefreshToken)
	if strings.TrimSpace(clientID) != "" {
		form.Set("client_id", strings.TrimSpace(clientID))
	}
	if strings.TrimSpace(clientSecret) != "" {
		form.Set("client_secret", strings.TrimSpace(clientSecret))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthRefreshResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return OAuthRefreshResult{}, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return OAuthRefreshResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OAuthRefreshResult{}, fmt.Errorf("oauth refresh failed with status %d", resp.StatusCode)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return OAuthRefreshResult{}, err
	}
	result := OAuthRefreshResult{
		AccessToken:  strings.TrimSpace(fmt.Sprint(payload["access_token"])),
		RefreshToken: strings.TrimSpace(fmt.Sprint(payload["refresh_token"])),
		TokenType:    strings.TrimSpace(fmt.Sprint(payload["token_type"])),
		Raw:          payload,
	}
	if result.AccessToken == "" || result.AccessToken == "<nil>" {
		return OAuthRefreshResult{}, fmt.Errorf("oauth refresh response missing access_token")
	}
	if result.RefreshToken == "<nil>" {
		result.RefreshToken = ""
	}
	if expiresIn, ok := payload["expires_in"]; ok {
		if seconds, ok := asNumberishInt64(expiresIn); ok && seconds > 0 {
			result.TokenExpiry = time.Now().UTC().Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
		}
	}
	return result, nil
}

func asNumberishInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func oauthRefreshConfig(c store.ProviderConnection) (tokenURL, clientID, clientSecret string) {
	if c.ProviderSpecificData == nil {
		return "", "", ""
	}
	if v, ok := c.ProviderSpecificData["tokenUrl"].(string); ok {
		tokenURL = strings.TrimSpace(v)
	}
	if v, ok := c.ProviderSpecificData["clientId"].(string); ok {
		clientID = strings.TrimSpace(v)
	}
	if v, ok := c.ProviderSpecificData["clientSecret"].(string); ok {
		clientSecret = strings.TrimSpace(v)
	}
	return tokenURL, clientID, clientSecret
}

func shouldRefreshOAuthToken(c store.ProviderConnection, now time.Time) bool {
	if c.AuthType != "oauth" || strings.TrimSpace(c.RefreshToken) == "" {
		return false
	}
	if strings.TrimSpace(c.TokenExpiry) == "" {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, c.TokenExpiry)
	if err != nil {
		return false
	}
	return expiry.Before(now.Add(5 * time.Minute))
}

func (f *Forwarder) maybeRefreshOAuthConnection(ctx context.Context, c store.ProviderConnection) (store.ProviderConnection, error) {
	now := time.Now().UTC()
	if !shouldRefreshOAuthToken(c, now) {
		return c, nil
	}
	tokenURL, clientID, clientSecret := oauthRefreshConfig(c)
	result, err := f.RefreshOAuthToken(ctx, c, tokenURL, clientID, clientSecret)
	if err != nil {
		return c, err
	}
	updated, err := f.store.UpdateOAuthTokens(c.ID, result.AccessToken, result.RefreshToken, result.TokenExpiry)
	if err != nil {
		return c, err
	}
	return updated, nil
}

func (f *Forwarder) ForwardMedia(ctx context.Context, request MediaRequest) (*http.Response, error) {
	if err := f.refreshTransport(); err != nil {
		return nil, err
	}
	apiType := mediaAPIType(request.Path)
	if apiType == "" {
		return nil, fmt.Errorf("unsupported media path")
	}
	candidates := f.store.GetActiveConnections(strings.TrimSpace(request.Provider))
	if len(candidates) == 0 && strings.TrimSpace(request.Provider) != "" {
		candidates = f.store.GetActiveConnections("")
	}
	filtered := make([]store.ProviderConnection, 0, len(candidates))
	for _, c := range candidates {
		if entry, ok := store.GetProviderCatalogEntry(c.Provider); ok && supportsMediaAPI(c.Provider, entry.APIType, apiType) {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no active %s provider connections", apiType)
	}
	maxAttempts := maxForwardAttempts(f.store.GetSettings())
	attempts := 0
	var lastErr error
	for _, c := range filtered {
		if attempts >= maxAttempts {
			break
		}
		attempts++
		resp, err := f.forwardMediaWithConnection(ctx, c, request, apiType)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !strings.HasPrefix(err.Error(), "upstream ") {
			_ = f.store.MarkConnectionCooldown(c.ID, time.Now().Add(getCircuitOpenDuration(c.ConsecutiveFailures+1)), 0, err.Error())
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all %s providers failed", apiType)
	}
	return nil, lastErr
}

func mediaAPIType(path string) string {
	switch path {
	case "/v1/embeddings":
		return "embedding"
	case "/v1/images/generations", "/v1/images/edits":
		return "image"
	case "/v1/images/analyze":
		return "vision"
	case "/v1/videos/generations", "/v1/videos/edits", "/v1/videos/extensions":
		return "video"
	case "/v1/audio/generations":
		return "music"
	case "/v1/audio/speech":
		return "tts"
	case "/v1/audio/transcriptions":
		return "stt"
	default:
		return ""
	}
}

func supportsMediaAPI(provider, providerAPIType, requested string) bool {
	if providerAPIType == requested {
		return true
	}
	return provider == "openai" && providerAPIType == "openai" && (requested == "embedding" || requested == "image" || requested == "vision" || requested == "video" || requested == "music" || requested == "tts" || requested == "stt")
}

func (f *Forwarder) forwardMediaWithConnection(ctx context.Context, c store.ProviderConnection, request MediaRequest, apiType string) (*http.Response, error) {
	endpoint, mode, err := resolveMediaEndpoint(c, request.Path, apiType)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(request.Body))
	if err != nil {
		return nil, err
	}
	for k, values := range request.Headers {
		if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Host") || strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, value := range values {
			req.Header.Add(k, value)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	setAuthHeader(req, c, mode)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		now := time.Now()
		until, ok := parseRateLimitReset(resp.Header, now)
		if !ok {
			until = now.Add(getFallbackCooldown(resp.StatusCode))
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		_ = f.store.MarkConnectionCooldown(c.ID, until, resp.StatusCode, fmt.Sprintf("upstream %s status %d", c.Provider, resp.StatusCode))
		return nil, fmt.Errorf("upstream %s status %d", c.Provider, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}
	if c.RateLimitedUntil != "" || c.CircuitOpenUntil != "" || c.TestStatus == "unavailable" || c.BackoffLevel > 0 || c.ConsecutiveFailures > 0 {
		_ = f.store.ClearConnectionCooldown(c.ID)
	}
	resp.Header.Set("X-XRouter-Provider", c.Provider)
	return resp, nil
}

func resolveMediaEndpoint(c store.ProviderConnection, path, apiType string) (string, string, error) {
	entry, ok := store.GetProviderCatalogEntry(c.Provider)
	if !ok {
		return "", "", fmt.Errorf("provider %s missing baseUrl", c.Provider)
	}
	baseURL := strings.TrimRight(entry.BaseURL, "/")
	if c.ProviderSpecificData != nil {
		if v, ok := c.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			baseURL = strings.TrimRight(strings.TrimSpace(v), "/")
		}
	}
	switch apiType {
	case "embedding":
		return joinOpenAIEndpoint(baseURL, "/v1/embeddings"), "openai", nil
	case "image":
		return joinOpenAIEndpoint(baseURL, path), "openai", nil
	case "vision":
		return joinOpenAIEndpoint(baseURL, "/v1/chat/completions"), "openai", nil
	case "video":
		return joinOpenAIEndpoint(baseURL, path), "openai", nil
	case "music":
		return joinOpenAIEndpoint(baseURL, "/v1/audio/generations"), "openai", nil
	case "tts":
		return joinOpenAIEndpoint(baseURL, "/v1/audio/speech"), "openai", nil
	case "stt":
		if c.Provider == "deepgram" {
			return baseURL + "/listen", "deepgram", nil
		}
		return joinOpenAIEndpoint(baseURL, "/v1/audio/transcriptions"), "openai", nil
	default:
		return "", "", fmt.Errorf("unsupported media api type")
	}
}

func (f *Forwarder) Forward(ctx context.Context, scope, path string, requestBody []byte) (*http.Response, error) {
	if err := f.refreshTransport(); err != nil {
		return nil, err
	}

	var body map[string]interface{}
	if err := json.Unmarshal(requestBody, &body); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	model := extractModel(body)
	if targets, ok := f.store.GetComboModelsMap()[model]; ok && len(targets) > 0 {
		var lastErr error
		for _, target := range targets {
			nextBody := cloneRequestBody(body)
			nextBody["model"] = target
			raw, _ := json.Marshal(nextBody)
			resp, err := f.Forward(ctx, scope, path, raw)
			if err == nil {
				return resp, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
	}
	poolHint := extractPoolHint(body)
	nodeHint := extractNodeHint(body)
	delete(body, "pool")
	delete(body, "node")
	var selectedPolicy store.RoutePolicy
	hasSelectedPolicy := false
	for _, policy := range f.store.ListRoutePolicies() {
		if !matchRoutePolicy(policy, model, "") {
			continue
		}
		selectedPolicy = policy
		hasSelectedPolicy = true
		if poolHint == "" && strings.TrimSpace(policy.TargetPoolID) != "" {
			poolHint = strings.TrimSpace(policy.TargetPoolID)
		}
		if nodeHint == "" && strings.TrimSpace(policy.TargetNodeID) != "" {
			nodeHint = strings.TrimSpace(policy.TargetNodeID)
		}
		if forced := strings.TrimSpace(policy.ForceModel); forced != "" {
			body["model"] = forced
			model = forced
		}
		break
	}
	if model != "" {
		forcedMappings := f.store.GetForcedModelMappings()
		if target, ok := forcedMappings[model]; ok && strings.TrimSpace(target) != "" {
			body["model"] = target
			model = target
		}
	}
	providerHint := ""
	if strings.Contains(model, "/") {
		providerHint = strings.SplitN(model, "/", 2)[0]
	}

	candidates := f.store.GetActiveConnections(providerHint)
	if len(candidates) == 0 && providerHint != "" {
		candidates = f.store.GetActiveConnections("")
	}
	if hasSelectedPolicy {
		candidates = filterCandidatesByPolicy(candidates, selectedPolicy)
	}
	if strings.TrimSpace(poolHint) != "" {
		if pool, ok := f.store.GetProxyPool(poolHint); ok {
			allowed := map[string]bool{}
			for _, id := range pool.ConnectionIDs {
				allowed[id] = true
			}
			filtered := make([]store.ProviderConnection, 0, len(candidates))
			for _, c := range candidates {
				if allowed[c.ID] {
					filtered = append(filtered, c)
				}
			}
			candidates = filtered
		} else {
			return nil, fmt.Errorf("proxy pool not found")
		}
	}
	if strings.TrimSpace(nodeHint) != "" {
		if node, ok := f.store.GetProviderNode(nodeHint); ok {
			allowed := map[string]bool{}
			for _, id := range node.ConnectionIDs {
				allowed[id] = true
			}
			filtered := make([]store.ProviderConnection, 0, len(candidates))
			for _, c := range candidates {
				if allowed[c.ID] {
					filtered = append(filtered, c)
				}
			}
			candidates = filtered
		} else {
			return nil, fmt.Errorf("provider node not found")
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no active provider connections")
	}
	candidates = f.reorderCandidates(scope, model, candidates)
	maxAttempts := maxForwardAttempts(f.store.GetSettings())
	attempts := 0

	upstreamBody := normalizeModelForUpstream(cloneRequestBody(body), providerHint)
	if !isStreaming(body) && f.store.GetSettings().ComboStrategy == "fallback" && strings.TrimSpace(poolHint) == "" && strings.TrimSpace(nodeHint) == "" && !hasSelectedPolicy && !anyConnectionHasModelControls(candidates) {
		if resp, err, handled := f.forwardDedup(ctx, scope, path, upstreamBody, model, providerHint); handled {
			return resp, err
		}
	}
	var lastErr error
	for _, c := range candidates {
		effectiveModel := applyConnectionModelAlias(c, model)
		if connectionModelExcluded(c, model) || connectionModelExcluded(c, effectiveModel) {
			continue
		}
		if attempts >= maxAttempts {
			break
		}
		attempts++
		if updated, err := f.maybeRefreshOAuthConnection(ctx, c); err == nil {
			c = updated
		}
		effectiveBody := cloneRequestBody(body)
		if effectiveModel != "" {
			effectiveBody["model"] = effectiveModel
		}
		endpoint, mode, err := resolveEndpoint(c, effectiveModel, path)
		if err != nil {
			lastErr = err
			continue
		}
		connectionBody := normalizeBodyForMode(effectiveBody, c.Provider, mode)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(connectionBody))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		setAuthHeader(req, c, mode)

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			_ = f.store.MarkConnectionCooldown(c.ID, time.Now().Add(getCircuitOpenDuration(c.ConsecutiveFailures+1)), 0, err.Error())
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized && c.AuthType == "oauth" && strings.TrimSpace(c.RefreshToken) != "" {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			tokenURL, clientID, clientSecret := oauthRefreshConfig(c)
			refreshed, refreshErr := f.RefreshOAuthToken(ctx, c, tokenURL, clientID, clientSecret)
			if refreshErr != nil {
				lastErr = refreshErr
				continue
			}
			updated, updateErr := f.store.UpdateOAuthTokens(c.ID, refreshed.AccessToken, refreshed.RefreshToken, refreshed.TokenExpiry)
			if updateErr != nil {
				lastErr = updateErr
				continue
			}
			c = updated
			retryReq, retryErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(connectionBody))
			if retryErr != nil {
				lastErr = retryErr
				continue
			}
			retryReq.Header.Set("Content-Type", "application/json")
			setAuthHeader(retryReq, c, mode)
			resp, err = f.client.Do(retryReq)
			if err != nil {
				lastErr = err
				_ = f.store.MarkConnectionCooldown(c.ID, time.Now().Add(getCircuitOpenDuration(c.ConsecutiveFailures+1)), 0, err.Error())
				continue
			}
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			now := time.Now()
			until, ok := parseRateLimitReset(resp.Header, now)
			if !ok {
				until = now.Add(getFallbackCooldown(resp.StatusCode))
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("upstream %s status %d", c.Provider, resp.StatusCode)
			_ = f.store.MarkConnectionCooldown(c.ID, until, resp.StatusCode, lastErr.Error())
			continue
		}
		if c.RateLimitedUntil != "" || c.CircuitOpenUntil != "" || c.TestStatus == "unavailable" || c.BackoffLevel > 0 || c.ConsecutiveFailures > 0 {
			_ = f.store.ClearConnectionCooldown(c.ID)
		}
		resp, err = normalizeResponseForMode(resp, mode)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Header.Set("X-XRouter-Provider", c.Provider)
		resp.Header.Set("X-XRouter-Connection-Id", c.ID)
		if upstreamModel, ok := effectiveBody["model"].(string); ok && strings.TrimSpace(upstreamModel) != "" {
			resp.Header.Set("X-XRouter-Model", strings.TrimSpace(upstreamModel))
		}
		return resp, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("all providers failed")
	}
	return nil, lastErr
}

func (f *Forwarder) forwardDedup(ctx context.Context, scope, path string, upstreamBody []byte, model, providerHint string) (*http.Response, error, bool) {
	key := dedupKey(scope, path, upstreamBody)
	now := time.Now()

	f.dedupMu.Lock()
	if current, ok := f.dedup[key]; ok {
		if !current.ready {
			waitCh := current.waitCh
			f.dedupMu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err(), true
			case <-waitCh:
				if current.err != nil {
					return nil, current.err, true
				}
				return cloneResponse(current.status, current.headers, current.body), nil, true
			}
		}
		if now.Before(current.expiresAt) {
			resp := cloneResponse(current.status, current.headers, current.body)
			f.dedupMu.Unlock()
			return resp, nil, true
		}
		delete(f.dedup, key)
	}
	entry := &dedupEntry{
		expiresAt: now.Add(dedupTTL),
		waitCh:    make(chan struct{}),
	}
	f.dedup[key] = entry
	f.dedupMu.Unlock()

	resp, err := f.forwardDirect(ctx, path, upstreamBody, model, providerHint)

	f.dedupMu.Lock()
	defer f.dedupMu.Unlock()
	if err != nil {
		entry.err = err
		entry.ready = true
		close(entry.waitCh)
		delete(f.dedup, key)
		return nil, err, true
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if readErr != nil {
		entry.err = readErr
		entry.ready = true
		close(entry.waitCh)
		delete(f.dedup, key)
		return nil, readErr, true
	}
	entry.expiresAt = time.Now().Add(dedupTTL)
	entry.ready = true
	entry.status = resp.StatusCode
	entry.headers = cloneHeader(resp.Header)
	entry.body = body
	close(entry.waitCh)
	return cloneResponse(entry.status, entry.headers, entry.body), nil, true
}

func (f *Forwarder) forwardDirect(ctx context.Context, path string, upstreamBody []byte, model, providerHint string) (*http.Response, error) {
	candidates := f.store.GetActiveConnections(providerHint)
	if len(candidates) == 0 && providerHint != "" {
		candidates = f.store.GetActiveConnections("")
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no active provider connections")
	}
	maxAttempts := maxForwardAttempts(f.store.GetSettings())
	attempts := 0

	var body map[string]interface{}
	_ = json.Unmarshal(upstreamBody, &body)
	var lastErr error
	for _, c := range candidates {
		if attempts >= maxAttempts {
			break
		}
		attempts++
		endpoint, mode, err := resolveEndpoint(c, model, path)
		if err != nil {
			lastErr = err
			continue
		}
		connectionBody := upstreamBody
		if len(body) > 0 {
			connectionBody = normalizeBodyForMode(cloneRequestBody(body), c.Provider, mode)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(connectionBody))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		setAuthHeader(req, c, mode)

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			_ = f.store.MarkConnectionCooldown(c.ID, time.Now().Add(getCircuitOpenDuration(c.ConsecutiveFailures+1)), 0, err.Error())
			continue
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			now := time.Now()
			until, ok := parseRateLimitReset(resp.Header, now)
			if !ok {
				until = now.Add(getFallbackCooldown(resp.StatusCode))
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("upstream %s status %d", c.Provider, resp.StatusCode)
			_ = f.store.MarkConnectionCooldown(c.ID, until, resp.StatusCode, lastErr.Error())
			continue
		}
		if c.RateLimitedUntil != "" || c.CircuitOpenUntil != "" || c.TestStatus == "unavailable" || c.BackoffLevel > 0 || c.ConsecutiveFailures > 0 {
			_ = f.store.ClearConnectionCooldown(c.ID)
		}
		resp, err = normalizeResponseForMode(resp, mode)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Header.Set("X-XRouter-Provider", c.Provider)
		resp.Header.Set("X-XRouter-Connection-Id", c.ID)
		if upstreamModel, ok := body["model"].(string); ok && strings.TrimSpace(upstreamModel) != "" {
			resp.Header.Set("X-XRouter-Model", strings.TrimSpace(upstreamModel))
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all providers failed")
	}
	return nil, lastErr
}
