package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"xrouter/internal/store"
)

func TestParseRetryAfterSeconds(t *testing.T) {
	now := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	until, ok := parseRetryAfter("15", now)
	if !ok {
		t.Fatalf("expected retry-after to parse")
	}
	if got := until.Sub(now); got != 15*time.Second {
		t.Fatalf("expected 15s cooldown, got %s", got)
	}
}

func TestParseRateLimitResetHeader(t *testing.T) {
	now := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-ratelimit-reset", "1800000000")

	until, ok := parseRateLimitReset(headers, now)
	if !ok {
		t.Fatalf("expected x-ratelimit-reset to parse")
	}
	if !until.After(now) {
		t.Fatalf("expected future cooldown, got %s", until)
	}
}

func TestFallbackCooldownFor429(t *testing.T) {
	if got := getFallbackCooldown(429); got != 15*time.Second {
		t.Fatalf("expected 15s for 429, got %s", got)
	}
}

func TestResolveEndpointWave1Providers(t *testing.T) {
	tests := map[string]string{
		"deepseek":          "https://api.deepseek.com/v1/chat/completions",
		"groq":              "https://api.groq.com/openai/v1/chat/completions",
		"mistral":           "https://api.mistral.ai/v1/chat/completions",
		"cerebras":          "https://api.cerebras.ai/v1/chat/completions",
		"fireworks":         "https://api.fireworks.ai/inference/v1/chat/completions",
		"together":          "https://api.together.xyz/v1/chat/completions",
		"siliconflow":       "https://api.siliconflow.cn/v1/chat/completions",
		"vercel-ai-gateway": "https://ai-gateway.vercel.sh/v1/chat/completions",
		"cohere":            "https://api.cohere.com/compatibility/v1/chat/completions",
		"perplexity":        "https://api.perplexity.ai/v1/chat/completions",
		"nvidia":            "https://integrate.api.nvidia.com/v1/chat/completions",
		"huggingface":       "https://router.huggingface.co/v1/chat/completions",
		"minimax":           "https://api.minimax.io/v1/chat/completions",
		"glm":               "https://open.bigmodel.cn/api/paas/v4/chat/completions",
	}
	for provider, want := range tests {
		got, mode, err := resolveEndpoint(store.ProviderConnection{Provider: provider}, provider+"/test-model", "/v1/chat/completions")
		if err != nil {
			t.Fatalf("%s endpoint failed: %v", provider, err)
		}
		if got != want || mode != "openai" {
			t.Fatalf("%s expected %s/openai, got %s/%s", provider, want, got, mode)
		}
	}
}

func TestResolveEndpointCustomProviderRequiresBaseURL(t *testing.T) {
	_, _, err := resolveEndpoint(store.ProviderConnection{Provider: "custom"}, "custom/model", "/v1/chat/completions")
	if err == nil {
		t.Fatalf("expected missing baseUrl error")
	}
}

func TestNormalizeOpenAIToGeminiBodySystemInstruction(t *testing.T) {
	body := map[string]interface{}{
		"model": "gemini-compatible/gemini-1.5-flash",
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "follow policy"},
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}
	raw := normalizeOpenAIToGeminiBody(body, "gemini-compatible")
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode transformed body: %v", err)
	}
	if _, ok := out["model"]; ok {
		t.Fatalf("gemini body should not keep model field: %#v", out)
	}
	systemInstruction, ok := out["systemInstruction"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing systemInstruction: %#v", out)
	}
	parts, ok := systemInstruction["parts"].([]interface{})
	if !ok || len(parts) != 1 {
		t.Fatalf("unexpected systemInstruction parts: %#v", systemInstruction)
	}
	part, _ := parts[0].(map[string]interface{})
	if strings.TrimSpace(part["text"].(string)) != "follow policy" {
		t.Fatalf("unexpected systemInstruction text: %#v", part)
	}
	contents, ok := out["contents"].([]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("unexpected contents: %#v", out["contents"])
	}
	content, _ := contents[0].(map[string]interface{})
	if content["role"] != "user" {
		t.Fatalf("unexpected content role: %#v", content)
	}
}

func TestResolveEndpointCloudflareAIUsesAccountID(t *testing.T) {
	got, mode, err := resolveEndpoint(store.ProviderConnection{
		Provider: "cloudflare-ai",
		ProviderSpecificData: map[string]interface{}{
			"accountId": "acc-123",
		},
	}, "cloudflare-ai/test-model", "/v1/chat/completions")
	if err != nil {
		t.Fatalf("cloudflare-ai endpoint failed: %v", err)
	}
	want := "https://api.cloudflare.com/client/v4/accounts/acc-123/ai/v1/chat/completions"
	if got != want || mode != "openai" {
		t.Fatalf("expected %s/openai, got %s/%s", want, got, mode)
	}
}

func TestResolveEndpointAzureUsesDeployment(t *testing.T) {
	got, mode, err := resolveEndpoint(store.ProviderConnection{
		Provider: "azure",
		ProviderSpecificData: map[string]interface{}{
			"azureEndpoint":   "https://example.openai.azure.com",
			"deployment":      "gpt-4o-mini",
			"azureApiVersion": "2024-10-21",
		},
	}, "azure/gpt-4o-mini", "/v1/chat/completions")
	if err != nil {
		t.Fatalf("azure endpoint failed: %v", err)
	}
	want := "https://example.openai.azure.com/openai/deployments/gpt-4o-mini/chat/completions?api-version=2024-10-21"
	if got != want || mode != "openai" {
		t.Fatalf("expected %s/openai, got %s/%s", want, got, mode)
	}
}

func TestSetAuthHeaderClarifaiUsesKeyScheme(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.clarifai.com/v2/ext/openai/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	setAuthHeader(req, store.ProviderConnection{Provider: "clarifai", APIKey: "pat-123"}, "openai")
	if got := req.Header.Get("Authorization"); got != "Key pat-123" {
		t.Fatalf("expected Clarifai Key auth, got %q", got)
	}
}

func TestNormalizeModelForWave1Provider(t *testing.T) {
	body := map[string]interface{}{"model": "deepseek/deepseek-chat"}
	raw := normalizeModelForUpstream(body, "deepseek")
	if string(raw) != `{"model":"deepseek-chat"}` {
		t.Fatalf("unexpected normalized body: %s", raw)
	}
}

func TestNormalizeSearchResultsBrave(t *testing.T) {
	raw := map[string]interface{}{
		"web": map[string]interface{}{
			"results": []interface{}{
				map[string]interface{}{
					"title":       "OpenAI",
					"url":         "https://openai.com",
					"description": "AI research and deployment company",
				},
			},
		},
	}
	out := normalizeSearchResults("brave-search", raw)
	if len(out) != 1 || out[0].Title != "OpenAI" || out[0].URL != "https://openai.com" {
		t.Fatalf("unexpected normalize output: %#v", out)
	}
}

func TestNormalizeSearchResultsSerper(t *testing.T) {
	raw := map[string]interface{}{
		"organic": []interface{}{
			map[string]interface{}{
				"title":   "Serper",
				"link":    "https://serper.dev",
				"snippet": "Google Search API",
			},
		},
	}
	out := normalizeSearchResults("serper", raw)
	if len(out) != 1 || out[0].Title != "Serper" || out[0].URL != "https://serper.dev" {
		t.Fatalf("unexpected normalize output: %#v", out)
	}
}

func TestResolveMediaEndpointEmbeddings(t *testing.T) {
	tests := map[string]string{
		"openai":    "https://api.openai.com/v1/embeddings",
		"cohere":    "https://api.cohere.com/compatibility/v1/embeddings",
		"voyage-ai": "https://api.voyageai.com/v1/embeddings",
		"jina-ai":   "https://api.jina.ai/v1/embeddings",
	}
	for provider, want := range tests {
		got, _, err := resolveMediaEndpoint(store.ProviderConnection{Provider: provider}, "/v1/embeddings", "embedding")
		if err != nil {
			t.Fatalf("%s endpoint failed: %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s expected %s, got %s", provider, want, got)
		}
	}
}

func TestResolveMediaEndpointTTSProviders(t *testing.T) {
	tests := map[string]string{
		"openai-tts": "https://api.openai.com/v1/audio/speech",
		"elevenlabs": "https://api.elevenlabs.io/v1/text-to-speech",
		"cartesia":   "https://api.cartesia.ai/tts/bytes",
		"aws-polly":  "https://polly.us-east-1.amazonaws.com/v1/speech",
	}
	for provider, want := range tests {
		got, _, err := resolveMediaEndpoint(store.ProviderConnection{Provider: provider, ProviderSpecificData: map[string]interface{}{"region": "us-east-1"}}, "/v1/audio/speech", "tts")
		if err != nil {
			t.Fatalf("%s endpoint failed: %v", provider, err)
		}
		if got != want {
			t.Fatalf("%s expected %s, got %s", provider, want, got)
		}
	}
}

func TestSignAWSPollyRequestSetsAuthorization(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://polly.us-east-1.amazonaws.com/v1/speech", bytes.NewReader([]byte(`{"Text":"hello"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	signAWSPollyRequest(req, store.ProviderConnection{
		Provider: "aws-polly",
		APIKey:   "secret-access-key",
		ProviderSpecificData: map[string]interface{}{
			"accessKeyId": "AKIAEXAMPLE",
			"region":      "us-east-1",
		},
	})
	if got := req.Header.Get("Authorization"); !strings.HasPrefix(got, "AWS4-HMAC-SHA256 Credential=AKIAEXAMPLE/") {
		t.Fatalf("expected sigv4 auth header, got %q", got)
	}
	if req.Header.Get("X-Amz-Date") == "" || req.Header.Get("X-Amz-Content-Sha256") == "" {
		t.Fatalf("expected aws sigv4 headers")
	}
}

func TestResolveMediaEndpointBlackForestLabs(t *testing.T) {
	got, mode, err := resolveMediaEndpoint(store.ProviderConnection{Provider: "black-forest-labs"}, "/v1/images/generations", "image")
	if err != nil {
		t.Fatalf("black-forest-labs endpoint failed: %v", err)
	}
	if got != "https://api.bfl.ai" || mode != "black-forest-labs" {
		t.Fatalf("expected https://api.bfl.ai/black-forest-labs, got %s/%s", got, mode)
	}
}

func TestShouldRefreshOAuthToken(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	c1 := store.ProviderConnection{AuthType: "oauth", RefreshToken: "r1", TokenExpiry: now.Add(4 * time.Minute).Format(time.RFC3339)}
	c2 := store.ProviderConnection{AuthType: "oauth", RefreshToken: "r2", TokenExpiry: now.Add(10 * time.Minute).Format(time.RFC3339)}
	if !shouldRefreshOAuthToken(c1, now) {
		t.Fatalf("expected token with <5m ttl to refresh")
	}
	if shouldRefreshOAuthToken(c2, now) {
		t.Fatalf("did not expect token with >5m ttl to refresh")
	}
}

func TestOAuthRefreshConfig(t *testing.T) {
	c := store.ProviderConnection{
		ProviderSpecificData: map[string]interface{}{
			"tokenUrl":     "https://example.com/token",
			"clientId":     "cid",
			"clientSecret": "sec",
		},
	}
	tokenURL, clientID, clientSecret := oauthRefreshConfig(c)
	if tokenURL != "https://example.com/token" || clientID != "cid" || clientSecret != "sec" {
		t.Fatalf("unexpected refresh config: %s %s %s", tokenURL, clientID, clientSecret)
	}
}

func TestNormalizeOpenAIToGeminiBodyIncludesTools(t *testing.T) {
	body := map[string]interface{}{
		"model":    "gemini-compatible/gemini-1.5-flash",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "weather?"}},
		"tools": []interface{}{map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_weather",
				"description": "Get weather by city",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
					},
				},
			},
		}},
	}
	raw := normalizeOpenAIToGeminiBody(body, "gemini-compatible")
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode transformed body: %v", err)
	}
	tools, ok := out["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("unexpected gemini tools: %#v", out["tools"])
	}
	tool, _ := tools[0].(map[string]interface{})
	decls, ok := tool["functionDeclarations"].([]interface{})
	if !ok || len(decls) != 1 {
		t.Fatalf("unexpected function declarations: %#v", tool)
	}
	decl, _ := decls[0].(map[string]interface{})
	if decl["name"] != "get_weather" {
		t.Fatalf("unexpected declaration: %#v", decl)
	}
}

func TestNormalizeOpenAIToAnthropicBodyIncludesTools(t *testing.T) {
	body := map[string]interface{}{
		"model":    "anthropic-compatible/claude-3-5-sonnet-20241022",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "weather?"}},
		"tools": []interface{}{map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_weather",
				"description": "Get weather by city",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
					},
				},
			},
		}},
	}
	raw := normalizeOpenAIToAnthropicBody(body, "anthropic-compatible")
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode transformed body: %v", err)
	}
	tools, ok := out["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("unexpected anthropic tools: %#v", out["tools"])
	}
	tool, _ := tools[0].(map[string]interface{})
	if tool["name"] != "get_weather" {
		t.Fatalf("unexpected anthropic tool: %#v", tool)
	}
	if _, ok := tool["input_schema"].(map[string]interface{}); !ok {
		t.Fatalf("missing input_schema: %#v", tool)
	}
}

func TestNormalizeGeminiToOpenAIResponseToolCall(t *testing.T) {
	raw := map[string]interface{}{
		"candidates": []interface{}{map[string]interface{}{
			"finishReason": "FUNCTION_CALL",
			"content": map[string]interface{}{
				"parts": []interface{}{map[string]interface{}{
					"functionCall": map[string]interface{}{
						"id":   "call_weather_1",
						"name": "get_weather",
						"args": map[string]interface{}{"city": "Hanoi"},
					},
				}},
			},
		}},
	}
	out := normalizeGeminiToOpenAIResponse(raw)
	choices, ok := out["choices"].([]map[string]interface{})
	if !ok || len(choices) != 1 {
		t.Fatalf("unexpected choices: %#v", out)
	}
	msg, _ := choices[0]["message"].(map[string]interface{})
	calls, ok := msg["tool_calls"].([]map[string]interface{})
	if !ok || len(calls) != 1 {
		t.Fatalf("unexpected tool calls: %#v", msg)
	}
	fn, _ := calls[0]["function"].(map[string]interface{})
	if fn["name"] != "get_weather" || !strings.Contains(fn["arguments"].(string), "Hanoi") {
		t.Fatalf("unexpected tool call function payload: %#v", fn)
	}
	if choices[0]["finish_reason"] != "tool_calls" {
		t.Fatalf("unexpected finish reason: %#v", choices[0]["finish_reason"])
	}
}

func TestNormalizeOpenAIToGeminiBodyMultimodalImageDataURL(t *testing.T) {
	body := map[string]interface{}{
		"model": "gemini-compatible/gemini-1.5-flash",
		"messages": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "Describe this image"},
					map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,aGVsbG8="}},
				},
			},
		},
	}
	raw := normalizeOpenAIToGeminiBody(body, "gemini-compatible")
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode transformed body: %v", err)
	}
	contents, ok := out["contents"].([]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("unexpected contents: %#v", out)
	}
	content, _ := contents[0].(map[string]interface{})
	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) != 2 {
		t.Fatalf("unexpected parts: %#v", content)
	}
	imagePart, _ := parts[1].(map[string]interface{})
	inlineData, ok := imagePart["inline_data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing inline_data: %#v", imagePart)
	}
	if inlineData["mime_type"] != "image/png" || inlineData["data"] != "aGVsbG8=" {
		t.Fatalf("unexpected inline_data: %#v", inlineData)
	}
}

func TestNormalizeOpenAIToAnthropicBodyMultimodalKeepsText(t *testing.T) {
	body := map[string]interface{}{
		"model": "anthropic-compatible/claude-3-5-sonnet-20241022",
		"messages": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "Describe this image"},
					map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,aGVsbG8="}},
				},
			},
		},
	}
	raw := normalizeOpenAIToAnthropicBody(body, "anthropic-compatible")
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode transformed body: %v", err)
	}
	messages, ok := out["messages"].([]interface{})
	if !ok || len(messages) != 1 {
		t.Fatalf("unexpected messages: %#v", out)
	}
	msg, _ := messages[0].(map[string]interface{})
	if msg["content"] != "Describe this image" {
		t.Fatalf("unexpected anthropic content: %#v", msg)
	}
}

func TestNormalizeGeminiSSEToOpenAI(t *testing.T) {
	raw := []byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\ndata: {\"candidates\":[{\"finishReason\":\"STOP\",\"content\":{\"parts\":[{\"text\":\" world\"}]}}]}\n\n")
	out := string(normalizeGeminiSSEToOpenAI(raw))
	if !strings.Contains(out, `"object":"chat.completion.chunk"`) {
		t.Fatalf("expected chunk object, got %s", out)
	}
	if !strings.Contains(out, `"content":"Hello"`) {
		t.Fatalf("expected first content chunk, got %s", out)
	}
	if !strings.Contains(out, `"content":" world"`) {
		t.Fatalf("expected second content chunk, got %s", out)
	}
	if !strings.Contains(out, `data: [DONE]`) {
		t.Fatalf("expected done marker, got %s", out)
	}
}

func TestNormalizeResponseForModeGeminiSSE(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hi\"}]}}]}\n\n")),
	}
	normalized, err := normalizeResponseForMode(resp, "gemini")
	if err != nil {
		t.Fatalf("normalize response: %v", err)
	}
	body, err := io.ReadAll(normalized.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), `"chat.completion.chunk"`) {
		t.Fatalf("unexpected normalized sse body: %s", string(body))
	}
	if got := normalized.Header.Get("Content-Type"); !strings.Contains(strings.ToLower(got), "text/event-stream") {
		t.Fatalf("unexpected content-type: %q", got)
	}
}
