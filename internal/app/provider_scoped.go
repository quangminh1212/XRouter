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
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || !strings.HasPrefix(parts[1], "v1/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	provider := strings.TrimSpace(parts[0])
	path := "/" + strings.TrimSpace(parts[1])
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
	switch path {
	case "/v1/chat/completions", "/v1/completions", "/v1/messages", "/v1/responses":
		s.handleProxy(w, next)
	case "/v1/embeddings", "/v1/images/generations", "/v1/images/edits", "/v1/audio/speech", "/v1/audio/transcriptions", "/v1/videos/generations", "/v1/videos/edits", "/v1/videos/extensions":
		s.handleMediaProxy(w, next)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unsupported provider scoped path"})
	}
}
