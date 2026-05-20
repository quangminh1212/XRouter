package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestProviderCatalogEndpoint(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/catalog?apiType=openai&authType=apikey", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Count     int                          `json:"count"`
		Providers []store.ProviderCatalogEntry `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count == 0 || len(payload.Providers) == 0 {
		t.Fatalf("expected providers in catalog: %#v", payload)
	}
	expected := map[string]string{
		"azure":          "",
		"nvidia":         "https://integrate.api.nvidia.com/v1",
		"moonshot":       "https://api.moonshot.ai/v1",
		"qwen":           "https://dashscope.aliyuncs.com/compatible-mode/v1",
		"hyperbolic":     "https://api.hyperbolic.xyz/v1",
		"byteplus":       "https://ark.ap-southeast.bytepluses.com/api/coding/v3",
		"cloudflare-ai":  "https://api.cloudflare.com/client/v4/accounts/{accountId}/ai/v1",
		"glm-cn":         "https://open.bigmodel.cn/api/paas/v4",
		"nebius":         "https://api.studio.nebius.ai/v1",
		"clarifai":       "https://api.clarifai.com/v2/ext/openai/v1",
		"opencode":       "https://api.opencode.ai/v1",
		"kiro":           "https://api.kiro.dev/v1",
		"reka":           "https://api.reka.ai/v1",
		"zai":            "https://api.z.ai/api/paas/v4",
		"featherless-ai": "https://api.featherless.ai/v1",
	}
	seen := make(map[string]string, len(payload.Providers))
	for _, provider := range payload.Providers {
		if provider.APIType != "openai" || provider.AuthType != "apikey" {
			t.Fatalf("unexpected filtered provider: %#v", provider)
		}
		seen[provider.Provider] = provider.BaseURL
	}
	for name, baseURL := range expected {
		if got := seen[name]; got != baseURL {
			t.Fatalf("expected %s baseUrl %s, got %q", name, baseURL, got)
		}
	}
}

func TestProviderCatalogIncludesGrokOAuthAlias(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/catalog?authType=oauth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Providers []store.ProviderCatalogEntry `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	wantOAuth := map[string]bool{"claude": false, "codex": false, "gemini": false, "gemini-cli": false, "xai": false, "grok": false, "kimi": false, "antigravity": false}
	for _, provider := range payload.Providers {
		if provider.AuthType != "oauth" {
			t.Fatalf("expected only oauth providers, got %#v", provider)
		}
		if _, ok := wantOAuth[provider.Provider]; ok {
			wantOAuth[provider.Provider] = true
		}
	}
	for name, found := range wantOAuth {
		if !found {
			t.Fatalf("missing oauth provider %s in catalog", name)
		}
	}
}

func TestProviderCatalogIncludesSearchAliases(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/catalog?apiType=search&authType=apikey", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Providers []store.ProviderCatalogEntry `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expected := map[string]string{
		"brave-search":      "https://api.search.brave.com/res/v1",
		"serper":            "https://google.serper.dev",
		"serper-search":     "https://google.serper.dev",
		"tavily":            "https://api.tavily.com",
		"tavily-search":     "https://api.tavily.com",
		"exa":               "https://api.exa.ai",
		"exa-search":        "https://api.exa.ai",
		"perplexity-search": "https://api.perplexity.ai",
		"google-pse-search": "https://customsearch.googleapis.com/customsearch/v1",
	}
	seen := map[string]string{}
	for _, provider := range payload.Providers {
		if provider.APIType != "search" || provider.AuthType != "apikey" {
			t.Fatalf("unexpected filtered provider: %#v", provider)
		}
		seen[provider.Provider] = provider.BaseURL
	}
	for name, baseURL := range expected {
		if got := seen[name]; got != baseURL {
			t.Fatalf("expected %s baseUrl %s, got %q", name, baseURL, got)
		}
	}
}

func TestProviderCatalogIncludesTTSProviders(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/catalog?apiType=tts&authType=apikey", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Providers []store.ProviderCatalogEntry `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expected := map[string]string{
		"openai-tts": "https://api.openai.com/v1",
		"elevenlabs": "https://api.elevenlabs.io/v1/text-to-speech",
		"cartesia":   "https://api.cartesia.ai/tts/bytes",
	}
	seen := map[string]string{}
	for _, provider := range payload.Providers {
		if provider.APIType != "tts" || provider.AuthType != "apikey" {
			t.Fatalf("unexpected filtered provider: %#v", provider)
		}
		seen[provider.Provider] = provider.BaseURL
	}
	for name, baseURL := range expected {
		if got := seen[name]; got != baseURL {
			t.Fatalf("expected %s baseUrl %s, got %q", name, baseURL, got)
		}
	}
}

func TestProviderCatalogIncludesImageProviders(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/catalog?apiType=image&authType=apikey", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Providers []store.ProviderCatalogEntry `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	seen := map[string]string{}
	for _, provider := range payload.Providers {
		if provider.APIType != "image" || provider.AuthType != "apikey" {
			t.Fatalf("unexpected filtered provider: %#v", provider)
		}
		seen[provider.Provider] = provider.BaseURL
	}
	if got := seen["black-forest-labs"]; got != "https://api.bfl.ai" {
		t.Fatalf("expected black-forest-labs baseUrl, got %q", got)
	}
}

func TestProviderCatalogIncludesWebCookieProviders(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/catalog?authType=web_cookie", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Providers []store.ProviderCatalogEntry `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expected := map[string]string{
		"chatgpt-web":    "https://chatgpt.com/backend-api",
		"gemini-web":     "https://gemini.google.com",
		"deepseek-web":   "https://chat.deepseek.com",
		"grok-web":       "https://grok.com",
		"perplexity-web": "https://www.perplexity.ai",
		"copilot-web":    "https://copilot.microsoft.com",
	}
	seen := map[string]string{}
	for _, provider := range payload.Providers {
		if provider.AuthType != "web_cookie" {
			t.Fatalf("unexpected filtered provider: %#v", provider)
		}
		seen[provider.Provider] = provider.BaseURL
	}
	for name, baseURL := range expected {
		if got := seen[name]; got != baseURL {
			t.Fatalf("expected %s baseUrl %s, got %q", name, baseURL, got)
		}
	}
}
