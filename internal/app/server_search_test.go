package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchRejectsInvalidBody(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/search", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSearchWithoutProvidersFailsCleanly(t *testing.T) {
	srv := newTestServer(t)
	settings, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	if err != nil || settings.RequireAPIKey {
		t.Fatalf("failed to disable api key auth: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{"query": "openai", "maxResults": 3})
	req := httptest.NewRequest(http.MethodPost, "/v1/search", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
}
