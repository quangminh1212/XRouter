package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestManagementProviderNodesCRUD(t *testing.T) {
	srv := newTestServer(t)
	createReq := httptest.NewRequest(http.MethodPost, "/api/management/provider-nodes", bytes.NewBufferString(`{"name":"node-a","provider":"openai","connectionIds":["c1","c2"]}`))
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
		t.Fatalf("missing node id")
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/management/provider-nodes/"+created.ID, bytes.NewBufferString(`{"connectionIds":["c3"]}`))
	patchReq.Host = "localhost"
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/management/provider-nodes/"+created.ID, nil)
	deleteReq.Host = "localhost"
	deleteRec := httptest.NewRecorder()
	srv.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestProviderNodeFiltersUpstreamCandidate(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"provider": "A"})
	}))
	defer upstreamA.Close()
	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"provider": "B"})
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
	node, _ := srv.store.CreateProviderNode(store.ProviderNode{Name: "only-b", Provider: "openai", ConnectionIDs: []string{connB.ID}})
	_ = connA

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"node":"`+node.ID+`","model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["provider"] != "B" {
		t.Fatalf("expected provider B via node filter, got %#v", payload)
	}
}

func TestProviderNodesAliasCRUD(t *testing.T) {
	srv := newTestServer(t)
	createReq := httptest.NewRequest(http.MethodPost, "/api/provider-nodes", bytes.NewBufferString(`{"name":"node-a","provider":"openai"}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("decode created node: id=%q err=%v", created.ID, err)
	}
	getReq := httptest.NewRequest(http.MethodGet, "/api/provider-nodes/"+created.ID, nil)
	getReq.Host = "localhost"
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
}
