package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xrouter/internal/store"
)

func TestUsageLogsEndpointRecordsProxyRequest(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	if err != nil {
		t.Fatalf("failed to disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-XRouter-Provider", "openai")
		w.Header().Set("X-XRouter-Model", "openai/gpt-4o-mini")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id": "resp_1",
			"usage": map[string]interface{}{
				"prompt_tokens":     12,
				"completion_tokens": 8,
			},
		})
	}))
	defer upstream.Close()

	_, err = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai logs",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
		DefaultModel: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	body := bytes.NewBufferString(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	logReq := httptest.NewRequest(http.MethodGet, "/api/usage/logs?limit=1", nil)
	logRec := httptest.NewRecorder()
	srv.ServeHTTP(logRec, logReq)
	if logRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", logRec.Code, logRec.Body.String())
	}

	var payload struct {
		Count int                `json:"count"`
		Items []store.RequestLog `json:"items"`
	}
	if err := json.Unmarshal(logRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode logs payload: %v", err)
	}
	if payload.Count != 1 || len(payload.Items) != 1 {
		t.Fatalf("unexpected logs payload: %#v", payload)
	}
	item := payload.Items[0]
	if item.Path != "/v1/chat/completions" || item.Provider != "openai" {
		t.Fatalf("unexpected request log item: %#v", item)
	}
	if item.Model == "" || item.StatusCode != http.StatusOK || item.RequestBytes == 0 || item.ResponseBytes == 0 {
		t.Fatalf("unexpected request log metrics: %#v", item)
	}
}

func TestUsageStreamSnapshot(t *testing.T) {
	srv := newTestServer(t)
	if err := srv.store.RecordRequestLog(store.RequestLog{
		ID:         "rlog_stream_1",
		Path:       "/v1/chat/completions",
		Provider:   "openai",
		Model:      "openai/gpt-4o-mini",
		StatusCode: http.StatusOK,
	}); err != nil {
		t.Fatalf("record request log: %v", err)
	}
	if err := srv.store.RecordUsage(store.UsageEntry{
		Provider:         "openai",
		Model:            "gpt-4o-mini",
		PromptTokens:     10,
		CompletionTokens: 5,
	}); err != nil {
		t.Fatalf("record usage: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/usage/stream?once=1&limit=1", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("unexpected content type: %s", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: snapshot") || !strings.Contains(body, "rlog_stream_1") || !strings.Contains(body, "totalRequests") {
		t.Fatalf("unexpected stream body: %s", body)
	}
}

func TestDashboardRendersUsageUI(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Host = "localhost:1213"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("unexpected content type: %s", got)
	}
	body := rec.Body.String()
	for _, want := range []string{"XRouter Dashboard", "/api/usage/stats", "/api/usage/logs?limit=50", "/api/usage/stream?limit=50"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}
