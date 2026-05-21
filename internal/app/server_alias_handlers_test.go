package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAliasEndpointsReturnExpectedShape(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		key    string
	}{
		{name: "rate-limits", method: http.MethodGet, path: "/api/rate-limits", key: "rateLimits"},
		{name: "cache-stats", method: http.MethodGet, path: "/api/cache/stats", key: "cacheEnabled"},
		{name: "pricing", method: http.MethodGet, path: "/api/pricing", key: "pricing"},
		{name: "fallback-chains", method: http.MethodGet, path: "/api/fallback/chains", key: "chains"},
		{name: "telemetry-summary", method: http.MethodGet, path: "/api/telemetry/summary", key: "telemetry"},
		{name: "usage-budget", method: http.MethodGet, path: "/api/usage/budget", key: "enabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if _, ok := payload[tt.key]; !ok {
				t.Fatalf("missing key %q in payload: %#v", tt.key, payload)
			}
		})
	}
}

func TestCacheStatsDeleteReturnsSuccess(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/cache/stats", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["success"] != true {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestFallbackChainsPostRejectsInvalidBody(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/fallback/chains", httptest.NewRecorder().Body)
	req.Host = "localhost:1213"
	req.Body = http.NoBody
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d body=%s", rec.Code, rec.Body.String())
	}
}
