package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestProviderTestEndpointSuccess(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}))
	defer upstream.Close()

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai test",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
		DefaultModel: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/providers/"+conn.ID+"/test", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated, ok := srv.store.GetConnectionByIDRaw(conn.ID)
	if !ok || updated.TestStatus != "active" {
		t.Fatalf("expected active test status, got %#v", updated)
	}
}

func TestProviderTestEndpointFail(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "unauthorized"})
	}))
	defer upstream.Close()

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai test fail",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
		DefaultModel: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/providers/"+conn.ID+"/test", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	updated, ok := srv.store.GetConnectionByIDRaw(conn.ID)
	if !ok || updated.TestStatus != "unavailable" {
		t.Fatalf("expected unavailable test status, got %#v", updated)
	}
}
