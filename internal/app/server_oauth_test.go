package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
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

func TestRefreshOAuthProviderToken(t *testing.T) {
	srv := newTestServer(t)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected post to token server")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh-abc" {
			t.Fatalf("unexpected refresh form: %v", r.Form)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"access_token":  "access-new",
			"refresh_token": "refresh-new",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	created, err := srv.store.CreateProviderConnection(testOAuthConnection("codex", "refresh-abc"))
	if err != nil {
		t.Fatalf("create oauth connection: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{"tokenUrl": tokenServer.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/"+created.ID+"/refresh", bytes.NewReader(body))
	req.Host = "localhost:1213"
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated, ok := srv.store.GetConnectionByIDRaw(created.ID)
	if !ok {
		t.Fatalf("missing updated connection")
	}
	if updated.AccessToken != "access-new" || updated.RefreshToken != "refresh-new" || updated.TokenExpiry == "" {
		t.Fatalf("tokens not updated: %#v", updated)
	}
}

func testOAuthConnection(provider, refreshToken string) store.ProviderConnection {
	return store.ProviderConnection{
		Provider:     provider,
		Name:         provider + " test",
		AuthType:     "oauth",
		RefreshToken: refreshToken,
		IsActive:     true,
	}
}
