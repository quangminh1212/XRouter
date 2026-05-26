package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestProviderSuggestedModelsOpenRouterFree(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []map[string]interface{}{
			{"id": "paid", "name": "Paid", "context_length": 300000, "pricing": map[string]interface{}{"prompt": "1", "completion": "0"}},
			{"id": "free-small", "name": "Free Small", "context_length": 100000, "pricing": map[string]interface{}{"prompt": "0", "completion": "0"}},
			{"id": "free-big", "name": "Free Big", "context_length": 250000, "pricing": map[string]interface{}{"prompt": "0", "completion": "0"}},
			{"id": "free-bigger", "name": "Free Bigger", "context_length": 400000, "pricing": map[string]interface{}{"prompt": 0, "completion": 0}},
		}})
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/providers/suggested-models?type=openrouter-free&url="+url.QueryEscape(upstream.URL), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("expected 2 models, got %#v", payload.Data)
	}
	if payload.Data[0]["id"] != "free-bigger" || payload.Data[1]["id"] != "free-big" {
		t.Fatalf("unexpected order: %#v", payload.Data)
	}
}

func TestProviderSuggestedModelsOpenCodeFree(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": []map[string]interface{}{
			{"id": "alpha-free"},
			{"id": "beta"},
			{"id": "gamma-free"},
		}})
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/providers/suggested-models?type=opencode-free&url="+url.QueryEscape(upstream.URL), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 2 || payload.Data[0]["id"] != "alpha-free" || payload.Data[1]["id"] != "gamma-free" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestProviderSuggestedModelsValidation(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/suggested-models?type=unknown&url=https://example.com", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
