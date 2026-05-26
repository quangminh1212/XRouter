package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestWeightedQuotaAwareRoundRobin(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false, "comboStrategy": "round_robin", "defaultRequestsPerMinute": 1}); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"account": "a"})
	}))
	defer upstreamA.Close()
	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"account": "b"})
	}))
	defer upstreamB.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider:          "openai",
		Name:              "openai-a",
		AccountName:       "team-a",
		AccountWeight:     1,
		RequestsPerMinute: 1,
		AuthType:          "apikey",
		APIKey:            "x",
		IsActive:          true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstreamA.URL,
			"apiType": "openai",
		},
	})
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider:          "openai",
		Name:              "openai-b",
		AccountName:       "team-b",
		AccountWeight:     3,
		RequestsPerMinute: 100,
		AuthType:          "apikey",
		APIKey:            "x",
		IsActive:          true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstreamB.URL,
			"apiType": "openai",
		},
	})

	seen := map[string]int{}
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		seen[payload["account"].(string)]++
	}
	if seen["b"] != 3 || seen["a"] != 1 {
		t.Fatalf("expected weighted quota-aware distribution b=3 a=1, got %#v", seen)
	}
}
