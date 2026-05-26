package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestUsageLogDetailEndpointReturnsItemByID(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.store.RecordRequestLog(store.RequestLog{
		ID:           "rlog_test_1",
		Path:         "/v1/chat/completions",
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		StatusCode:   http.StatusOK,
		LatencyMs:    123,
		RequestBytes: 10,
	}); err != nil {
		t.Fatalf("record request log: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/usage/logs/rlog_test_1", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload store.RequestLog
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.ID != "rlog_test_1" || payload.Provider != "openai" || payload.Path != "/v1/chat/completions" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestUsageLogDetailEndpointReturns404(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/usage/logs/missing", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
