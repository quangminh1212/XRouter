package app

import (
	"bytes"
	"encoding/json"
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

func TestV0ManagementAmpCodeCompatRoutes(t *testing.T) {
	srv := newTestServer(t)

	putURL := httptest.NewRequest(http.MethodPut, "/v0/management/ampcode/upstream-url", bytes.NewBufferString(`{"value":"https://amp.example.com"}`))
	putURL.Host = "localhost"
	putURLRec := httptest.NewRecorder()
	srv.ServeHTTP(putURLRec, putURL)
	if putURLRec.Code != http.StatusOK {
		t.Fatalf("expected 200 put upstream url, got %d body=%s", putURLRec.Code, putURLRec.Body.String())
	}

	putKey := httptest.NewRequest(http.MethodPut, "/v0/management/ampcode/upstream-api-key", bytes.NewBufferString(`{"value":"amp-secret"}`))
	putKey.Host = "localhost"
	putKeyRec := httptest.NewRecorder()
	srv.ServeHTTP(putKeyRec, putKey)
	if putKeyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 put upstream api key, got %d body=%s", putKeyRec.Code, putKeyRec.Body.String())
	}

	putKeys := httptest.NewRequest(http.MethodPut, "/v0/management/ampcode/upstream-api-keys", bytes.NewBufferString(`{"value":[{"upstream-api-key":" upstream ","api-keys":[" key-1 ","","key-2"]}]}`))
	putKeys.Host = "localhost"
	putKeysRec := httptest.NewRecorder()
	srv.ServeHTTP(putKeysRec, putKeys)
	if putKeysRec.Code != http.StatusOK {
		t.Fatalf("expected 200 put upstream api keys, got %d body=%s", putKeysRec.Code, putKeysRec.Body.String())
	}

	putMappings := httptest.NewRequest(http.MethodPut, "/v0/management/ampcode/model-mappings", bytes.NewBufferString(`{"value":[{"from":"gpt-4","to":"claude-sonnet"}]}`))
	putMappings.Host = "localhost"
	putMappingsRec := httptest.NewRecorder()
	srv.ServeHTTP(putMappingsRec, putMappings)
	if putMappingsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 put model mappings, got %d body=%s", putMappingsRec.Code, putMappingsRec.Body.String())
	}

	putForce := httptest.NewRequest(http.MethodPut, "/v0/management/ampcode/force-model-mappings", bytes.NewBufferString(`{"value":true}`))
	putForce.Host = "localhost"
	putForceRec := httptest.NewRecorder()
	srv.ServeHTTP(putForceRec, putForce)
	if putForceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 put force model mappings, got %d body=%s", putForceRec.Code, putForceRec.Body.String())
	}

	getRoot := httptest.NewRequest(http.MethodGet, "/v0/management/ampcode", nil)
	getRoot.Host = "localhost"
	getRootRec := httptest.NewRecorder()
	srv.ServeHTTP(getRootRec, getRoot)
	if getRootRec.Code != http.StatusOK {
		t.Fatalf("expected 200 get ampcode, got %d body=%s", getRootRec.Code, getRootRec.Body.String())
	}
	var payload struct {
		AmpCode struct {
			UpstreamURL        string `json:"upstream-url"`
			UpstreamAPIKey     string `json:"upstream-api-key"`
			ForceModelMappings bool   `json:"force-model-mappings"`
			UpstreamAPIKeys    []struct {
				UpstreamAPIKey string   `json:"upstream-api-key"`
				APIKeys        []string `json:"api-keys"`
			} `json:"upstream-api-keys"`
			ModelMappings []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"model-mappings"`
		} `json:"ampcode"`
	}
	if err := json.Unmarshal(getRootRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode ampcode payload: %v", err)
	}
	if payload.AmpCode.UpstreamURL != "https://amp.example.com" || payload.AmpCode.UpstreamAPIKey != "amp-secret" || !payload.AmpCode.ForceModelMappings {
		t.Fatalf("unexpected ampcode root: %#v", payload.AmpCode)
	}
	if len(payload.AmpCode.UpstreamAPIKeys) != 1 || payload.AmpCode.UpstreamAPIKeys[0].UpstreamAPIKey != "upstream" || len(payload.AmpCode.UpstreamAPIKeys[0].APIKeys) != 2 {
		t.Fatalf("unexpected upstream api keys: %#v", payload.AmpCode.UpstreamAPIKeys)
	}
	if len(payload.AmpCode.ModelMappings) != 1 || payload.AmpCode.ModelMappings[0].From != "gpt-4" || payload.AmpCode.ModelMappings[0].To != "claude-sonnet" {
		t.Fatalf("unexpected model mappings: %#v", payload.AmpCode.ModelMappings)
	}
}

func TestV0ManagementDebugRequestLogAndUsageQueueCompat(t *testing.T) {
	srv := newTestServer(t)

	putDebug := httptest.NewRequest(http.MethodPut, "/v0/management/debug", bytes.NewBufferString(`{"value":true}`))
	putDebug.Host = "localhost"
	putDebugRec := httptest.NewRecorder()
	srv.ServeHTTP(putDebugRec, putDebug)
	if putDebugRec.Code != http.StatusOK {
		t.Fatalf("expected 200 put debug, got %d body=%s", putDebugRec.Code, putDebugRec.Body.String())
	}
	if !strings.Contains(putDebugRec.Body.String(), `"debug":true`) {
		t.Fatalf("unexpected debug response: %s", putDebugRec.Body.String())
	}

	putLog := httptest.NewRequest(http.MethodPatch, "/v0/management/request-log", bytes.NewBufferString(`{"value":true}`))
	putLog.Host = "localhost"
	putLogRec := httptest.NewRecorder()
	srv.ServeHTTP(putLogRec, putLog)
	if putLogRec.Code != http.StatusOK {
		t.Fatalf("expected 200 put request-log, got %d body=%s", putLogRec.Code, putLogRec.Body.String())
	}
	if !strings.Contains(putLogRec.Body.String(), `"request-log":true`) {
		t.Fatalf("unexpected request-log response: %s", putLogRec.Body.String())
	}

	queueReq := httptest.NewRequest(http.MethodGet, "/v0/management/usage-queue?count=2", nil)
	queueReq.Host = "localhost"
	queueRec := httptest.NewRecorder()
	srv.ServeHTTP(queueRec, queueReq)
	if queueRec.Code != http.StatusOK {
		t.Fatalf("expected 200 usage-queue, got %d body=%s", queueRec.Code, queueRec.Body.String())
	}
	var payload struct {
		Items []map[string]interface{} `json:"items"`
		Count int                      `json:"count"`
	}
	if err := json.Unmarshal(queueRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode usage-queue: %v", err)
	}
	if payload.Count != 0 || payload.Items == nil {
		t.Fatalf("unexpected usage-queue payload: %#v", payload)
	}
}
