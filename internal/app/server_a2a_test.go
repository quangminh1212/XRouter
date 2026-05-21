package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
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
	var created struct {
		ID string `json:"id"`
	}
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

func TestA2ACanonicalJSONRPCListAgents(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.CreateA2AAgent(store.A2AAgent{Name: "Planner", URL: "https://a2a.example.com", Protocol: "jsonrpc", Capabilities: []string{"chat"}, Enabled: true})
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewBufferString(`{"jsonrpc":"2.0","id":"req-1","method":"agents/list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		JSONRPC string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  struct {
			Agents []store.A2AAgent `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.JSONRPC != "2.0" || payload.ID != "req-1" || len(payload.Result.Agents) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestA2ACanonicalJSONRPCMessageSend(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"message/send","params":{"message":{"role":"user","parts":[{"text":"hi"}]}}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, _ := payload["result"].(map[string]interface{})
	if result["status"] != "accepted" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestACPAgentsCRUDByQueryID(t *testing.T) {
	srv := newTestServer(t)
	createReq := httptest.NewRequest(http.MethodPost, "/api/acp/agents", bytes.NewBufferString(`{"name":"ACP Agent","url":"https://a2a.example.com","protocol":"jsonrpc","capabilities":["chat"],"enabled":true}`))
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
		t.Fatalf("decode created agent: id=%q err=%v", created.ID, err)
	}
	getReq := httptest.NewRequest(http.MethodGet, "/api/acp/agents", nil)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 list, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	patchReq := httptest.NewRequest(http.MethodPut, "/api/acp/agents?id="+created.ID, bytes.NewBufferString(`{"enabled":false}`))
	patchReq.Host = "localhost"
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 update, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/acp/agents?id="+created.ID, nil)
	deleteReq.Host = "localhost"
	deleteRec := httptest.NewRecorder()
	srv.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}
