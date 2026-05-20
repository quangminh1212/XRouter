package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthFilesMultiAccountFiltering(t *testing.T) {
	srv := newTestServer(t)
	payload1 := `{"name":"claude-a.json","provider":"claude","accountName":"team-a","accountEmail":"a@example.com","contentB64":"` + base64.StdEncoding.EncodeToString([]byte(`{"token":"one"}`)) + `"}`
	payload2 := `{"name":"claude-b.json","provider":"claude","accountName":"team-b","accountEmail":"b@example.com","contentB64":"` + base64.StdEncoding.EncodeToString([]byte(`{"token":"two"}`)) + `"}`
	for _, payload := range []string{payload1, payload2} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth-files", bytes.NewBufferString(payload))
		req.Host = "localhost"
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth-files?provider=claude&account=team-b", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Files []struct {
			AccountName  string `json:"accountName"`
			AccountEmail string `json:"accountEmail"`
			ContentB64   string `json:"contentB64"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Files) != 1 || payload.Files[0].AccountName != "team-b" || payload.Files[0].ContentB64 != "" {
		t.Fatalf("unexpected filtered payload: %#v", payload)
	}
}
