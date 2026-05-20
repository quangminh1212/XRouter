package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestCreateWave2ProviderFillsDefaults(t *testing.T) {
	srv := newTestServer(t)
	body := bytes.NewBufferString(`{"provider":"nvidia","name":"NVIDIA test","apiKey":"secret"}`)
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
	if data["baseUrl"] != "https://integrate.api.nvidia.com/v1" {
		t.Fatalf("unexpected baseUrl: %#v", data["baseUrl"])
	}
	if data["apiType"] != "openai" {
		t.Fatalf("unexpected apiType: %#v", data["apiType"])
	}
}

func TestCreateWave3ProviderFillsDefaults(t *testing.T) {
	srv := newTestServer(t)
	body := bytes.NewBufferString(`{"provider":"moonshot","name":"Moonshot test","apiKey":"secret"}`)
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
	if data["baseUrl"] != "https://api.moonshot.ai/v1" {
		t.Fatalf("unexpected baseUrl: %#v", data["baseUrl"])
	}
	if data["apiType"] != "openai" {
		t.Fatalf("unexpected apiType: %#v", data["apiType"])
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
		"deepseek/deepseek-chat":                            false,
		"groq/llama-3.1-70b-versatile":                      false,
		"perplexity/sonar-pro":                              false,
		"nvidia/deepseek-ai/deepseek-v4-flash":              false,
		"huggingface/deepseek-ai/DeepSeek-V3-0324:fastest":  false,
		"minimax/MiniMax-M2.7":                              false,
		"glm/glm-4.7":                                       false,
		"glm-cn/glm-4.7":                                    false,
		"minimax-cn/MiniMax-Text-01":                        false,
		"moonshot/moonshot-v1-8k":                           false,
		"hyperbolic/meta-llama/Meta-Llama-3.1-70B-Instruct": false,
		"novita/deepseek/deepseek-v3":                       false,
		"sambanova/Meta-Llama-3.1-70B-Instruct":             false,
		"chutes/deepseek-ai/DeepSeek-V3-0324":               false,
		"lambda-ai/hermes-3-llama-3.1-405b-fp8":             false,
		"featherless-ai/Qwen/Qwen2.5-Coder-32B-Instruct":    false,
		"kluster/meta-llama/Meta-Llama-3.1-70B-Instruct":    false,
		"reka/reka-core":                                    false,
		"zai/glm-4.7":                                       false,
		"qwen/qwen-plus":                                    false,
		"opencode/gpt-4o-mini":                              false,
		"opencode-go/gpt-4o-mini":                           false,
		"opencode-zen/gpt-4o-mini":                          false,
		"kiro/kiro-pro":                                     false,
		"grok/grok-2-latest":                                false,
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

func TestWave2ChatCompletionsProxySuccess(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		upstream string
	}{
		{provider: "nvidia", model: "nvidia/deepseek-ai/deepseek-v4-flash", upstream: "deepseek-ai/deepseek-v4-flash"},
		{provider: "huggingface", model: "huggingface/deepseek-ai/DeepSeek-V3-0324:fastest", upstream: "deepseek-ai/DeepSeek-V3-0324:fastest"},
		{provider: "minimax", model: "minimax/MiniMax-M2.7", upstream: "MiniMax-M2.7"},
		{provider: "glm", model: "glm/glm-4.7", upstream: "glm-4.7"},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			srv := newTestServer(t)
			_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/chat/completions" {
					t.Fatalf("unexpected upstream path: %s", r.URL.Path)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if body["model"] != tt.upstream {
					t.Fatalf("unexpected model: %#v", body["model"])
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"id":      "chatcmpl-wave2",
					"choices": []map[string]interface{}{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
				})
			}))
			defer upstream.Close()
			_, err := srv.store.CreateProviderConnection(store.ProviderConnection{
				Provider: tt.provider, Name: tt.provider + " smoke", AuthType: "apikey", APIKey: "x", IsActive: true,
				ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
			})
			if err != nil {
				t.Fatalf("create connection: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"`+tt.model+`","messages":[{"role":"user","content":"hello"}]}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestWave3ChatCompletionsProxySuccess(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		upstream string
	}{
		{provider: "glm-cn", model: "glm-cn/glm-4.7", upstream: "glm-4.7"},
		{provider: "minimax-cn", model: "minimax-cn/MiniMax-Text-01", upstream: "MiniMax-Text-01"},
		{provider: "moonshot", model: "moonshot/moonshot-v1-8k", upstream: "moonshot-v1-8k"},
		{provider: "hyperbolic", model: "hyperbolic/meta-llama/Meta-Llama-3.1-70B-Instruct", upstream: "meta-llama/Meta-Llama-3.1-70B-Instruct"},
		{provider: "novita", model: "novita/deepseek/deepseek-v3", upstream: "deepseek/deepseek-v3"},
		{provider: "sambanova", model: "sambanova/Meta-Llama-3.1-70B-Instruct", upstream: "Meta-Llama-3.1-70B-Instruct"},
		{provider: "chutes", model: "chutes/deepseek-ai/DeepSeek-V3-0324", upstream: "deepseek-ai/DeepSeek-V3-0324"},
		{provider: "lambda-ai", model: "lambda-ai/hermes-3-llama-3.1-405b-fp8", upstream: "hermes-3-llama-3.1-405b-fp8"},
		{provider: "featherless-ai", model: "featherless-ai/Qwen/Qwen2.5-Coder-32B-Instruct", upstream: "Qwen/Qwen2.5-Coder-32B-Instruct"},
		{provider: "kluster", model: "kluster/meta-llama/Meta-Llama-3.1-70B-Instruct", upstream: "meta-llama/Meta-Llama-3.1-70B-Instruct"},
		{provider: "reka", model: "reka/reka-core", upstream: "reka-core"},
		{provider: "zai", model: "zai/glm-4.7", upstream: "glm-4.7"},
		{provider: "opencode", model: "opencode/gpt-4o-mini", upstream: "gpt-4o-mini"},
		{provider: "opencode-go", model: "opencode-go/gpt-4o-mini", upstream: "gpt-4o-mini"},
		{provider: "opencode-zen", model: "opencode-zen/gpt-4o-mini", upstream: "gpt-4o-mini"},
		{provider: "kiro", model: "kiro/kiro-pro", upstream: "kiro-pro"},
		{provider: "qwen", model: "qwen/qwen-plus", upstream: "qwen-plus"},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			srv := newTestServer(t)
			_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/chat/completions" {
					t.Fatalf("unexpected upstream path: %s", r.URL.Path)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if body["model"] != tt.upstream {
					t.Fatalf("unexpected model: %#v", body["model"])
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"id":      "chatcmpl-wave3",
					"choices": []map[string]interface{}{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
				})
			}))
			defer upstream.Close()
			_, err := srv.store.CreateProviderConnection(store.ProviderConnection{
				Provider: tt.provider, Name: tt.provider + " smoke", AuthType: "apikey", APIKey: "x", IsActive: true,
				ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"},
			})
			if err != nil {
				t.Fatalf("create connection: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"`+tt.model+`","messages":[{"role":"user","content":"hello"}]}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
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

func TestGenericCompatibleProviderAdapters(t *testing.T) {
	tests := []struct {
		provider     string
		apiType      string
		model        string
		wantPath     string
		wantUpstream string
		response     map[string]interface{}
	}{
		{
			provider: "openai-compatible", apiType: "openai", model: "openai-compatible/gpt-test",
			wantPath:     "/v1/chat/completions",
			wantUpstream: "gpt-test",
			response:     map[string]interface{}{"id": "chatcmpl-openai", "choices": []map[string]interface{}{{"message": map[string]string{"role": "assistant", "content": "openai ok"}}}},
		},
		{
			provider: "anthropic-compatible", apiType: "anthropic", model: "anthropic-compatible/claude-test",
			wantPath:     "/v1/messages",
			wantUpstream: "claude-test",
			response:     map[string]interface{}{"id": "msg-anthropic", "content": []map[string]string{{"type": "text", "text": "anthropic ok"}}},
		},
		{
			provider: "gemini-compatible", apiType: "gemini", model: "gemini-compatible/gemini-test",
			wantPath:     "/v1beta/models/gemini-test:generateContent",
			wantUpstream: "",
			response: map[string]interface{}{"candidates": []map[string]interface{}{{
				"content":      map[string]interface{}{"role": "model", "parts": []map[string]string{{"text": "gemini ok"}}},
				"finishReason": "STOP",
			}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			srv := newTestServer(t)
			_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.wantPath {
					t.Fatalf("unexpected upstream path: %s", r.URL.Path)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if tt.wantUpstream != "" && body["model"] != tt.wantUpstream {
					t.Fatalf("unexpected upstream model: %#v", body["model"])
				}
				if tt.apiType == "gemini" {
					if _, ok := body["contents"].([]interface{}); !ok {
						t.Fatalf("expected gemini contents, got %#v", body)
					}
				}
				writeJSON(w, http.StatusOK, tt.response)
			}))
			defer upstream.Close()
			_, err := srv.store.CreateProviderConnection(store.ProviderConnection{
				Provider: tt.provider, Name: tt.provider + " adapter", AuthType: "apikey", APIKey: "x", IsActive: true,
				ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": tt.apiType},
			})
			if err != nil {
				t.Fatalf("create connection: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"`+tt.model+`","messages":[{"role":"user","content":"hello"}]}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestProviderEndpointRegressionMatrix(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		apiType      string
		path         string
		model        string
		requestBody  string
		wantPath     string
		wantContains string
		responseBody string
		contentType  string
	}{
		{
			name:         "openai chat completions",
			provider:     "openai",
			apiType:      "openai",
			path:         "/v1/chat/completions",
			model:        "openai/gpt-4o-mini",
			requestBody:  `{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
			wantPath:     "/v1/chat/completions",
			wantContains: `"model":"gpt-4o-mini"`,
			responseBody: `{"id":"chatcmpl_1","choices":[{"message":{"role":"assistant","content":"ok"}}]}`,
			contentType:  "application/json",
		},
		{
			name:         "openai completions",
			provider:     "openai",
			apiType:      "openai",
			path:         "/v1/completions",
			model:        "openai/gpt-4o-mini",
			requestBody:  `{"model":"openai/gpt-4o-mini","prompt":"hello","max_tokens":8}`,
			wantPath:     "/v1/completions",
			wantContains: `"model":"gpt-4o-mini"`,
			responseBody: `{"id":"cmpl_1","choices":[{"text":"ok"}]}`,
			contentType:  "application/json",
		},
		{
			name:         "openai responses",
			provider:     "openai",
			apiType:      "responses",
			path:         "/v1/responses",
			model:        "openai/gpt-4o-mini",
			requestBody:  `{"model":"openai/gpt-4o-mini","input":"hello"}`,
			wantPath:     "/responses",
			wantContains: `"model":"gpt-4o-mini"`,
			responseBody: `{"id":"resp_1","object":"response"}`,
			contentType:  "application/json",
		},
		{
			name:         "anthropic messages adapter",
			provider:     "anthropic-compatible",
			apiType:      "anthropic",
			path:         "/v1/chat/completions",
			model:        "anthropic-compatible/claude-test",
			requestBody:  `{"model":"anthropic-compatible/claude-test","messages":[{"role":"user","content":"hello"}]}`,
			wantPath:     "/v1/messages",
			wantContains: `"model":"claude-test"`,
			responseBody: `{"id":"msg_1","content":[{"type":"text","text":"ok"}]}`,
			contentType:  "application/json",
		},
		{
			name:         "gemini compatible chat adapter",
			provider:     "gemini-compatible",
			apiType:      "gemini",
			path:         "/v1/chat/completions",
			model:        "gemini-compatible/gemini-test",
			requestBody:  `{"model":"gemini-compatible/gemini-test","messages":[{"role":"user","content":"hello"}]}`,
			wantPath:     "/v1beta/models/gemini-test:generateContent",
			wantContains: `"contents":[{"parts":[{"text":"hello"}],"role":"user"}]`,
			responseBody: `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`,
			contentType:  "application/json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.wantPath {
					t.Fatalf("unexpected upstream path: %s", r.URL.Path)
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if tt.wantContains != "" && !strings.Contains(string(raw), tt.wantContains) {
					t.Fatalf("unexpected upstream body: %s", string(raw))
				}
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer upstream.Close()
			_, err := srv.store.CreateProviderConnection(store.ProviderConnection{
				Provider: tt.provider, Name: tt.name, AuthType: "apikey", APIKey: "x", IsActive: true,
				ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": tt.apiType},
			})
			if err != nil {
				t.Fatalf("create connection: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
