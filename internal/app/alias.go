package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleManagementDebug(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"debug": s.store.GetSettings().Debug})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"debug": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"debug": settings.Debug})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRequestLog(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"request-log": s.store.GetSettings().RequestLog})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"requestLog": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"request-log": settings.RequestLog})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementUsageQueue(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	count := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("count")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			count = parsed
		}
	}
	logs := s.store.GetRequestLogs(count)
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": logs, "count": len(logs)})
}

func (s *Server) handleManagementProxyURL(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"proxy-url": s.store.GetSettings().OutboundProxyURL})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		value := strings.TrimSpace(body.Value)
		settings, err := s.store.UpdateSettings(map[string]interface{}{"outboundProxyUrl": value, "outboundProxyEnabled": value != ""})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy-url": settings.OutboundProxyURL})
	case http.MethodDelete:
		settings, err := s.store.UpdateSettings(map[string]interface{}{"outboundProxyUrl": "", "outboundProxyEnabled": false})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy-url": settings.OutboundProxyURL})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRequestRetry(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]int{"request-retry": s.store.GetSettings().MaxRetries})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value int `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Value < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "value must be >= 0"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"maxRetries": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"request-retry": settings.MaxRetries})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementMaxRetryInterval(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]int{"max-retry-interval": s.store.GetSettings().MaxCooldownSeconds})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value int `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Value < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "value must be >= 0"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"maxCooldownSeconds": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"max-retry-interval": settings.MaxCooldownSeconds})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func cliProxyRoutingStrategy(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "round-robin", "roundrobin", "rr", "round_robin":
		return "round_robin", true
	case "fill-first", "fillfirst", "ff", "fallback":
		return "fallback", true
	case "cost-optimized", "cost_optimized", "costoptimized", "cheap", "auto/cheap":
		return "cost_optimized", true
	case "auto", "smart", "auto/smart":
		return "auto", true
	case "last-known-good", "last_known_good", "lkg", "lkgp":
		return "last_known_good", true
	default:
		return "", false
	}
}

func (s *Server) handleManagementRoutingStrategyAlias(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		strategy := s.store.GetSettings().ComboStrategy
		if strategy == "round_robin" {
			strategy = "round-robin"
		} else if strategy == "fallback" || strategy == "" {
			strategy = "fill-first"
		}
		writeJSON(w, http.StatusOK, map[string]string{"strategy": strategy})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		strategy, ok := cliProxyRoutingStrategy(body.Value)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid strategy"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"comboStrategy": strategy})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := settings.ComboStrategy
		if out == "round_robin" {
			out = "round-robin"
		} else if out == "fallback" || out == "" {
			out = "fill-first"
		}
		writeJSON(w, http.StatusOK, map[string]string{"strategy": out})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementForceModelPrefix(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"force-model-prefix": s.store.GetSettings().ForceModelPrefix})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"forceModelPrefix": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"force-model-prefix": settings.ForceModelPrefix})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
