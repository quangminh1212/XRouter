package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementModelAliasesCRUD(t *testing.T) {
	srv := newTestServer(t)

	putBody := bytes.NewBufferString(`{"aliases":{"openai/gpt-4o-mini":"gpt-mini","anthropic/claude-3-5-sonnet":"claude-sonnet"}}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/management/model-aliases", putBody)
	putReq.Host = "localhost"
	putRec := httptest.NewRecorder()
	srv.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on put, got %d body=%s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/management/model-aliases", nil)
	getReq.Host = "localhost"
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getPayload struct {
		Aliases map[string]string `json:"aliases"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("decode get payload: %v", err)
	}
	if getPayload.Aliases["openai/gpt-4o-mini"] != "gpt-mini" {
		t.Fatalf("missing alias after put: %#v", getPayload.Aliases)
	}

	patchBody := bytes.NewBufferString(`{"aliases":{"openai/gpt-4o-mini":"gpt-4o-mini-fast"}}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/management/model-aliases", patchBody)
	patchReq.Host = "localhost"
	patchRec := httptest.NewRecorder()
	srv.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	modelsReq := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	modelsRec := httptest.NewRecorder()
	srv.ServeHTTP(modelsRec, modelsReq)
	if modelsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on models, got %d body=%s", modelsRec.Code, modelsRec.Body.String())
	}
	var modelsPayload struct {
		Models []map[string]string `json:"models"`
	}
	if err := json.Unmarshal(modelsRec.Body.Bytes(), &modelsPayload); err != nil {
		t.Fatalf("decode models payload: %v", err)
	}
	found := false
	for _, item := range modelsPayload.Models {
		if item["fullModel"] == "openai/gpt-4o-mini" && item["alias"] == "gpt-4o-mini-fast" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("updated alias not reflected in /api/models: %#v", modelsPayload.Models)
	}

	deleteBody := bytes.NewBufferString(`{"models":["openai/gpt-4o-mini"]}`)
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/management/model-aliases", deleteBody)
	deleteReq.Host = "localhost"
	deleteRec := httptest.NewRecorder()
	srv.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}
