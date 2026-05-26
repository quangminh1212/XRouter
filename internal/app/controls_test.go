package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestProviderConnectionModelAliasControlsForwarding(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"model_seen": body["model"],
		})
	}))
	defer upstream.Close()

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai alias control",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	patchBody := bytes.NewBufferString(`{"modelAliases":{"openai/friendly":"openai/gpt-4o-mini"}}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/providers/"+conn.ID, patchBody)
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch connection failed: %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"openai/friendly","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["model_seen"] != "gpt-4o-mini" {
		t.Fatalf("expected aliased upstream model, got %#v", payload)
	}
}

func TestProviderConnectionExcludedModelsBlocksCandidate(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai excluded control",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": "http://127.0.0.1:1",
			"apiType": "openai",
		},
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	patchBody := bytes.NewBufferString(`{"excludedModels":["openai/gpt-4o-mini"]}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/providers/"+conn.ID, patchBody)
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch connection failed: %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
}
