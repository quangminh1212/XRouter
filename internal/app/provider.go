package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"xrouter/internal/store"
)

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"connections": s.store.GetAllConnections()})
	case http.MethodPost:
		var body store.ProviderConnection
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Provider) == "" || strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider and name are required"})
			return
		}
		if body.AuthType == "" {
			body.AuthType = "apikey"
		}
		body.IsActive = true
		created, err := s.store.CreateProviderConnection(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		created.APIKey, created.AccessToken, created.RefreshToken = "", "", ""
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleProviderCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	entries := store.ListProviderCatalogEntries()
	apiType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("apiType")))
	authType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("authType")))
	filtered := make([]store.ProviderCatalogEntry, 0, len(entries))
	for _, item := range entries {
		if apiType != "" && strings.ToLower(strings.TrimSpace(item.APIType)) != apiType {
			continue
		}
		if authType != "" && strings.ToLower(strings.TrimSpace(item.AuthType)) != authType {
			continue
		}
		filtered = append(filtered, item)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": filtered,
		"count":     len(filtered),
	})
}

func (s *Server) handleProviderCookieImport(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cookie import is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Provider     string                 `json:"provider"`
		Name         string                 `json:"name"`
		Cookie       string                 `json:"cookie"`
		Session      string                 `json:"session"`
		DefaultModel string                 `json:"defaultModel"`
		AccountName  string                 `json:"accountName"`
		AccountEmail string                 `json:"accountEmail"`
		Data         map[string]interface{} `json:"providerSpecificData"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	provider := strings.TrimSpace(body.Provider)
	if provider == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider is required"})
		return
	}
	cookie := strings.TrimSpace(body.Cookie)
	if cookie == "" {
		cookie = strings.TrimSpace(body.Session)
	}
	if cookie == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cookie or session is required"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = provider + " browser session"
	}
	created, err := s.store.CreateProviderConnection(store.ProviderConnection{
		Provider:             provider,
		Name:                 name,
		AuthType:             "cookie",
		APIKey:               cookie,
		IsActive:             true,
		DefaultModel:         strings.TrimSpace(body.DefaultModel),
		AccountName:          strings.TrimSpace(body.AccountName),
		AccountEmail:         strings.TrimSpace(body.AccountEmail),
		ProviderSpecificData: body.Data,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	created.APIKey = ""
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleProviderByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	if strings.HasSuffix(path, "/test-models") {
		s.handleProviderTestModels(w, r)
		return
	}
	if strings.HasSuffix(path, "/validate") {
		s.handleProviderValidate(w, r)
		return
	}
	if strings.HasSuffix(path, "/models") {
		s.handleProviderModels(w, r)
		return
	}
	if strings.HasSuffix(path, "/test") {
		s.handleProviderTest(w, r)
		return
	}
	id := path
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing provider id"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateProviderConnection(id, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		updated.APIKey, updated.AccessToken, updated.RefreshToken = "", "", ""
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteProviderConnection(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleProviderModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/models")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing provider id"})
		return
	}
	connection, ok := s.store.GetConnectionByIDRaw(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider connection not found"})
		return
	}
	result, err := s.forwarder.GetProviderModels(r.Context(), connection)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/test")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing provider id"})
		return
	}
	connection, ok := s.store.GetConnectionByIDRaw(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider connection not found"})
		return
	}
	result, err := s.forwarder.ProbeConnection(r.Context(), connection)
	if err != nil {
		updated, _ := s.store.UpdateConnectionTestStatus(id, "unavailable", result.Message, result.StatusCode)
		updated.APIKey, updated.AccessToken, updated.RefreshToken = "", "", ""
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"result": result, "connection": updated, "error": err.Error()})
		return
	}
	status := "active"
	if !result.Healthy {
		status = "unavailable"
	}
	updated, updateErr := s.store.UpdateConnectionTestStatus(id, status, result.Message, result.StatusCode)
	if updateErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": updateErr.Error()})
		return
	}
	updated.APIKey, updated.AccessToken, updated.RefreshToken = "", "", ""
	writeJSON(w, http.StatusOK, map[string]interface{}{"result": result, "connection": updated})
}

func (s *Server) handleProviderValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/validate")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing provider id"})
		return
	}
	connection, ok := s.store.GetConnectionByIDRaw(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider connection not found"})
		return
	}
	result, err := s.forwarder.ProbeConnection(r.Context(), connection)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"result": result, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"result": result})
}

func (s *Server) handleProviderTestModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/test-models")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing provider id"})
		return
	}
	connection, ok := s.store.GetConnectionByIDRaw(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider connection not found"})
		return
	}
	var body struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	results, err := s.forwarder.ProbeModels(r.Context(), connection, body.Models)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	passed := 0
	failed := 0
	lastMessage := ""
	lastCode := 0
	for _, item := range results {
		if item.Healthy {
			passed++
			continue
		}
		failed++
		lastMessage = item.Message
		lastCode = item.StatusCode
	}
	status := "active"
	if failed > 0 {
		status = "unavailable"
	}
	updated, updateErr := s.store.UpdateConnectionTestStatus(id, status, lastMessage, lastCode)
	if updateErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": updateErr.Error()})
		return
	}
	updated.APIKey, updated.AccessToken, updated.RefreshToken = "", "", ""
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary": map[string]int{
			"total":  len(results),
			"passed": passed,
			"failed": failed,
		},
		"results":    results,
		"connection": updated,
	})
}
