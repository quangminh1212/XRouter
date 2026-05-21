package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"xrouter/internal/store"
)

func (s *Server) handleOAuthProviders(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	providers := make([]store.ProviderCatalogEntry, 0)
	for _, entry := range store.ListProviderCatalogEntries() {
		if entry.AuthType == "oauth" {
			providers = append(providers, entry)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}

func (s *Server) handleOAuthProviderImport(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Path)
	if strings.HasSuffix(path, "/start") {
		s.handleOAuthProviderStart(w, r)
		return
	}
	if strings.HasSuffix(path, "/exchange") {
		s.handleOAuthProviderExchange(w, r)
		return
	}
	if strings.HasSuffix(path, "/refresh") {
		s.handleOAuthProviderRefresh(w, r)
		return
	}
	if strings.HasSuffix(path, "/import-file") {
		s.handleVertexCredentialImport(w, r)
		return
	}
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/oauth/providers/")
	provider := strings.TrimSuffix(strings.TrimSpace(suffix), "/import")
	if !strings.HasSuffix(suffix, "/import") || provider == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	entry, ok := store.GetProviderCatalogEntry(provider)
	if !ok || entry.AuthType != "oauth" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown oauth provider"})
		return
	}
	var body struct {
		Name                 string                 `json:"name"`
		AccessToken          string                 `json:"accessToken"`
		RefreshToken         string                 `json:"refreshToken"`
		APIKey               string                 `json:"apiKey"`
		DefaultModel         string                 `json:"defaultModel"`
		ProviderSpecificData map[string]interface{} `json:"providerSpecificData"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(body.AccessToken) == "" && strings.TrimSpace(body.APIKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "accessToken or apiKey is required"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = entry.Provider + " oauth"
	}
	created, err := s.store.CreateProviderConnection(store.ProviderConnection{
		Provider:             entry.Provider,
		Name:                 name,
		AuthType:             "oauth",
		APIKey:               strings.TrimSpace(body.APIKey),
		AccessToken:          strings.TrimSpace(body.AccessToken),
		RefreshToken:         strings.TrimSpace(body.RefreshToken),
		IsActive:             true,
		DefaultModel:         strings.TrimSpace(body.DefaultModel),
		ProviderSpecificData: body.ProviderSpecificData,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	created.APIKey, created.AccessToken, created.RefreshToken = "", "", ""
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleOAuthProviderRefresh(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/oauth/providers/")
	id := strings.TrimSuffix(strings.TrimSpace(suffix), "/refresh")
	if !strings.HasSuffix(suffix, "/refresh") || id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	connection, ok := s.store.GetConnectionByIDRaw(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider connection not found"})
		return
	}
	if connection.AuthType != "oauth" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider connection is not oauth"})
		return
	}
	var body struct {
		TokenURL     string `json:"tokenUrl"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	result, err := s.forwarder.RefreshOAuthToken(r.Context(), connection, strings.TrimSpace(body.TokenURL), strings.TrimSpace(body.ClientID), strings.TrimSpace(body.ClientSecret))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	updated, err := s.store.UpdateOAuthTokens(connection.ID, result.AccessToken, result.RefreshToken, result.TokenExpiry)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	updated.APIKey, updated.AccessToken, updated.RefreshToken = "", "", ""
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"connection":  updated,
		"tokenType":   result.TokenType,
		"tokenExpiry": result.TokenExpiry,
	})
}

func (s *Server) handleVertexCredentialImport(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/oauth/providers/")
	provider := strings.TrimSuffix(strings.TrimSpace(suffix), "/import-file")
	if !strings.HasSuffix(suffix, "/import-file") || provider != "vertex" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	var body struct {
		Name         string `json:"name"`
		AuthFileID   string `json:"authFileId"`
		DefaultModel string `json:"defaultModel"`
		ProjectID    string `json:"projectId"`
		Location     string `json:"location"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	authFile, ok := s.store.GetAuthFile(strings.TrimSpace(body.AuthFileID))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "auth file not found"})
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(authFile.ContentB64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth file content is invalid"})
		return
	}
	var creds map[string]interface{}
	if err := json.Unmarshal(decoded, &creds); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth file must contain JSON credentials"})
		return
	}
	projectID := strings.TrimSpace(body.ProjectID)
	if projectID == "" {
		if raw, ok := creds["project_id"].(string); ok {
			projectID = strings.TrimSpace(raw)
		}
	}
	location := strings.TrimSpace(body.Location)
	if location == "" {
		location = "us-central1"
	}
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "vertex oauth"
	}
	created, err := s.store.CreateProviderConnection(store.ProviderConnection{
		Provider:     "vertex",
		Name:         name,
		AuthType:     "oauth",
		IsActive:     true,
		DefaultModel: strings.TrimSpace(body.DefaultModel),
		ProviderSpecificData: map[string]interface{}{
			"authFileId": strings.TrimSpace(body.AuthFileID),
			"projectId":  projectID,
			"location":   location,
		},
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleAuthFiles(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "auth file api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"files": s.store.ListAuthFiles(r.URL.Query().Get("provider"), r.URL.Query().Get("account"))})
	case http.MethodPost:
		var body struct {
			Name         string `json:"name"`
			Provider     string `json:"provider"`
			AccountName  string `json:"accountName"`
			AccountEmail string `json:"accountEmail"`
			ContentB64   string `json:"contentB64"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 3*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		name := strings.TrimSpace(body.Name)
		contentB64 := strings.TrimSpace(body.ContentB64)
		if name == "" || contentB64 == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and contentB64 are required"})
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(contentB64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contentB64 must be valid base64"})
			return
		}
		if len(decoded) > 2*1024*1024 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth file too large"})
			return
		}
		created, err := s.store.CreateAuthFile(store.AuthFile{
			Name:         name,
			Provider:     strings.TrimSpace(body.Provider),
			AccountName:  strings.TrimSpace(body.AccountName),
			AccountEmail: strings.TrimSpace(body.AccountEmail),
			ContentB64:   contentB64,
			Size:         len(decoded),
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		created.ContentB64 = ""
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAuthFileByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "auth file api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/auth-files/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing auth file id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, ok := s.store.GetAuthFile(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "auth file not found"})
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(item.ContentB64)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stored auth file is invalid"})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", item.Name))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(decoded)
	case http.MethodDelete:
		if err := s.store.DeleteAuthFile(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
