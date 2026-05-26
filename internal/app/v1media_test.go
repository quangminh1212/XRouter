package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestAPIV1CompatEmbeddingsRoute(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []map[string]interface{}{{"embedding": []float64{1, 2, 3}}}})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "openai api v1 embeddings", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embeddings", bytes.NewBufferString(`{"model":"text-embedding-3-small","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIV1CompatImagesGenerationsRoute(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"created": 1, "data": []map[string]string{{"url": "https://img.example/out.png"}}})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "openai api v1 images", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/generations", bytes.NewBufferString(`{"model":"gpt-image-1","prompt":"a router"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0]["url"] == "" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestAPIV1CompatAudioSpeechRoute(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("audio"))
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai-tts", Name: "openai api v1 tts", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "tts"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audio/speech", bytes.NewBufferString(`{"model":"gpt-4o-mini-tts","input":"hello","voice":"alloy"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "audio" {
		t.Fatalf("unexpected speech response: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIV1CompatAudioTranscriptionsRoute(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]string{"text": "hello"})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "openai api v1 stt", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "stt"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audio/transcriptions", bytes.NewBufferString("--x\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nwhisper-1\r\n--x--\r\n"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
