package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestManagementRoutePoliciesCRUD(t *testing.T) {
	srv := newTestServer(t)
	createReq := httptest.NewRequest(http.MethodPost, "/api/management/route-policies", bytes.NewBufferString(`{"name":"force-openai","modelPrefix":"smart/","forceModel":"openai/gpt-4o-mini"}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)
	if created.ID == "" {
		t.Fatalf("missing route policy id")
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/management/route-policies/"+created.ID, bytes.NewBufferString(`{"forceModel":"openai/gpt-4.1-mini"}`))
	patchReq.Host = "localhost"
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/management/route-policies/"+created.ID, nil)
	deleteReq.Host = "localhost"
	deleteRec := httptest.NewRecorder()
	srv.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestRoutePolicyForcesModelAndPool(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}
	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadGateway)
	}))
	defer upstreamA.Close()
	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, http.StatusOK, map[string]interface{}{"provider": "B", "model": body["model"]})
	}))
	defer upstreamB.Close()
	connA, _ := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "A", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstreamA.URL, "apiType": "openai"},
	})
	connB, _ := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "B", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstreamB.URL, "apiType": "openai"},
	})
	pool, _ := srv.store.CreateProxyPool(store.ProxyPool{Name: "only-b", ConnectionIDs: []string{connB.ID}})
	_ = connA
	_, _ = srv.store.CreateRoutePolicy(store.RoutePolicy{
		Name:         "smart route",
		ModelPrefix:  "smart/",
		TargetPoolID: pool.ID,
		ForceModel:   "openai/gpt-4o-mini",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"smart/coder","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["provider"] != "B" || payload["model"] != "gpt-4o-mini" {
		t.Fatalf("unexpected policy routing payload: %#v", payload)
	}
}
