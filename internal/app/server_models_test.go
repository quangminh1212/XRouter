package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"xrouter/internal/store"
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

func TestWave1ChatCompletionsProxySuccess(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["model"] != "deepseek-chat" {
			t.Fatalf("unexpected model: %#v", body["model"])
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":      "chatcmpl-wave1",
			"choices": []map[string]interface{}{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer upstream.Close()
	_, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "deepseek", Name: "deepseek smoke", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"deepseek/deepseek-chat","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChatCompletionsCooldownOn429(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer upstream.Close()
	conn, err := srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "deepseek", Name: "deepseek cooldown", AuthType: "apikey", APIKey: "x", IsActive: true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"deepseek/deepseek-chat","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated, ok := srv.store.GetConnectionByIDRaw(conn.ID)
	if !ok {
		t.Fatalf("missing updated connection")
	}
	if updated.RateLimitedUntil == "" || updated.BackoffLevel == 0 || updated.ConsecutiveFailures == 0 || updated.TestStatus == "" {
		t.Fatalf("expected cooldown state, got %#v", updated)
	}
}

func TestLegacyProvidersStillProxyChatCompletions(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	providers := []string{"openai", "anthropic", "openrouter"}
	for _, provider := range providers {
		provider := provider
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id":      "chatcmpl-legacy",
				"choices": []map[string]interface{}{{"message": map[string]string{"role": "assistant", "content": provider}}},
			})
		}))
		defer upstream.Close()
		apiType := "openai"
		model := provider + "/gpt-4o-mini"
		if provider == "anthropic" {
			apiType = "anthropic"
			model = "claude/claude-3-5-sonnet-latest"
		}
		_, err := srv.store.CreateProviderConnection(store.ProviderConnection{
			Provider: provider, Name: provider + " regression", AuthType: "apikey", APIKey: "x", IsActive: true,
			ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": apiType},
		})
		if err != nil {
			t.Fatalf("create %s connection: %v", provider, err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"`+model+`","messages":[{"role":"user","content":"hello"}]}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d body=%s", provider, rec.Code, rec.Body.String())
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
