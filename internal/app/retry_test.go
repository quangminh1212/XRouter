package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManagementRetryConfigEndpointUpdatesSettings(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPatch, "/api/management/retry-config", bytes.NewBufferString(`{"maxRetries":2,"maxCooldownSeconds":30}`))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		MaxRetries         int `json:"maxRetries"`
		MaxCooldownSeconds int `json:"maxCooldownSeconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.MaxRetries != 2 || payload.MaxCooldownSeconds != 30 {
		t.Fatalf("unexpected retry config payload: %#v", payload)
	}
	settings := srv.store.GetSettings()
	if settings.MaxRetries != 2 || settings.MaxCooldownSeconds != 30 {
		t.Fatalf("settings not updated: %#v", settings)
	}
}

func TestManagementRetryConfigRejectsInvalidValues(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/management/retry-config", bytes.NewBufferString(`{"maxRetries":-1}`))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestManagementRetryConfigRejectsOversizedBody(t *testing.T) {
	srv := newTestServer(t)
	body := `{"maxRetries":1,"padding":"` + strings.Repeat("a", 1024*1024) + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/management/retry-config", bytes.NewBufferString(body))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
