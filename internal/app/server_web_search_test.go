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
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" && r.URL.Path != "/web/search" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"results": []map[string]string{{"title": "XRouter", "url": "https://example.com"}},
		})
	}))
	defer upstream.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "tavily",
		Name:     "tavily search",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/web/search", bytes.NewBufferString(`{"query":"xrouter","max_results":1}`))
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
}
