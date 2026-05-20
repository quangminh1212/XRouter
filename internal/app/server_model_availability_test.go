package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementModelAvailabilityCRUD(t *testing.T) {
	srv := newTestServer(t)

	putReq := httptest.NewRequest(http.MethodPut, "/api/management/model-availability", bytes.NewBufferString(`{"availability":{"deepseek/deepseek-chat":"available","claude/claude-3-5-sonnet-latest":"unavailable"}}`))
	putReq.Host = "localhost"
	putRec := httptest.NewRecorder()
	srv.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on put, got %d body=%s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/management/model-availability", nil)
	getReq.Host = "localhost"
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getPayload struct {
		Availability map[string]string `json:"availability"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("decode get payload: %v", err)
	}
	if getPayload.Availability["deepseek/deepseek-chat"] != "available" {
		t.Fatalf("unexpected availability payload: %#v", getPayload)
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
		if item["fullModel"] == "deepseek/deepseek-chat" && item["availability"] == "available" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("availability not reflected in /api/models: %#v", modelsPayload.Models)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/management/model-availability", bytes.NewBufferString(`{"models":["deepseek/deepseek-chat"]}`))
	deleteReq.Host = "localhost"
	deleteRec := httptest.NewRecorder()
	srv.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}
