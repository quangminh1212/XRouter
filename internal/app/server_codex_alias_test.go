package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestCodexResponsesAliasProxy(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantPath    string
		responseID  string
		requestBody string
	}{
		{
			name:        "standard alias",
			path:        "/backend-api/codex/responses",
			wantPath:    "/responses",
			responseID:  "codex_resp_1",
			requestBody: `{"model":"codex/gpt-4o-mini","input":"hello"}`,
		},
		{
			name:        "compact passthrough alias",
			path:        "/backend-api/codex/responses/compact",
			wantPath:    "/responses/compact",
			responseID:  "codex_resp_compact_1",
			requestBody: `{"model":"codex/gpt-4o-mini","input":"hello"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
				t.Fatalf("disable api key auth: %v", err)
			}

			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.wantPath {
					t.Fatalf("unexpected upstream path: %s", r.URL.Path)
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{"id": tt.responseID, "object": "response"})
			}))
			defer upstream.Close()

			_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
				Provider: "codex",
				Name:     "codex responses",
				AuthType: "oauth",
				APIKey:   "x",
				IsActive: true,
				ProviderSpecificData: map[string]interface{}{
					"baseUrl": upstream.URL,
					"apiType": "responses",
				},
				DefaultModel: "codex/gpt-4o-mini",
			})

			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.requestBody))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload["id"] != tt.responseID {
				t.Fatalf("unexpected payload: %#v", payload)
			}
		})
	}
}
