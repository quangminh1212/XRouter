package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementA2AAgentsCRUDAndPublicList(t *testing.T) {
	srv := newTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/management/a2a-agents", bytes.NewBufferString(`{"name":"Planner Agent","url":"https://a2a.example.com","protocol":"jsonrpc","capabilities":["chat","tools","chat"],"enabled":true,"headers":{"Authorization":"Bearer secret"}}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID           string                 `json:"id"`
		Capabilities []string               `json:"capabilities"`
		Headers      map[string]interface{} `json:"headers"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("missing a2a agent id")
	}
	if created.Headers != nil {
		t.Fatalf("headers should be sanitized from response")
	}
	if len(created.Capabilities) != 2 {
		t.Fatalf("expected deduped capabilities, got %#v", created.Capabilities)
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/api/a2a/agents", nil)
	publicRec := httptest.NewRecorder()
	srv.ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", publicRec.Code, publicRec.Body.String())
	}
	var publicPayload struct {
		Agents []map[string]interface{} `json:"agents"`
	}
	if err := json.Unmarshal(publicRec.Body.Bytes(), &publicPayload); err != nil {
		t.Fatalf("decode public response: %v", err)
	}
	if len(publicPayload.Agents) != 1 {
		t.Fatalf("expected 1 public a2a agent, got %#v", publicPayload)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/management/a2a-agents/"+created.ID, bytes.NewBufferString(`{"enabled":false}`))
	patchReq.Host = "localhost"
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	publicReq2 := httptest.NewRequest(http.MethodGet, "/api/a2a/agents", nil)
	publicRec2 := httptest.NewRecorder()
	srv.ServeHTTP(publicRec2, publicReq2)
	var publicPayload2 struct {
		Agents []map[string]interface{} `json:"agents"`
	}
	_ = json.Unmarshal(publicRec2.Body.Bytes(), &publicPayload2)
	if len(publicPayload2.Agents) != 0 {
		t.Fatalf("expected disabled agent hidden from public list, got %#v", publicPayload2)
	}
}

func TestA2AAgentsAliasByIDRoute(t *testing.T) {
	srv := newTestServer(t)
	createReq := httptest.NewRequest(http.MethodPost, "/api/management/a2a-agents", bytes.NewBufferString(`{"name":"Alias Agent","url":"https://a2a.example.com","protocol":"jsonrpc","capabilities":["chat"],"enabled":true}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct { ID string `json:"id"` }
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("decode created agent: id=%q err=%v", created.ID, err)
	}
	getReq := httptest.NewRequest(http.MethodGet, "/api/a2a-agents/"+created.ID, nil)
	getReq.Host = "localhost"
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
}
