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
		provider string
		path     string
		body     string
		queryKey string
	}{
		{provider: "tavily", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: ""},
		{provider: "tavily-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: ""},
		{provider: "serper-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: ""},
		{provider: "exa-search", path: "/search", body: `{"query":"xrouter","max_results":1}`, queryKey: ""},
		{provider: "google-pse-search", path: "", body: `{"query":"xrouter","max_results":1}`, queryKey: "q"},
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
					writeJSON(w, http.StatusOK, map[string]interface{}{
						"items": []map[string]string{{"title": "XRouter", "link": "https://example.com", "snippet": "ok"}},
					})
					return
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"results": []map[string]string{{"title": "XRouter", "url": "https://example.com"}},
				})
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
			if _, ok := payload["results"]; !ok {
				t.Fatalf("unexpected payload: %#v", payload)
			}
		})
	}
}
