package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAudioVoicesList(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/audio/voices", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Voices []map[string]string `json:"voices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Voices) == 0 {
		t.Fatalf("expected non-empty voices list")
	}
	if payload.Voices[0]["provider"] == "" || payload.Voices[0]["id"] == "" {
		t.Fatalf("unexpected payload: %#v", payload.Voices[0])
	}
}
