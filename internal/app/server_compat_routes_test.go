package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xrouter/internal/store"
)

func TestV0ManagementCompatRoutes(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/v0/management/health", want: "ok"},
		{path: "/v0/management/version", want: "version"},
		{path: "/v0/management/providers/catalog", want: "providers"},
		{path: "/v0/management/keys", want: "keys"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			srv := newTestServer(t)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Host = "localhost:1213"
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.want) {
				t.Fatalf("expected body to contain %q, got %s", tt.want, rec.Body.String())
			}
		})
	}
}

func TestAPIV1CompatChatRoute(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": "chat_1"})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai api v1 compat",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", bytes.NewBufferString(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOAuthCallbackAliasRoutes(t *testing.T) {
	for _, path := range []string{"/anthropic/callback", "/codex/callback", "/google/callback", "/iflow/callback"} {
		t.Run(path, func(t *testing.T) {
			srv := newTestServer(t)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for missing state, got %d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "missing state") {
				t.Fatalf("unexpected callback alias response: %s", rec.Body.String())
			}
		})
	}
}
