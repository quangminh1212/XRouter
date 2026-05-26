package app

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVertexCredentialImportFromAuthFile(t *testing.T) {
	srv := newTestServer(t)
	creds := `{"type":"service_account","project_id":"demo-project"}`
	content := base64.StdEncoding.EncodeToString([]byte(creds))

	createFileReq := httptest.NewRequest(http.MethodPost, "/api/auth-files", strings.NewReader(`{"name":"vertex.json","provider":"vertex","contentB64":"`+content+`"}`))
	createFileReq.Host = "localhost"
	createFileRec := httptest.NewRecorder()
	srv.ServeHTTP(createFileRec, createFileReq)
	if createFileRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createFileRec.Code, createFileRec.Body.String())
	}
	var authFile struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createFileRec.Body.Bytes(), &authFile); err != nil {
		t.Fatalf("decode auth file response: %v", err)
	}

	importReq := httptest.NewRequest(http.MethodPost, "/api/oauth/providers/vertex/import-file", strings.NewReader(`{"authFileId":"`+authFile.ID+`","defaultModel":"vertex/gemini-1.5-flash"}`))
	importReq.Host = "localhost"
	importRec := httptest.NewRecorder()
	srv.ServeHTTP(importRec, importReq)
	if importRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", importRec.Code, importRec.Body.String())
	}
	var payload struct {
		Provider             string                 `json:"provider"`
		AuthType             string                 `json:"authType"`
		DefaultModel         string                 `json:"defaultModel"`
		ProviderSpecificData map[string]interface{} `json:"providerSpecificData"`
	}
	if err := json.Unmarshal(importRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if payload.Provider != "vertex" || payload.AuthType != "oauth" || payload.DefaultModel != "vertex/gemini-1.5-flash" {
		t.Fatalf("unexpected import payload: %#v", payload)
	}
	if payload.ProviderSpecificData["projectId"] != "demo-project" {
		t.Fatalf("expected projectId from auth file, got %#v", payload.ProviderSpecificData)
	}
}
