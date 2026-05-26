package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (s *Server) handleProviderScopedProxy(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/provider/")
	if strings.HasPrefix(r.URL.Path, "/api/v1/providers/") {
		trimmed = strings.TrimPrefix(r.URL.Path, "/api/v1/providers/")
	}
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	provider := strings.TrimSpace(parts[0])
	path := normalizeProviderScopedPath(strings.TrimSpace(parts[1]))
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 24*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") && len(body) > 0 {
		var payload map[string]interface{}
		if json.Unmarshal(body, &payload) == nil {
			if model, ok := payload["model"].(string); ok && strings.TrimSpace(model) != "" && !strings.Contains(model, "/") {
				payload["model"] = provider + "/" + strings.TrimSpace(model)
			}
			body, _ = json.Marshal(payload)
		}
	}
	next := r.Clone(r.Context())
	next.URL.Path = path
	next.RequestURI = ""
	next.Body = io.NopCloser(bytes.NewReader(body))
	next.ContentLength = int64(len(body))
	switch {
	case path == "/v1/models" || strings.HasPrefix(path, "/v1/models/"):
		s.handleModels(w, next)
	case path == "/v1/search" || path == "/v1/web/search":
		s.handleSearch(w, next)
	case path == "/v1/web/fetch":
		s.handleWebFetch(w, next)
	case path == "/v1/audio/voices":
		s.handleAudioVoices(w, next)
	case strings.HasPrefix(path, "/v1/videos/") && path != "/v1/videos/generations" && path != "/v1/videos/edits" && path != "/v1/videos/extensions":
		s.handleVideoByID(w, next)
	case path == "/v1/chat/completions" || path == "/v1/completions" || path == "/v1/messages" || path == "/v1/messages/count_tokens" || path == "/v1/responses" || path == "/v1/responses/compact" || path == "/v1/responses/stream":
		s.handleProxy(w, next)
	case path == "/v1/embeddings" || path == "/v1/images/generations" || path == "/v1/images/edits" || path == "/v1/images/analyze" || path == "/v1/audio/generations" || path == "/v1/audio/speech" || path == "/v1/audio/transcriptions" || path == "/v1/videos/generations" || path == "/v1/videos/edits" || path == "/v1/videos/extensions":
		s.handleMediaProxy(w, next)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unsupported provider scoped path"})
	}
}

func normalizeProviderScopedPath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "v1/") {
		return "/" + path
	}
	return "/v1/" + path
}
