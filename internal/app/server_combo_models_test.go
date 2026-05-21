package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestManagementComboModelsCRUDAndModelsExposure(t *testing.T) {
	srv := newTestServer(t)
	createReq := httptest.NewRequest(http.MethodPost, "/api/management/combo-models", bytes.NewBufferString(`{"alias":"combo/coder","targets":["openai/gpt-4o-mini","deepseek/deepseek-chat"]}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	modelsReq := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	modelsRec := httptest.NewRecorder()
	srv.ServeHTTP(modelsRec, modelsReq)
	if modelsRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", modelsRec.Code, modelsRec.Body.String())
	}
	var modelsPayload struct {
		Models []map[string]string `json:"models"`
	}
	_ = json.Unmarshal(modelsRec.Body.Bytes(), &modelsPayload)
	found := false
	for _, item := range modelsPayload.Models {
		if item["fullModel"] == "combo/coder" && item["availability"] == "combo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("combo model missing from /api/models: %#v", modelsPayload.Models)
	}
}

func TestComboModelFallbacksToNextTarget(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadGateway)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"provider": "good"})
	}))
	defer good.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "openai bad", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": bad.URL, "apiType": "openai"},
	})
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "deepseek", Name: "deepseek good", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": good.URL, "apiType": "openai"},
	})
	_, _ = srv.store.CreateComboModel(store.ComboModel{Alias: "combo/coder", Targets: []string{"openai/gpt-4o-mini", "deepseek/deepseek-chat"}})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"combo/coder","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["provider"] != "good" {
		t.Fatalf("expected fallback to next combo target, got %#v", payload)
	}
}

func TestCombosAliasByAliasRoute(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.CreateComboModel(store.ComboModel{Alias: "combo/coder", Targets: []string{"openai/gpt-4o-mini"}})
	getReq := httptest.NewRequest(http.MethodGet, "/api/combos/combo%2Fcoder", nil)
	getReq.Host = "localhost"
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
}
