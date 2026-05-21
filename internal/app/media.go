package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xrouter/internal/proxy"
	"xrouter/internal/store"
)

func (s *Server) handleAudioVoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	if provider == "" {
		provider = providerFromVoicePath(r.URL.Path)
	}
	if provider == "" {
		provider = "openai-tts"
	}
	voices := []map[string]string{
		{"id": "alloy", "name": "alloy", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/alloy"},
		{"id": "ash", "name": "ash", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/ash"},
		{"id": "ballad", "name": "ballad", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/ballad"},
		{"id": "coral", "name": "coral", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/coral"},
		{"id": "echo", "name": "echo", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/echo"},
		{"id": "fable", "name": "fable", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/fable"},
		{"id": "nova", "name": "nova", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/nova"},
		{"id": "onyx", "name": "onyx", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/onyx"},
		{"id": "sage", "name": "sage", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/sage"},
		{"id": "shimmer", "name": "shimmer", "provider": provider, "language": "multilingual", "lang": "multi", "model": provider + "/shimmer"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"voices":    voices,
		"object":    "list",
		"data":      voices,
		"languages": []string{"multi"},
		"byLang": map[string]interface{}{
			"multi": map[string]interface{}{"voices": voices},
		},
	})
}

func providerFromVoicePath(path string) string {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/media-providers/tts/"), "/")
	if trimmed == "" || trimmed == "voices" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) >= 2 && parts[1] == "voices" {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func geminiContentsToMessages(contents []map[string]interface{}) []map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(contents))
	for _, item := range contents {
		role := "user"
		if v, ok := item["role"].(string); ok && strings.TrimSpace(v) != "" {
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "model", "assistant":
				role = "assistant"
			default:
				role = "user"
			}
		}
		parts, _ := item["parts"].([]interface{})
		contentParts := make([]interface{}, 0, len(parts))
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]interface{})
			if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
				contentParts = append(contentParts, map[string]interface{}{"type": "text", "text": text})
			}
		}
		if len(contentParts) == 0 {
			continue
		}
		messages = append(messages, map[string]interface{}{"role": role, "content": contentParts})
	}
	return messages
}

func openAIResponseToGemini(raw map[string]interface{}) map[string]interface{} {
	text := ""
	if choices, ok := raw["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					text = content
				}
			}
		}
	}
	return map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"role":  "model",
					"parts": []map[string]string{{"text": text}},
				},
				"finishReason": "STOP",
			},
		},
	}
}

func (s *Server) handleGeminiAction(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	apiKey, err := s.authorize(r)
	if err != nil {
		if strings.Contains(err.Error(), "rate limit exceeded") {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1beta/models/") {
		path = strings.TrimPrefix(path, "/api")
	}
	trimmed := strings.TrimPrefix(path, "/v1beta/models/")
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid gemini action path"})
		return
	}
	modelName := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	var input map[string]interface{}
	if err := json.Unmarshal(raw, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	contentItems := []map[string]interface{}{}
	if items, ok := input["contents"].([]interface{}); ok {
		for _, item := range items {
			if mapped, ok := item.(map[string]interface{}); ok {
				contentItems = append(contentItems, mapped)
			}
		}
	}
	converted := map[string]interface{}{
		"model":    "gemini/" + modelName,
		"messages": geminiContentsToMessages(contentItems),
	}
	if cfg, ok := input["generationConfig"].(map[string]interface{}); ok {
		if v, ok := cfg["maxOutputTokens"]; ok {
			converted["max_tokens"] = v
		}
		if v, ok := cfg["temperature"]; ok {
			converted["temperature"] = v
		}
	}
	if action == "streamGenerateContent" {
		converted["stream"] = true
	}
	body, _ := json.Marshal(converted)
	resp, err := s.forwarder.Forward(r.Context(), apiKey.ID, "/v1/chat/completions", body)
	if err != nil {
		_ = s.store.RecordRequestLog(store.RequestLog{
			Path:         r.URL.Path,
			APIKeyID:     apiKey.ID,
			StatusCode:   http.StatusBadGateway,
			LatencyMs:    time.Since(start).Milliseconds(),
			RequestBytes: len(body),
			Error:        err.Error(),
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	if action == "streamGenerateContent" {
		for _, k := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
			if v := resp.Header.Get(k); v != "" {
				w.Header().Set(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(responseBody)
		return
	}
	var openAIResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &openAIResponse); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid upstream response"})
		return
	}
	writeJSON(w, http.StatusOK, openAIResponseToGemini(openAIResponse))
}

func resolveConnectionBaseURL(conn store.ProviderConnection) (string, bool) {
	if conn.ProviderSpecificData != nil {
		if v, ok := conn.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimRight(strings.TrimSpace(v), "/"), true
		}
	}
	entry, ok := store.GetProviderCatalogEntry(conn.Provider)
	if !ok {
		return "", false
	}
	return strings.TrimRight(strings.TrimSpace(entry.BaseURL), "/"), true
}

func (s *Server) handleVideoByID(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	apiKey, err := s.authorize(r)
	if err != nil {
		if strings.Contains(err.Error(), "rate limit exceeded") {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	videoID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/videos/"), "/")
	if videoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing video id"})
		return
	}
	connections := s.store.GetActiveConnections("")
	var targetConn *store.ProviderConnection
	for i := range connections {
		conn := &connections[i]
		entry, ok := store.GetProviderCatalogEntry(conn.Provider)
		if !ok {
			continue
		}
		if entry.APIType == "video" || entry.APIType == "openai" {
			targetConn = conn
			break
		}
	}
	if targetConn == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "no active video provider connections"})
		return
	}
	baseURL, ok := resolveConnectionBaseURL(*targetConn)
	if !ok || baseURL == "" {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "video provider base url is missing"})
		return
	}
	upstreamURL := baseURL + "/v1/videos/" + url.PathEscape(videoID)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to build upstream request"})
		return
	}
	if strings.TrimSpace(targetConn.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(targetConn.APIKey))
	} else if strings.TrimSpace(targetConn.AccessToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(targetConn.AccessToken))
	}
	resp, err := s.forwarder.HTTPClient().Do(req)
	if err != nil {
		_ = s.store.RecordRequestLog(store.RequestLog{
			Path:       r.URL.Path,
			APIKeyID:   apiKey.ID,
			StatusCode: http.StatusBadGateway,
			LatencyMs:  time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	_ = s.store.RecordRequestLog(store.RequestLog{
		Path:          r.URL.Path,
		APIKeyID:      apiKey.ID,
		StatusCode:    resp.StatusCode,
		LatencyMs:     time.Since(start).Milliseconds(),
		ResponseBytes: len(body),
	})
	for _, k := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func (s *Server) handleMediaProxy(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	normalizedPath := r.URL.Path
	if strings.HasPrefix(normalizedPath, "/api/v1/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/api")
	}
	apiKey, err := s.authorize(r)
	if err != nil {
		if strings.Contains(err.Error(), "rate limit exceeded") {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 24*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	providerHint := ""
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") || normalizedPath == "/v1/embeddings" || normalizedPath == "/v1/audio/speech" {
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err == nil {
			if v, ok := payload["provider"].(string); ok {
				providerHint = strings.TrimSpace(v)
				delete(payload, "provider")
			}
			if providerHint == "" {
				if model, ok := payload["model"].(string); ok && strings.Contains(model, "/") {
					parts := strings.SplitN(model, "/", 2)
					providerHint = strings.TrimSpace(parts[0])
					payload["model"] = strings.TrimSpace(parts[1])
				}
			}
			if nextBody, err := json.Marshal(payload); err == nil {
				body = nextBody
			}
		}
	}
	resp, err := s.forwarder.ForwardMedia(r.Context(), proxy.MediaRequest{
		Path:     normalizedPath,
		Body:     body,
		Provider: providerHint,
		Headers:  r.Header.Clone(),
	})
	if err != nil {
		_ = s.store.RecordRequestLog(store.RequestLog{
			Path:         r.URL.Path,
			APIKeyID:     apiKey.ID,
			StatusCode:   http.StatusBadGateway,
			LatencyMs:    time.Since(start).Milliseconds(),
			RequestBytes: len(body),
			Error:        err.Error(),
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	for _, k := range []string{"Content-Type", "Cache-Control", "X-Request-Id", "X-XRouter-Provider"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	rawResp, readErr := io.ReadAll(io.LimitReader(resp.Body, 12*1024*1024))
	if readErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(rawResp)
	_ = s.store.RecordRequestLog(store.RequestLog{
		Path:          r.URL.Path,
		Provider:      strings.TrimSpace(resp.Header.Get("X-XRouter-Provider")),
		APIKeyID:      apiKey.ID,
		StatusCode:    resp.StatusCode,
		LatencyMs:     time.Since(start).Milliseconds(),
		RequestBytes:  len(body),
		ResponseBytes: len(rawResp),
	})
}

func asInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func extractUsageEntry(raw []byte, provider, model string) (store.UsageEntry, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return store.UsageEntry{}, false
	}
	usageRaw, ok := payload["usage"].(map[string]interface{})
	if !ok {
		return store.UsageEntry{}, false
	}
	entry := store.UsageEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Provider:  provider,
		Model:     model,
		TotalCost: 0,
	}
	if v, ok := asInt64(usageRaw["prompt_tokens"]); ok {
		entry.PromptTokens = v
	}
	if v, ok := asInt64(usageRaw["input_tokens"]); ok && entry.PromptTokens == 0 {
		entry.PromptTokens = v
	}
	if v, ok := asInt64(usageRaw["completion_tokens"]); ok {
		entry.CompletionTokens = v
	}
	if v, ok := asInt64(usageRaw["output_tokens"]); ok && entry.CompletionTokens == 0 {
		entry.CompletionTokens = v
	}
	if v, ok := asInt64(usageRaw["total_tokens"]); ok {
		if entry.PromptTokens == 0 && entry.CompletionTokens == 0 {
			entry.PromptTokens = v
		}
	}
	return entry, true
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(data)
}
