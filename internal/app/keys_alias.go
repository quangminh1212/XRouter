package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"xrouter/internal/store"
)

func (s *Server) handleManagementProviderKeysAlias(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	provider, keyName := providerKeyAlias(r.URL.Path)
	if provider == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		connections := s.store.GetActiveConnections(provider)
		items := make([]map[string]interface{}, 0, len(connections))
		for _, conn := range connections {
			items = append(items, providerKeyAliasPayload(conn))
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{keyName: items})
	case http.MethodPut:
		values, err := decodeProviderKeyAliasValues(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		for _, conn := range s.store.GetActiveConnections(provider) {
			_ = s.store.DeleteProviderConnection(conn.ID)
		}
		created, err := s.createProviderKeyAliasConnections(provider, values)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{keyName: created})
	case http.MethodPatch:
		values, err := decodeProviderKeyAliasValues(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		created, err := s.createProviderKeyAliasConnections(provider, values)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{keyName: created})
	case http.MethodDelete:
		queryKey := strings.TrimSpace(r.URL.Query().Get("api-key"))
		deleted := 0
		for _, conn := range s.store.GetActiveConnections(provider) {
			if queryKey != "" && conn.APIKey != queryKey {
				continue
			}
			if err := s.store.DeleteProviderConnection(conn.ID); err == nil {
				deleted++
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "deleted": deleted})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func providerKeyAlias(path string) (string, string) {
	name := strings.Trim(strings.TrimPrefix(path, "/v0/management/"), "/")
	switch name {
	case "gemini-api-key":
		return "gemini", name
	case "claude-api-key":
		return "claude", name
	case "codex-api-key":
		return "codex", name
	case "vertex-api-key":
		return "vertex", name
	case "openai-compatibility":
		return "openai-compatible", name
	default:
		return "", ""
	}
}

func providerKeyAliasPayload(conn store.ProviderConnection) map[string]interface{} {
	payload := map[string]interface{}{
		"id":        conn.ID,
		"name":      conn.Name,
		"provider":  conn.Provider,
		"isActive":  conn.IsActive,
		"baseUrl":   conn.ProviderSpecificData["baseUrl"],
		"apiType":   conn.ProviderSpecificData["apiType"],
		"createdAt": conn.CreatedAt,
		"updatedAt": conn.UpdatedAt,
	}
	if conn.APIKey != "" {
		payload["api-key"] = maskSecretPreview(conn.APIKey)
	}
	return payload
}

func decodeProviderKeyAliasValues(r *http.Request) ([]map[string]interface{}, error) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}
	raw, ok := body["value"]
	if !ok {
		raw = body["values"]
	}
	if raw == nil {
		return nil, fmt.Errorf("value is required")
	}
	items, ok := raw.([]interface{})
	if !ok {
		items = []interface{}{raw}
	}
	values := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				values = append(values, map[string]interface{}{"api-key": strings.TrimSpace(v)})
			}
		case map[string]interface{}:
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("value is empty")
	}
	return values, nil
}

func (s *Server) createProviderKeyAliasConnections(provider string, values []map[string]interface{}) ([]map[string]interface{}, error) {
	entry, ok := store.GetProviderCatalogEntry(provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider")
	}
	created := make([]map[string]interface{}, 0, len(values))
	for i, value := range values {
		apiKey := firstString(value, "api-key", "apiKey", "key")
		accessToken := firstString(value, "access-token", "accessToken")
		refreshToken := firstString(value, "refresh-token", "refreshToken")
		if apiKey == "" && accessToken == "" {
			return nil, fmt.Errorf("api-key or access-token is required")
		}
		baseURL := firstString(value, "base-url", "baseUrl", "url")
		apiType := firstString(value, "api-type", "apiType")
		if apiType == "" {
			apiType = entry.APIType
		}
		data := map[string]interface{}{"apiType": apiType}
		if baseURL != "" {
			data["baseUrl"] = baseURL
		}
		conn, err := s.store.CreateProviderConnection(store.ProviderConnection{
			Provider:             provider,
			Name:                 fmt.Sprintf("%s compat %d", provider, i+1),
			AuthType:             entry.AuthType,
			APIKey:               apiKey,
			AccessToken:          accessToken,
			RefreshToken:         refreshToken,
			IsActive:             true,
			DefaultModel:         firstString(value, "default-model", "defaultModel"),
			ProviderSpecificData: data,
		})
		if err != nil {
			return nil, err
		}
		created = append(created, providerKeyAliasPayload(conn))
	}
	return created, nil
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
