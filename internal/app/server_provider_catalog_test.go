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
		"nvidia":         "https://integrate.api.nvidia.com/v1",
		"moonshot":       "https://api.moonshot.ai/v1",
		"qwen":           "https://dashscope.aliyuncs.com/compatible-mode/v1",
		"hyperbolic":     "https://api.hyperbolic.xyz/v1",
		"glm-cn":         "https://open.bigmodel.cn/api/paas/v4",
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
	wantOAuth := map[string]bool{"claude": false, "codex": false, "gemini": false, "xai": false, "grok": false, "kimi": false, "antigravity": false}
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
