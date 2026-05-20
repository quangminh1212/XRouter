package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestVideoEditsExtensionsAndRetrieveProxy(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	seen := map[string]bool{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = true
		switch r.URL.Path {
		case "/v1/videos/edits":
			writeJSON(w, http.StatusOK, map[string]interface{}{"id": "vid_edit_1", "status": "queued"})
		case "/v1/videos/extensions":
			writeJSON(w, http.StatusOK, map[string]interface{}{"id": "vid_extend_1", "status": "queued"})
		case "/v1/videos/vid_1":
			writeJSON(w, http.StatusOK, map[string]interface{}{"id": "vid_1", "status": "completed"})
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai",
		Name:     "openai video extras",
		AuthType: "apikey",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})

	for _, path := range []string{"/v1/videos/edits", "/v1/videos/extensions"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"model":"sora-mini","video":"vid_1"}`))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/videos/vid_1", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 retrieve, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode retrieve: %v", err)
	}
	if payload["status"] != "completed" {
		t.Fatalf("unexpected retrieve payload: %#v", payload)
	}
	for _, path := range []string{"/v1/videos/edits", "/v1/videos/extensions", "/v1/videos/vid_1"} {
		if !seen[path] {
			t.Fatalf("upstream path not seen: %s", path)
		}
	}
}
