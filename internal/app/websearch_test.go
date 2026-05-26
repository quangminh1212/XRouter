package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestWebSearchAliasProxy(t *testing.T) {
	tests := []struct {
		provider     string
		path         string
		body         string
		queryKey     string
		responseBody map[string]interface{}
	}{
		{provider: "brave-search", path: "/web/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "q", responseBody: map[string]interface{}{"web": map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "description": "ok"}}}}},
		{provider: "serper", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"organic": []map[string]string{{"title": "XRouter", "link": "https://example.com", "snippet": "ok"}}}},
		{provider: "serper-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"organic": []map[string]string{{"title": "XRouter", "link": "https://example.com", "snippet": "ok"}}}},
		{provider: "tavily", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "content": "ok"}}}},
		{provider: "tavily-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "content": "ok"}}}},
		{provider: "exa", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "content": "ok"}}}},
		{provider: "exa-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "content": "ok"}}}},
		{provider: "perplexity-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "content": "ok"}}}},
		{provider: "google-pse-search", path: "", body: `{"query":"xrouter","max_results":1}`, queryKey: "q", responseBody: map[string]interface{}{"items": []map[string]string{{"title": "XRouter", "link": "https://example.com", "snippet": "ok"}}}},
		{provider: "linkup", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"results": []map[string]string{{"name": "XRouter", "url": "https://example.com", "snippet": "ok"}}}},
		{provider: "linkup-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: "", responseBody: map[string]interface{}{"results": []map[string]string{{"name": "XRouter", "url": "https://example.com", "snippet": "ok"}}}},
		{provider: "searchapi", path: "", body: `{"query":"xrouter","max_results":1}`, queryKey: "q", responseBody: map[string]interface{}{"organic_results": []map[string]string{{"title": "XRouter", "link": "https://example.com", "snippet": "ok"}}}},
		{provider: "youcom", path: "", body: `{"query":"xrouter","max_results":1}`, queryKey: "query", responseBody: map[string]interface{}{"hits": []map[string]string{{"title": "XRouter", "url": "https://example.com", "description": "ok"}}}},
		{provider: "youcom-search", path: "", body: `{"query":"xrouter","max_results":1}`, queryKey: "query", responseBody: map[string]interface{}{"hits": []map[string]string{{"title": "XRouter", "url": "https://example.com", "description": "ok"}}}},
		{provider: "searxng", path: "", body: `{"query":"xrouter","max_results":1}`, queryKey: "q", responseBody: map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "content": "ok"}}}},
		{provider: "searxng-search", path: "", body: `{"query":"xrouter","max_results":1}`, queryKey: "q", responseBody: map[string]interface{}{"results": []map[string]string{{"title": "XRouter", "url": "https://example.com", "content": "ok"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			srv := newTestServer(t)
			if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
				t.Fatalf("disable api key auth: %v", err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.path != "" && r.URL.Path != tt.path && r.URL.Path != "/web/search" {
					t.Fatalf("unexpected upstream path: %s", r.URL.Path)
				}
				if tt.queryKey != "" {
					if got := r.URL.Query().Get(tt.queryKey); got != "xrouter" {
						t.Fatalf("unexpected query value: %q", got)
					}
				}
				writeJSON(w, http.StatusOK, tt.responseBody)
			}))
			defer upstream.Close()
			providerData := map[string]interface{}{
				"baseUrl": upstream.URL,
			}
			if tt.provider == "google-pse-search" {
				providerData["cx"] = "demo-cx"
			}
			_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
				Provider:             tt.provider,
				Name:                 tt.provider + " search",
				AuthType:             "apikey",
				APIKey:               "x",
				IsActive:             true,
				ProviderSpecificData: providerData,
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/web/search", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			results, ok := payload["results"].([]interface{})
			if !ok || len(results) == 0 {
				t.Fatalf("unexpected payload: %#v", payload)
			}
		})
	}
}
