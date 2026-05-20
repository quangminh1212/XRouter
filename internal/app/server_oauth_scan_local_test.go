package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOAuthProviderScanLocal(t *testing.T) {
	srv := newTestServer(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	credDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatalf("mkdir cred dir: %v", err)
	}
	content := `{"access_token":"tok_1234567890abcdef","refresh_token":"ref_abcdef1234567890"}`
	if err := os.WriteFile(filepath.Join(credDir, "auth.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write credential file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/scan-local", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Matches []struct {
			Path           string   `json:"path"`
			Provider       string   `json:"provider"`
			SecretPreviews []string `json:"secretPreviews"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Matches) != 1 {
		t.Fatalf("expected 1 match, got %#v", payload)
	}
	if payload.Matches[0].Provider != "codex" {
		t.Fatalf("unexpected provider: %#v", payload.Matches[0])
	}
	if len(payload.Matches[0].SecretPreviews) == 0 {
		t.Fatalf("expected secret previews, got %#v", payload.Matches[0])
	}
}
