package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xrouter/internal/store"
)

func TestGeminiGenerateContentAlias(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false}); err != nil {
		t.Fatalf("disable api key auth: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["model"] != "gemini-1.5-flash" {
			t.Fatalf("unexpected model: %#v", body)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "Xin chao"}}},
		})
	}))
	defer upstream.Close()

	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider: "gemini",
		Name:     "gemini alias",
		AuthType: "oauth",
		APIKey:   "x",
		IsActive: true,
		ProviderSpecificData: map[string]interface{}{
			"baseUrl": upstream.URL,
			"apiType": "openai",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-1.5-flash:generateContent", bytes.NewBufferString(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}],"generationConfig":{"maxOutputTokens":8}}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []map[string]string `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Candidates) != 1 || len(payload.Candidates[0].Content.Parts) != 1 || payload.Candidates[0].Content.Parts[0]["text"] != "Xin chao" {
		t.Fatalf("unexpected gemini payload: %#v", payload)
	}
}

func TestAPIV1BetaCompatGeminiActionRoute(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	hit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"candidates": []map[string]interface{}{{
				"content": map[string]interface{}{
					"role":  "model",
					"parts": []map[string]string{{"text": "hello"}},
				},
				"finishReason": "STOP",
			}},
		})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{
		Provider:             "gemini-compatible",
		Name:                 "gemini api v1beta compat",
		AuthType:             "apikey",
		APIKey:               "x",
		IsActive:             true,
		ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "gemini"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1beta/models/gemini-test:generateContent", bytes.NewBufferString(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !hit {
		t.Fatalf("expected upstream to be called, body=%s", rec.Body.String())
	}
	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []map[string]string `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Candidates) != 1 || len(payload.Candidates[0].Content.Parts) != 1 || payload.Candidates[0].Content.Parts[0]["text"] != "hello" {
		t.Fatalf("unexpected gemini payload: %#v", payload)
	}
}
