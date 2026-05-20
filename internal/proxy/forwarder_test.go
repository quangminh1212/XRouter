package proxy

import (
	"net/http"
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
