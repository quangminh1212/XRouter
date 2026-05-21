package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"xrouter/internal/version"
)

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
