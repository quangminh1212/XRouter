package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestUsageHistoryEndpointReturnsRecentUsage(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.store.RecordUsage(store.UsageEntry{Provider: "openai", Model: "gpt-4o-mini", PromptTokens: 10, CompletionTokens: 5}); err != nil {
		t.Fatalf("record openai usage: %v", err)
	}
	if err := srv.store.RecordUsage(store.UsageEntry{Provider: "anthropic", Model: "claude-3-5-sonnet", PromptTokens: 20, CompletionTokens: 10}); err != nil {
		t.Fatalf("record anthropic usage: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/usage/history?limit=1", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Count int                `json:"count"`
		Items []store.UsageEntry `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Count != 1 || len(payload.Items) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.Items[0].Provider != "anthropic" {
		t.Fatalf("expected newest usage first, got %#v", payload.Items[0])
	}
}

func TestUsageHistoryEndpointFiltersProvider(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.store.RecordUsage(store.UsageEntry{Provider: "openai", Model: "gpt-4o-mini", PromptTokens: 10}); err != nil {
		t.Fatalf("record openai usage: %v", err)
	}
	if err := srv.store.RecordUsage(store.UsageEntry{Provider: "anthropic", Model: "claude-3-5-sonnet", PromptTokens: 20}); err != nil {
		t.Fatalf("record anthropic usage: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/usage/history?provider=openai", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Count int                `json:"count"`
		Items []store.UsageEntry `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Count != 1 || payload.Items[0].Provider != "openai" {
		t.Fatalf("unexpected filtered payload: %#v", payload)
	}
}
