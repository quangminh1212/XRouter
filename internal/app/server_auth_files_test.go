package app

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthFilesUploadListDownloadDelete(t *testing.T) {
	srv := newTestServer(t)
	content := base64.StdEncoding.EncodeToString([]byte("secret-auth-content"))

	createReq := httptest.NewRequest(http.MethodPost, "/api/auth-files", strings.NewReader(`{"name":"token.json","provider":"vertex","contentB64":"`+content+`"}`))
	createReq.Host = "localhost"
	createRec := httptest.NewRecorder()
	srv.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Size int    `json:"size"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Name != "token.json" || created.Size == 0 {
		t.Fatalf("unexpected create payload: %#v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/auth-files", nil)
	listReq.Host = "localhost"
	listRec := httptest.NewRecorder()
	srv.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Files []map[string]interface{} `json:"files"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Files) != 1 {
		t.Fatalf("expected 1 file, got %#v", listed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/auth-files/"+created.ID, nil)
	getReq.Host = "localhost"
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	if getRec.Body.String() != "secret-auth-content" {
		t.Fatalf("unexpected downloaded content: %q", getRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/auth-files/"+created.ID, nil)
	deleteReq.Host = "localhost"
	deleteRec := httptest.NewRecorder()
	srv.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}
