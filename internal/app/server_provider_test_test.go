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

func TestProviderValidateEndpointSuccess(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}))
	defer upstream.Close()

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai validate",
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

	req := httptest.NewRequest(http.MethodPost, "/api/providers/"+conn.ID+"/validate", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Result struct {
			Healthy bool   `json:"healthy"`
			Model   string `json:"model"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !payload.Result.Healthy || payload.Result.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected validate payload: %#v", payload)
	}
	updated, ok := srv.store.GetConnectionByIDRaw(conn.ID)
	if !ok {
		t.Fatalf("connection not found after validate")
	}
	if updated.TestStatus != "" {
		t.Fatalf("validate should not update test status, got %q", updated.TestStatus)
	}
}

func TestProviderValidateEndpointFail(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "unauthorized"})
	}))
	defer upstream.Close()

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai validate fail",
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

	req := httptest.NewRequest(http.MethodPost, "/api/providers/"+conn.ID+"/validate", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated, ok := srv.store.GetConnectionByIDRaw(conn.ID)
	if !ok {
		t.Fatalf("connection not found after validate")
	}
	if updated.TestStatus != "" {
		t.Fatalf("validate should not update test status, got %q", updated.TestStatus)
	}
}

func TestProviderTestModelsEndpoint(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)
		if model == "bad-model" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "bad model"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}))
	defer upstream.Close()

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai models test",
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

	body, _ := json.Marshal(map[string]interface{}{"models": []string{"good-model", "bad-model"}})
	req := httptest.NewRequest(http.MethodPost, "/api/providers/"+conn.ID+"/test-models", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Summary struct {
			Total  int `json:"total"`
			Passed int `json:"passed"`
			Failed int `json:"failed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Summary.Total != 2 || payload.Summary.Passed != 1 || payload.Summary.Failed != 1 {
		t.Fatalf("unexpected summary: %#v", payload.Summary)
	}
}

func TestProviderModelsEndpointLive(t *testing.T) {
	srv := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []map[string]string{
				{"id": "gpt-test"},
				{"id": "openai/already-prefixed"},
			},
		})
	}))
	defer upstream.Close()

	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai models",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/providers/"+conn.ID+"/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Provider string   `json:"provider"`
		Models   []string `json:"models"`
		Source   string   `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Source != "live" || len(payload.Models) != 2 {
		t.Fatalf("unexpected models payload: %#v", payload)
	}
	if payload.Models[0] != "openai/already-prefixed" || payload.Models[1] != "openai/gpt-test" {
		t.Fatalf("unexpected models: %#v", payload.Models)
	}
}

func TestProviderModelsEndpointFallback(t *testing.T) {
	srv := newTestServer(t)
	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider:     "anthropic",
		Name:         "anthropic models",
		AuthType:     "apikey",
		APIKey:       "x",
		IsActive:     true,
		DefaultModel: "anthropic/custom-default",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/providers/"+conn.ID+"/models", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Models []string `json:"models"`
		Source string   `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Source != "fallback" || len(payload.Models) == 0 {
		t.Fatalf("expected fallback models, got %#v", payload)
	}
}
