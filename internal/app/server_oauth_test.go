package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListOAuthProviders(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/providers", nil)
	req.Host = "localhost:1213"
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := out["providers"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("expected oauth providers list in response")
	}
}

func TestImportOAuthProviderToken(t *testing.T) {
	srv := newTestServer(t)
	body, _ := json.Marshal(map[string]interface{}{
		"name":        "Codex OAuth",
		"accessToken": "token-abc",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/codex/import", bytes.NewReader(body))
	req.Host = "localhost:1213"
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	connections := srv.store.GetAllConnectionsRaw()
	found := false
	for _, c := range connections {
		if c.Provider == "codex" && c.AuthType == "oauth" {
			found = true
			if c.AccessToken == "" {
				t.Fatalf("expected access token stored")
			}
		}
	}
	if !found {
		t.Fatalf("oauth connection not stored")
	}
}
