package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"xrouter/internal/version"
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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

func (s *Server) handleManagementLatestVersion(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	info := version.Info()
	latest := strings.TrimSpace(info["version"])
	if latest == "" {
		latest = "dev"
	}
	writeJSON(w, http.StatusOK, map[string]string{"latest-version": latest})
}

func (s *Server) handleManagementBoolValue(w http.ResponseWriter, r *http.Request, key string, value bool, patchKey string) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{key: value})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		_, err := s.store.UpdateSettings(map[string]interface{}{patchKey: body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{key: body.Value})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementUsageStatisticsEnabled(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	s.handleManagementBoolValue(w, r, "usage-statistics-enabled", s.store.GetSettings().UsageStatisticsEnabled, "usageStatisticsEnabled")
}

func (s *Server) handleManagementLoggingToFile(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	s.handleManagementBoolValue(w, r, "logging-to-file", s.store.GetSettings().LoggingToFile, "loggingToFile")
}

func (s *Server) handleManagementWSAuth(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	s.handleManagementBoolValue(w, r, "ws-auth", s.store.GetSettings().WebsocketAuth, "websocketAuth")
}

func (s *Server) handleManagementIntValue(w http.ResponseWriter, r *http.Request, key string, current int, patchKey string, minValue int) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]int{key: current})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value int `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Value < minValue {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "value too small"})
			return
		}
		_, err := s.store.UpdateSettings(map[string]interface{}{patchKey: body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{key: body.Value})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementLogsMaxTotalSizeMB(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	s.handleManagementIntValue(w, r, "logs-max-total-size-mb", s.store.GetSettings().LogsMaxTotalSizeMB, "logsMaxTotalSizeMb", 0)
}

func (s *Server) handleManagementErrorLogsMaxFiles(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	s.handleManagementIntValue(w, r, "error-logs-max-files", s.store.GetSettings().ErrorLogsMaxFiles, "errorLogsMaxFiles", 0)
}

func (s *Server) handleManagementLogs(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		lines := []string{}
		for _, log := range s.store.GetRequestLogs(200) {
			lines = append(lines, fmt.Sprintf("%s %d %s", log.Timestamp, log.StatusCode, log.Path))
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines, "count": len(lines)})
	case http.MethodDelete:
		_, _ = s.store.UpdateSettings(map[string]interface{}{"requestLog": false})
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Logs cleared successfully"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRequestErrorLogs(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v0/management/request-error-logs"), "/")
	if name == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": []map[string]interface{}{}, "count": 0})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "request error log not found"})
}
