package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementDisabledModelsFiltersModels(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPut, "/api/management/disabled-models", bytes.NewBufferString(`{"models":["deepseek/deepseek-chat"]}`))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	modelsReq := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	modelsRec := httptest.NewRecorder()
	srv.ServeHTTP(modelsRec, modelsReq)
	if modelsRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", modelsRec.Code, modelsRec.Body.String())
	}
	var payload struct {
		Models []map[string]string `json:"models"`
	}
	if err := json.Unmarshal(modelsRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode models payload: %v", err)
	}
	for _, item := range payload.Models {
		if item["fullModel"] == "deepseek/deepseek-chat" {
			t.Fatalf("disabled model should be filtered from /api/models")
		}
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/management/disabled-models", bytes.NewBufferString(`{"models":["deepseek/deepseek-chat"]}`))
	deleteReq.Host = "localhost"
	deleteRec := httptest.NewRecorder()
	srv.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestDisabledModelBlocksProxyRequest(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("failed to disable api key auth: %v", err)
	}
	if _, err := srv.store.ReplaceDisabledModels([]string{"openai/gpt-4o-mini"}); err != nil {
		t.Fatalf("replace disabled models: %v", err)
	}

	body := bytes.NewBufferString(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
