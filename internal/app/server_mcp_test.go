package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementMCPServersCRUDAndPublicList(t *testing.T) {
	srv := newTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/management/mcp-servers", bytes.NewBufferString(`{"name":"Docs MCP","transport":"http","url":"https://mcp.example.com","enabled":true,"headers":{"Authorization":"Bearer secret"}}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID      string                 `json:"id"`
		Headers map[string]interface{} `json:"headers"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("missing mcp server id")
	}
	if created.Headers != nil {
		t.Fatalf("headers should be sanitized from response")
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/api/mcp/servers", nil)
	publicRec := httptest.NewRecorder()
	srv.ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", publicRec.Code, publicRec.Body.String())
	}
	var publicPayload struct {
		Servers []map[string]interface{} `json:"servers"`
	}
	if err := json.Unmarshal(publicRec.Body.Bytes(), &publicPayload); err != nil {
		t.Fatalf("decode public response: %v", err)
	}
	if len(publicPayload.Servers) != 1 {
		t.Fatalf("expected 1 public mcp server, got %#v", publicPayload)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/management/mcp-servers/"+created.ID, bytes.NewBufferString(`{"enabled":false}`))
	patchReq.Host = "localhost"
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	publicReq2 := httptest.NewRequest(http.MethodGet, "/api/mcp/servers", nil)
	publicRec2 := httptest.NewRecorder()
	srv.ServeHTTP(publicRec2, publicReq2)
	var publicPayload2 struct {
		Servers []map[string]interface{} `json:"servers"`
	}
	_ = json.Unmarshal(publicRec2.Body.Bytes(), &publicPayload2)
	if len(publicPayload2.Servers) != 0 {
		t.Fatalf("expected disabled server hidden from public list, got %#v", publicPayload2)
	}
}
