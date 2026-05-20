package app

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
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

func TestEmbeddingsProxySuccess(t *testing.T) {
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
		Provider: "openai", Name: "openai embeddings", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBufferString(`{"model":"text-embedding-3-small","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAudioSpeechProxySuccess(t *testing.T) {
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
		Provider: "openai-tts", Name: "openai tts", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "tts"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(`{"model":"gpt-4o-mini-tts","input":"hello","voice":"alloy"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "audio" {
		t.Fatalf("unexpected audio body: %q", rec.Body.String())
	}
}

func TestAudioTranscriptionsProxySuccess(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") == "" {
			t.Fatalf("missing content type")
		}
		writeJSON(w, http.StatusOK, map[string]string{"text": "hello"})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "openai", Name: "openai stt", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "stt"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewBufferString("--x\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nwhisper-1\r\n--x--\r\n"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssemblyAITranscriptionsProxySuccess(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/upload":
			writeJSON(w, http.StatusOK, map[string]string{"upload_url": "https://upload.example/audio.wav"})
		case "/v2/transcript":
			writeJSON(w, http.StatusOK, map[string]string{"id": "tr-123"})
		case "/v2/transcript/tr-123":
			writeJSON(w, http.StatusOK, map[string]string{"status": "completed", "text": "hello world"})
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "assemblyai", Name: "assemblyai stt", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL + "/v2/transcript", "apiType": "stt"},
	})
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "assemblyai/universal-3-pro")
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("audio-bytes"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["text"] != "hello world" {
		t.Fatalf("unexpected transcription payload: %#v", payload)
	}
}
