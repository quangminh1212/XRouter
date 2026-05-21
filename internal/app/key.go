package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"xrouter/internal/store"
)

func parseInt64Loose(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case string:
		var out int64
		_, err := fmt.Sscan(strings.TrimSpace(n), &out)
		return out, err == nil
	default:
		return 0, false
	}
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "api key management is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"keys": s.store.GetAPIKeys()})
	case http.MethodPost:
		var body struct {
			Name              string `json:"name"`
			Key               string `json:"key"`
			RequestsPerMinute int    `json:"requestsPerMinute"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		item, err := s.store.CreateAPIKey(body.Name, body.Key, body.RequestsPerMinute)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		item.Key = ""
		writeJSON(w, http.StatusCreated, item)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "api key management is restricted to localhost"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/keys/")
	if strings.HasSuffix(path, "/rotate") {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		id := strings.TrimSpace(strings.TrimSuffix(path, "/rotate"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key id is required"})
			return
		}
		var body struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Key) == "" {
			body.Key = generateAPIKey()
		}
		item, err := s.store.RotateAPIKey(id, body.Key)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		item.Key = body.Key
		writeJSON(w, http.StatusOK, item)
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := strings.TrimSpace(path)
	id = strings.TrimSpace(id)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key id is required"})
		return
	}
	if err := s.store.DeleteAPIKey(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	aliases := s.store.GetModelAliases()
	disabled := toStringSet(s.store.GetDisabledModels())
	availability := s.store.GetModelAvailability()
	modelMap := map[string]map[string]string{}
	for model, alias := range aliases {
		if disabled[model] {
			continue
		}
		modelMap[model] = map[string]string{"fullModel": model, "alias": alias, "availability": availabilityOrDefault(availability[model])}
	}
	for _, model := range store.GetFallbackModels() {
		fullModel := strings.TrimSpace(model["fullModel"])
		if fullModel == "" || disabled[fullModel] {
			continue
		}
		if _, ok := modelMap[fullModel]; !ok {
			modelMap[fullModel] = map[string]string{"fullModel": fullModel, "alias": model["alias"], "availability": availabilityOrDefault(availability[fullModel])}
		}
	}
	for _, combo := range s.store.ListComboModels() {
		if strings.TrimSpace(combo.Alias) == "" {
			continue
		}
		modelMap[combo.Alias] = map[string]string{
			"fullModel":    combo.Alias,
			"alias":        combo.Alias,
			"availability": "combo",
			"comboTargets": strings.Join(combo.Targets, ","),
		}
	}
	models := make([]map[string]string, 0, len(modelMap))
	for _, model := range modelMap {
		models = append(models, model)
	}
	slices.SortFunc(models, func(a, b map[string]string) int {
		return strings.Compare(a["fullModel"], b["fullModel"])
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": models})
}

func availabilityOrDefault(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "unknown"
	}
	return status
}

func toStringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func isLocalOnlyRequest(r *http.Request) bool {
	hostCandidates := []string{
		r.URL.Hostname(),
		r.Host,
		r.Header.Get("Host"),
		r.Header.Get("X-Forwarded-Host"),
	}
	for _, raw := range hostCandidates {
		host := strings.ToLower(strings.TrimSpace(raw))
		if host == "" {
			continue
		}
		host = strings.TrimPrefix(host, "[")
		host = strings.TrimSuffix(host, "]")
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
		if slices.Contains([]string{"localhost", "127.0.0.1", "::1"}, host) {
			return true
		}
	}
	return false
}

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
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
	normalizedPath := r.URL.Path
	if strings.HasPrefix(normalizedPath, "/api/v1/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/api")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	if normalizedPath == "/v1/responses/stream" {
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err == nil {
			payload["stream"] = true
			if raw, err := json.Marshal(payload); err == nil {
				body = raw
			}
		}
	}
	if disabledModel, ok := s.resolveDisabledModel(body); ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model is disabled", "model": disabledModel})
		return
	}
	resp, err := s.forwarder.Forward(r.Context(), apiKey.ID, normalizedPath, body)
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
	for _, k := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	contentType := strings.ToLower(w.Header().Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		w.WriteHeader(resp.StatusCode)
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 16*1024)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if readErr == io.EOF {
				return
			}
			if readErr != nil {
				return
			}
		}
	}

	rawResp, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if readErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(rawResp)

	provider := strings.TrimSpace(resp.Header.Get("X-XRouter-Provider"))
	model := strings.TrimSpace(resp.Header.Get("X-XRouter-Model"))
	_ = s.store.RecordRequestLog(store.RequestLog{
		Path:          r.URL.Path,
		Provider:      provider,
		Model:         model,
		APIKeyID:      apiKey.ID,
		StatusCode:    resp.StatusCode,
		LatencyMs:     time.Since(start).Milliseconds(),
		RequestBytes:  len(body),
		ResponseBytes: len(rawResp),
	})
	if usageEntry, ok := extractUsageEntry(rawResp, provider, model); ok {
		_ = s.store.RecordUsage(usageEntry)
	}
}

func (s *Server) resolveDisabledModel(body []byte) (string, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}
	rawModel, _ := payload["model"].(string)
	model := strings.TrimSpace(rawModel)
	if model == "" {
		return "", false
	}
	if target, ok := s.store.GetForcedModelMappings()[model]; ok && strings.TrimSpace(target) != "" {
		model = strings.TrimSpace(target)
	}
	if s.store.IsModelDisabled(model) {
		return model, true
	}
	return "", false
}
