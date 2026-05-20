package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebFetchSuccessAndBlockLocalhost(t *testing.T) {
	srv := newTestServer(t)
	t.Setenv("XR_ALLOW_PRIVATE_FETCH", "1")
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello fetch"))
	}))
	defer upstream.Close()

	okReq := httptest.NewRequest(http.MethodPost, "/v1/web/fetch", bytes.NewBufferString(`{"url":"`+upstream.URL+`"}`))
	okRec := httptest.NewRecorder()
	srv.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", okRec.Code, okRec.Body.String())
	}
	var okPayload map[string]interface{}
	if err := json.Unmarshal(okRec.Body.Bytes(), &okPayload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if okPayload["status"] != float64(200) {
		t.Fatalf("unexpected fetch status payload: %#v", okPayload)
	}
	t.Setenv("XR_ALLOW_PRIVATE_FETCH", "")

	blockReq := httptest.NewRequest(http.MethodPost, "/v1/web/fetch", bytes.NewBufferString(`{"url":"http://localhost:8080/"}`))
	blockRec := httptest.NewRecorder()
	srv.ServeHTTP(blockRec, blockReq)
	if blockRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blocked host, got %d body=%s", blockRec.Code, blockRec.Body.String())
	}
}
