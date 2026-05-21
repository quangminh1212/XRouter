package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"xrouter/internal/store"
)

func (s *Server) handleManagementOAuthExcludedModels(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-excluded-models": s.store.GetSettings().OAuthExcludedModels})
	case http.MethodPut:
		var body map[string][]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"oauthExcludedModels": body})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-excluded-models": settings.OAuthExcludedModels})
	case http.MethodPatch:
		var body struct {
			Provider string   `json:"provider"`
			Models   []string `json:"models"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Provider) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		current := s.store.GetSettings().OAuthExcludedModels
		if current == nil {
			current = map[string][]string{}
		}
		provider := strings.ToLower(strings.TrimSpace(body.Provider))
		if len(body.Models) == 0 {
			delete(current, provider)
		} else {
			current[provider] = body.Models
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"oauthExcludedModels": current})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-excluded-models": settings.OAuthExcludedModels})
	case http.MethodDelete:
		provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
		if provider == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider is required"})
			return
		}
		current := s.store.GetSettings().OAuthExcludedModels
		if current == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
			return
		}
		if _, ok := current[provider]; !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
			return
		}
		delete(current, provider)
		settings, err := s.store.UpdateSettings(map[string]interface{}{"oauthExcludedModels": current})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-excluded-models": settings.OAuthExcludedModels})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementOAuthModelAlias(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-model-alias": s.store.GetSettings().OAuthModelAlias})
	case http.MethodPut:
		var body map[string][]map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"oauthModelAlias": body})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-model-alias": settings.OAuthModelAlias})
	case http.MethodPatch:
		var body struct {
			Provider string                   `json:"provider"`
			Channel  string                   `json:"channel"`
			Aliases  []map[string]interface{} `json:"aliases"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		channel := strings.ToLower(strings.TrimSpace(body.Channel))
		if channel == "" {
			channel = strings.ToLower(strings.TrimSpace(body.Provider))
		}
		if channel == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider or channel is required"})
			return
		}
		current := s.store.GetSettings().OAuthModelAlias
		if current == nil {
			current = map[string][]map[string]interface{}{}
		}
		current[channel] = body.Aliases
		settings, err := s.store.UpdateSettings(map[string]interface{}{"oauthModelAlias": current})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-model-alias": settings.OAuthModelAlias})
	case http.MethodDelete:
		channel := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("channel")))
		if channel == "" {
			channel = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
		}
		if channel == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel or provider is required"})
			return
		}
		current := s.store.GetSettings().OAuthModelAlias
		if current == nil || current[channel] == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
			return
		}
		delete(current, channel)
		settings, err := s.store.UpdateSettings(map[string]interface{}{"oauthModelAlias": current})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"oauth-model-alias": settings.OAuthModelAlias})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementAuthURL(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v0/management/"), "/")
	provider := strings.TrimSuffix(name, "-auth-url")
	if provider == name || provider == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	entry, ok := store.GetProviderCatalogEntry(provider)
	if !ok || entry.AuthType != "oauth" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown oauth provider"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":     provider,
		"authorizeUrl": entry.AuthorizeURL,
		"tokenUrl":     entry.TokenURL,
		"clientId":     entry.ClientID,
		"scopes":       entry.Scopes,
	})
}

func (s *Server) handleManagementGetAuthStatus(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	connections := s.store.GetActiveConnections("")
	status := make(map[string]interface{})
	for _, conn := range connections {
		if conn.AuthType == "oauth" {
			status[conn.Provider] = map[string]interface{}{
				"id":         conn.ID,
				"name":       conn.Name,
				"hasToken":   strings.TrimSpace(conn.AccessToken) != "",
				"hasRefresh": strings.TrimSpace(conn.RefreshToken) != "",
				"testStatus": conn.TestStatus,
				"updatedAt":  conn.UpdatedAt,
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"authStatus": status})
}
