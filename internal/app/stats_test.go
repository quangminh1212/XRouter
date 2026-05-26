package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestUsageStatsEndpointAggregatesUsage(t *testing.T) {
	srv := newTestServer(t)
	entries := []store.UsageEntry{
		{Timestamp: "2026-05-20T01:00:00Z", Provider: "openai", Model: "gpt-4o-mini", TotalCost: 0.01, PromptTokens: 10, CompletionTokens: 5},
		{Timestamp: "2026-05-20T02:00:00Z", Provider: "openai", Model: "gpt-4o-mini", TotalCost: 0.02, PromptTokens: 20, CompletionTokens: 10},
		{Timestamp: "2026-05-19T01:00:00Z", Provider: "anthropic", Model: "claude-3-5-sonnet", TotalCost: 0.03, PromptTokens: 30, CompletionTokens: 15},
	}
	for _, entry := range entries {
		if err := srv.store.RecordUsage(entry); err != nil {
			t.Fatalf("record usage: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/usage/stats", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		TotalRequests    int64                         `json:"totalRequests"`
		PromptTokens     int64                         `json:"promptTokens"`
		CompletionTokens int64                         `json:"completionTokens"`
		ByProvider       map[string]store.DailySummary `json:"byProvider"`
		ByModel          map[string]store.DailySummary `json:"byModel"`
		ByDay            map[string]store.DailySummary `json:"byDay"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.TotalRequests != 3 || payload.PromptTokens != 60 || payload.CompletionTokens != 30 {
		t.Fatalf("unexpected totals: %#v", payload)
	}
	if payload.ByProvider["openai"].Requests != 2 || payload.ByModel["gpt-4o-mini"].Requests != 2 {
		t.Fatalf("unexpected provider/model stats: %#v", payload)
	}
	if payload.ByDay["2026-05-20"].Requests != 2 || payload.ByDay["2026-05-19"].Requests != 1 {
		t.Fatalf("unexpected day stats: %#v", payload.ByDay)
	}
}
