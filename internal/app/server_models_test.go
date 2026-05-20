package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv
}

func TestCreateWave1ProviderFillsDefaults(t *testing.T) {
	srv := newTestServer(t)
	body := bytes.NewBufferString(`{"provider":"deepseek","name":"DeepSeek test","apiKey":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/providers", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := got["providerSpecificData"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected providerSpecificData in response: %#v", got)
	}
	if data["baseUrl"] != "https://api.deepseek.com" {
		t.Fatalf("unexpected baseUrl: %#v", data["baseUrl"])
	}
	if data["apiType"] != "openai" {
		t.Fatalf("unexpected apiType: %#v", data["apiType"])
	}
	if got["apiKey"] != nil && got["apiKey"] != "" {
		t.Fatalf("apiKey must be masked from response")
	}
}

func TestModelsIncludeWave1Fallbacks(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Models []map[string]string `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	wanted := map[string]bool{
		"deepseek/deepseek-chat":       false,
		"groq/llama-3.1-70b-versatile": false,
		"perplexity/sonar-pro":         false,
	}
	for _, model := range got.Models {
		if _, ok := wanted[model["fullModel"]]; ok {
			wanted[model["fullModel"]] = true
		}
	}
	for model, found := range wanted {
		if !found {
			t.Fatalf("missing fallback model %s in %#v", model, got.Models)
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
