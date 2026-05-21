package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
