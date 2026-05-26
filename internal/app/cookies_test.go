package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderCookieImportCreatesBrowserSessionConnection(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/providers/cookie-import", bytes.NewBufferString(`{"provider":"claude","name":"Claude Web","cookie":"sessionid=secret","accountName":"web-a","accountEmail":"web@example.com"}`))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		AuthType string `json:"authType"`
		APIKey   string `json:"apiKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.AuthType != "cookie" || response.APIKey != "" {
		t.Fatalf("unexpected sanitized response: %#v", response)
	}
	found := false
	for _, conn := range srv.store.GetAllConnectionsRaw() {
		if conn.Name == "Claude Web" {
			found = true
			if conn.AuthType != "cookie" || conn.APIKey != "sessionid=secret" || conn.AccountName != "web-a" {
				t.Fatalf("unexpected stored connection: %#v", conn)
			}
		}
	}
	if !found {
		t.Fatalf("cookie connection not stored")
	}
}

func TestProviderCookieImportAppliesCatalogDefaultsForWebProvider(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/providers/cookie-import", bytes.NewBufferString(`{"provider":"chatgpt-web","cookie":"session=secret"}`))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Provider             string                 `json:"provider"`
		Name                 string                 `json:"name"`
		DefaultModel         string                 `json:"defaultModel"`
		ProviderSpecificData map[string]interface{} `json:"providerSpecificData"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Provider != "chatgpt-web" || response.Name != "chatgpt-web browser session" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.DefaultModel != "chatgpt-web/gpt-4o-mini" {
		t.Fatalf("unexpected default model: %#v", response.DefaultModel)
	}
	if response.ProviderSpecificData["baseUrl"] != "https://chatgpt.com/backend-api" || response.ProviderSpecificData["apiType"] != "openai" {
		t.Fatalf("unexpected provider defaults: %#v", response.ProviderSpecificData)
	}
}
