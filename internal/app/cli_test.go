package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestCLIConfigLocalOnlyAndPayload(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.CreateAPIKey("local", "xr_test_local_123456", 0)
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider:     "deepseek",
		Name:         "DeepSeek A",
		AuthType:     "apikey",
		APIKey:       "sk-deepseek",
		IsActive:     true,
		DefaultModel: "deepseek/deepseek-chat",
	})

	remoteReq := httptest.NewRequest(http.MethodGet, "/api/cli/config", nil)
	remoteReq.Host = "example.com"
	remoteRec := httptest.NewRecorder()
	srv.ServeHTTP(remoteRec, remoteReq)
	if remoteRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-localhost, got %d body=%s", remoteRec.Code, remoteRec.Body.String())
	}

	localReq := httptest.NewRequest(http.MethodGet, "/api/cli/config?tool=openai", nil)
	localReq.Host = "localhost"
	localRec := httptest.NewRecorder()
	srv.ServeHTTP(localRec, localReq)
	if localRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", localRec.Code, localRec.Body.String())
	}
	var payload struct {
		Tool         string            `json:"tool"`
		BaseURL      string            `json:"baseUrl"`
		APIKeyValue  string            `json:"apiKeyValue"`
		DefaultModel string            `json:"defaultModel"`
		Env          map[string]string `json:"env"`
	}
	if err := json.Unmarshal(localRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Tool != "openai" {
		t.Fatalf("unexpected tool: %s", payload.Tool)
	}
	if payload.BaseURL == "" || payload.APIKeyValue == "" || payload.DefaultModel == "" {
		t.Fatalf("missing critical payload fields: %#v", payload)
	}
	if payload.Env["OPENAI_API_KEY"] != "xr_test_local_123456" {
		t.Fatalf("expected raw api key in env for localhost config, got %#v", payload.Env)
	}
}
