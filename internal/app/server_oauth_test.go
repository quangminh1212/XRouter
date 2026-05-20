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

func TestStartAndExchangeOAuthFlow(t *testing.T) {
	srv := newTestServer(t)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Fatalf("expected authorization_code grant")
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"access_token":  "access-from-exchange",
			"refresh_token": "refresh-from-exchange",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer tokenServer.Close()

	startBody, _ := json.Marshal(map[string]interface{}{
		"clientId":     "demo-client",
		"clientSecret": "demo-secret",
		"authorizeUrl": "https://example.com/oauth/authorize",
		"tokenUrl":     tokenServer.URL,
		"redirectUri":  "http://localhost:1213/api/oauth/callback",
		"scopes":       []string{"openid", "offline_access"},
	})
	startReq := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/codex/start", bytes.NewReader(startBody))
	startReq.Host = "localhost:1213"
	startRec := httptest.NewRecorder()
	srv.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d body=%s", startRec.Code, startRec.Body.String())
	}
	var startResp map[string]interface{}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startResp); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	state := startResp["state"].(string)
	exchangeBody, _ := json.Marshal(map[string]interface{}{
		"state": state,
		"code":  "auth-code-123",
		"name":  "Codex Browser OAuth",
	})
	exchangeReq := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/codex/exchange", bytes.NewReader(exchangeBody))
	exchangeReq.Host = "localhost:1213"
	exchangeRec := httptest.NewRecorder()
	srv.ServeHTTP(exchangeRec, exchangeReq)
	if exchangeRec.Code != http.StatusCreated {
		t.Fatalf("exchange expected 201, got %d body=%s", exchangeRec.Code, exchangeRec.Body.String())
	}
	found := false
	for _, c := range srv.store.GetAllConnectionsRaw() {
		if c.Provider == "codex" && c.Name == "Codex Browser OAuth" {
			found = true
			if c.AccessToken != "access-from-exchange" || c.RefreshToken != "refresh-from-exchange" {
				t.Fatalf("unexpected stored oauth tokens: %#v", c)
			}
		}
	}
	if !found {
		t.Fatalf("oauth exchange did not create provider connection")
	}
}

func TestClaudeOAuthStartUsesCatalogDefaults(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/claude/start", bytes.NewReader([]byte(`{}`)))
	req.Host = "localhost:1213"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	authURL, _ := payload["authorizationUrl"].(string)
	if !strings.Contains(authURL, "console.anthropic.com") || !strings.Contains(authURL, "client_id=claude-cli") {
		t.Fatalf("unexpected authorizationUrl: %s", authURL)
	}
}

func TestGeminiOAuthStartUsesCatalogDefaults(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/gemini/start", bytes.NewReader([]byte(`{}`)))
	req.Host = "localhost:1213"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	authURL, _ := payload["authorizationUrl"].(string)
	if !strings.Contains(authURL, "accounts.google.com") || !strings.Contains(authURL, "client_id=gemini-cli") {
		t.Fatalf("unexpected authorizationUrl: %s", authURL)
	}
}

func TestAntigravityOAuthStartUsesCatalogDefaultsAndEnvClientID(t *testing.T) {
	srv := newTestServer(t)
	t.Setenv("XROUTER_ANTIGRAVITY_OAUTH_CLIENT_ID", "antigravity-test-client")
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/antigravity/start", bytes.NewReader([]byte(`{}`)))
	req.Host = "localhost:1213"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	authURL, _ := payload["authorizationUrl"].(string)
	if !strings.Contains(authURL, "accounts.google.com") || !strings.Contains(authURL, "client_id=antigravity-test-client") {
		t.Fatalf("unexpected authorizationUrl: %s", authURL)
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
