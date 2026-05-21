package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIKeysRejectsOversizedBody(t *testing.T) {
	srv := newTestServer(t)
	body := `{"name":"x","key":"sk-test","requestsPerMinute":60,"padding":"` + strings.Repeat("a", 1024*1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/keys", bytes.NewBufferString(body))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d body=%s", rec.Code, rec.Body.String())
	}
}
