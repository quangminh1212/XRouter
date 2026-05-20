package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementTunnelsCRUDAndPublicList(t *testing.T) {
	srv := newTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/management/tunnels", bytes.NewBufferString(`{"name":"Prod Tunnel","provider":"cloudflared","publicUrl":"https://router.example.com","localTarget":"http://127.0.0.1:8080","protocol":"https","enabled":true}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("missing tunnel id")
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/api/tunnels", nil)
	publicRec := httptest.NewRecorder()
	srv.ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", publicRec.Code, publicRec.Body.String())
	}
	var publicPayload struct {
		Tunnels []map[string]interface{} `json:"tunnels"`
	}
	if err := json.Unmarshal(publicRec.Body.Bytes(), &publicPayload); err != nil {
		t.Fatalf("decode public response: %v", err)
	}
	if len(publicPayload.Tunnels) != 1 {
		t.Fatalf("expected 1 public tunnel, got %#v", publicPayload)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/management/tunnels/"+created.ID, bytes.NewBufferString(`{"enabled":false}`))
	patchReq.Host = "localhost"
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	publicReq2 := httptest.NewRequest(http.MethodGet, "/api/tunnels", nil)
	publicRec2 := httptest.NewRecorder()
	srv.ServeHTTP(publicRec2, publicReq2)
	var publicPayload2 struct {
		Tunnels []map[string]interface{} `json:"tunnels"`
	}
	_ = json.Unmarshal(publicRec2.Body.Bytes(), &publicPayload2)
	if len(publicPayload2.Tunnels) != 0 {
		t.Fatalf("expected disabled tunnel hidden from public list, got %#v", publicPayload2)
	}
}
