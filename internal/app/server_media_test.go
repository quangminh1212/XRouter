package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddingsWithoutProvidersFailsCleanly(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	if err != nil {
		t.Fatalf("failed to disable api key auth: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{"model": "cohere/embed-english-v3.0", "input": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAudioSpeechWithoutProvidersFailsCleanly(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	if err != nil {
		t.Fatalf("failed to disable api key auth: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{"model": "openai-tts/gpt-4o-mini-tts", "input": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
}
