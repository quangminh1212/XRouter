package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xrouter/internal/store"
)

func TestProviderScopedProxyChatPrefixesModel(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if payload["model"] != "gpt-4o-mini" {
			t.Fatalf("unexpected model payload: %#v", payload)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": "resp_1"})
	}))
	defer upstream.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai scoped",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/provider/openai/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProviderScopedProxyNormalizesPrefixedModelForUpstream(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !bytes.Contains(raw, []byte(`"model":"gpt-4o-mini"`)) {
			t.Fatalf("unexpected upstream body: %s", string(raw))
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": "resp_2"})
	}))
	defer upstream.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai scoped prefixed",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/provider/openai/v1/chat/completions", bytes.NewBufferString(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProviderScopedProxyRejectsUnsupportedPath(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/provider/openai/v1/unknown", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIV1ProviderScopedChatCompatRoute(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if payload["model"] != "gpt-4o-mini" {
			t.Fatalf("unexpected model payload: %#v", payload)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": "resp_api_v1_provider"})
	}))
	defer upstream.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai scoped api v1",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/providers/openai/chat/completions", bytes.NewBufferString(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIV1ProviderScopedEmbeddingsCompatRoute(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []map[string]interface{}{{"embedding": []float64{1, 2, 3}}}})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "openai scoped embed", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/providers/openai/embeddings", bytes.NewBufferString(`{"model":"text-embedding-3-small","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIProviderScopedModelsCompatRoute(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/provider/openai/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Models []map[string]string `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Models) == 0 {
		t.Fatalf("expected models payload, got %s", rec.Body.String())
	}
}

func TestAPIV1ProviderScopedModelsCompatRoute(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/openai/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Models []map[string]string `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Models) == 0 {
		t.Fatalf("expected models payload, got %s", rec.Body.String())
	}
}

func TestProviderScopedCompatRoutesCoverExtendedEndpoints(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		method       string
		body         string
		upstreamPath string
		kind         string
	}{
		{name: "count tokens", path: "/api/provider/anthropic/v1/messages/count_tokens", method: http.MethodPost, body: `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"hi"}]}`, upstreamPath: "/v1/messages/count_tokens", kind: "proxy"},
		{name: "responses compact", path: "/api/provider/openai/v1/responses/compact", method: http.MethodPost, body: `{"model":"gpt-4o-mini","input":"hi"}`, upstreamPath: "/responses", kind: "proxy"},
		{name: "image analyze", path: "/api/provider/openai/images/analyze", method: http.MethodPost, body: `{"model":"gpt-4o-mini","image":"data"}`, upstreamPath: "/v1/chat/completions", kind: "media"},
		{name: "audio voices", path: "/api/provider/openai/v1/audio/voices", method: http.MethodGet, kind: "voices"},
		{name: "audio transcriptions", path: "/api/provider/openai/v1/audio/transcriptions", method: http.MethodPost, body: `{"model":"whisper-1"}`, upstreamPath: "/v1/audio/transcriptions", kind: "media"},
		{name: "image edits", path: "/api/provider/openai/v1/images/edits", method: http.MethodPost, body: `{"model":"gpt-image-1"}`, upstreamPath: "/v1/images/edits", kind: "media"},
		{name: "image generations", path: "/api/provider/openai/v1/images/generations", method: http.MethodPost, body: `{"model":"gpt-image-1"}`, upstreamPath: "/v1/images/generations", kind: "media"},
		{name: "video generations", path: "/api/provider/openai/v1/videos/generations", method: http.MethodPost, body: `{"model":"gpt-video-1"}`, upstreamPath: "/v1/videos/generations", kind: "media"},
		{name: "web fetch", path: "/api/provider/openai/v1/web/fetch", method: http.MethodPost, body: `{"url":"https://example.com"}`, kind: "fetch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
				t.Fatalf("disable api key auth: %v", err)
			}
			if tt.kind != "voices" && tt.kind != "fetch" {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != tt.upstreamPath {
						t.Fatalf("unexpected upstream path: %s", r.URL.Path)
					}
					writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
				}))
				defer upstream.Close()
				provider := "openai"
				apiType := "openai"
				if tt.kind == "proxy" && strings.Contains(tt.path, "/anthropic/") {
					provider = "anthropic"
					apiType = "anthropic"
				}
				_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
					Provider: provider, Name: provider + " scoped extended", AuthType: "apikey", APIKey: "x", IsActive: true,
					ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": apiType},
				})
			}
			if tt.kind == "fetch" {
				t.Setenv("XR_ALLOW_PRIVATE_FETCH", "1")
			}
			var body io.Reader
			if tt.body != "" {
				body = bytes.NewBufferString(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), `"name":"xrouter"`) {
				t.Fatalf("route fell through to root handler: %s", rec.Body.String())
			}
		})
	}
}
