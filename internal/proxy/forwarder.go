package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
}

func NewForwarder(st *store.Store) *Forwarder {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 256
	transport.MaxIdleConnsPerHost = 64
	transport.MaxConnsPerHost = 128
	transport.IdleConnTimeout = 90 * time.Second

	return &Forwarder{
		client: &http.Client{Transport: transport, Timeout: 0},
		store:  st,
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
	if baseURL == "" {
		switch c.Provider {
		case "openai":
			baseURL = "https://api.openai.com"
		case "anthropic":
			baseURL = "https://api.anthropic.com"
		case "openrouter":
			baseURL = "https://openrouter.ai/api"
		default:
			return "", "", fmt.Errorf("provider %s missing baseUrl", c.Provider)
		}
	}

	if path == "/v1/responses" || apiType == "responses" {
		return baseURL + "/responses", "openai", nil
	}
	if strings.Contains(model, "claude") || c.Provider == "anthropic" || strings.HasPrefix(c.Provider, "anthropic-compatible-") {
		return baseURL + "/v1/messages", "anthropic", nil
	}
	return baseURL + "/v1/chat/completions", "openai", nil
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
	if c.AccessToken != "" && c.APIKey == "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	if mode == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

func extractModel(body map[string]interface{}) string {
	if v, ok := body["model"].(string); ok {
		return v
	}
	return ""
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

func (f *Forwarder) Forward(ctx context.Context, path string, requestBody []byte) (*http.Response, error) {
	if err := f.refreshTransport(); err != nil {
		return nil, err
	}

	var body map[string]interface{}
	if err := json.Unmarshal(requestBody, &body); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	model := extractModel(body)
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
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no active provider connections")
	}

	upstreamBody := normalizeModelForUpstream(body, providerHint)
	var lastErr error
	for _, c := range candidates {
		endpoint, mode, err := resolveEndpoint(c, model, path)
		if err != nil {
			lastErr = err
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(upstreamBody))
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
		return resp, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("all providers failed")
	}
	return nil, lastErr
}
