package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestProviderMetricsEndpoint(t *testing.T) {
	srv := newTestServer(t)
	_ = srv.store.RecordRequestLog(store.RequestLog{Provider: "openai", StatusCode: 200, LatencyMs: 100, ResponseBytes: 120})
	_ = srv.store.RecordRequestLog(store.RequestLog{Provider: "openai", StatusCode: 502, LatencyMs: 300, ResponseBytes: 20})
	_ = srv.store.RecordRequestLog(store.RequestLog{Provider: "groq", StatusCode: 200, LatencyMs: 80, ResponseBytes: 60})

	req := httptest.NewRequest(http.MethodGet, "/api/providers/metrics", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Count   int `json:"count"`
		Metrics []struct {
			Provider      string  `json:"provider"`
			Requests      int64   `json:"requests"`
			Failures      int64   `json:"failures"`
			SuccessRate   float64 `json:"successRate"`
			LatencyMsAvg  int64   `json:"latencyMsAvg"`
			ResponseBytes int64   `json:"responseBytes"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 2 || len(payload.Metrics) != 2 {
		t.Fatalf("unexpected metrics count: %#v", payload)
	}
	if payload.Metrics[0].Provider != "openai" || payload.Metrics[0].Requests != 2 || payload.Metrics[0].Failures != 1 {
		t.Fatalf("unexpected openai metrics: %#v", payload.Metrics[0])
	}
	if payload.Metrics[0].LatencyMsAvg != 200 || payload.Metrics[0].ResponseBytes != 140 {
		t.Fatalf("unexpected openai aggregates: %#v", payload.Metrics[0])
	}
	if payload.Metrics[0].SuccessRate != 0.5 {
		t.Fatalf("unexpected openai success rate: %#v", payload.Metrics[0])
	}
}
