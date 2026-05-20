package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestProviderCatalogEndpoint(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/catalog?apiType=openai&authType=apikey", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Count     int                          `json:"count"`
		Providers []store.ProviderCatalogEntry `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count == 0 || len(payload.Providers) == 0 {
		t.Fatalf("expected providers in catalog: %#v", payload)
	}
	foundNvidia := false
	for _, provider := range payload.Providers {
		if provider.APIType != "openai" || provider.AuthType != "apikey" {
			t.Fatalf("unexpected filtered provider: %#v", provider)
		}
		if provider.Provider == "nvidia" && provider.BaseURL == "https://integrate.api.nvidia.com/v1" {
			foundNvidia = true
		}
	}
	if !foundNvidia {
		t.Fatalf("expected nvidia in catalog: %#v", payload.Providers)
	}
}
