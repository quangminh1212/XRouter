package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestRoutePolicyProviderAndAccountAssignment(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"provider": "a"})
	}))
	defer upstreamA.Close()
	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"provider": "b"})
	}))
	defer upstreamB.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider:    "openai",
		Name:        "openai-a",
		AccountName: "team-a",
		AuthType:    "apikey",
		APIKey:      "x",
		IsActive:    true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstreamA.URL,
			"apiType": "openai",
		},
	})
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider:    "deepseek",
		Name:        "deepseek-b",
		AccountName: "team-b",
		AuthType:    "apikey",
		APIKey:      "x",
		IsActive:    true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstreamB.URL,
			"apiType": "openai",
		},
	})
	_, _ = srv.store.CreateRoutePolicy(store.RoutePolicy{
		Name:        "team-b policy",
		ModelPrefix: "smart/",
		Providers:   []string{"deepseek"},
		Accounts:    []string{"team-b"},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"smart/coder","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["provider"] != "b" {
		t.Fatalf("expected provider b due to provider/account policy, got %#v", payload)
	}
}
