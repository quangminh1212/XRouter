package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"xrouter/internal/proxy"
	"xrouter/internal/store"
)

type Server struct {
	store     *store.Store
	forwarder *proxy.Forwarder
	mux       *http.ServeMux
	startedAt time.Time
	limits    map[string]*rateBucket
	limitMu   sync.Mutex
	oauthMu   sync.Mutex
	oauth     map[string]oauthSession
}

type rateBucket struct {
	windowStart time.Time
	count       int
}

type oauthSession struct {
	Provider     string
	State        string
	CodeVerifier string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	TokenURL     string
	CreatedAt    time.Time
}

func generateAPIKey() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return "xr_" + hex.EncodeToString(buf)
}

func NewServer() (*Server, error) {
	st, err := store.NewStore()
	if err != nil {
		return nil, err
	}
	s := &Server{store: st, forwarder: proxy.NewForwarder(st), mux: http.NewServeMux(), startedAt: time.Now(), limits: map[string]*rateBucket{}, oauth: map[string]oauthSession{}}
	s.routes()
	go s.backgroundReload()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("X-Powered-By", "xrouter")
	s.mux.ServeHTTP(w, r)
	if d := time.Since(start); d > 500*time.Millisecond {
		log.Printf("slow request %s %s took %s", r.Method, r.URL.Path, d)
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/providers", s.handleProviders)
	s.mux.HandleFunc("/api/providers/", s.handleProviderByID)
	s.mux.HandleFunc("/api/oauth/providers", s.handleOAuthProviders)
	s.mux.HandleFunc("/api/oauth/providers/", s.handleOAuthProviderImport)
	s.mux.HandleFunc("/api/oauth/callback", s.handleOAuthCallback)
	s.mux.HandleFunc("/api/keys", s.handleAPIKeys)
	s.mux.HandleFunc("/api/keys/", s.handleAPIKeyByID)
	s.mux.HandleFunc("/api/models", s.handleModels)
	s.mux.HandleFunc("/api/management/model-mappings", s.handleManagementModelMappings)
	s.mux.HandleFunc("/api/quota", s.handleQuota)
	s.mux.HandleFunc("/api/usage/summary", s.handleQuota)
	s.mux.HandleFunc("/api/usage/logs", s.handleUsageLogs)
	s.mux.HandleFunc("/api/debug/db", s.handleDebugDB)
	s.mux.HandleFunc("/api/monitoring/health", s.handleMonitoringHealth)
	s.mux.HandleFunc("/v1/chat/completions", s.handleProxy)
	s.mux.HandleFunc("/v1/messages", s.handleProxy)
	s.mux.HandleFunc("/v1/responses", s.handleProxy)
	s.mux.HandleFunc("/v1/search", s.handleSearch)
	s.mux.HandleFunc("/v1/embeddings", s.handleMediaProxy)
	s.mux.HandleFunc("/v1/audio/speech", s.handleMediaProxy)
	s.mux.HandleFunc("/v1/audio/transcriptions", s.handleMediaProxy)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"name": "xrouter", "status": "ok", "uptimeSec": int(time.Since(s.startedAt).Seconds())})
	})
}

func (s *Server) backgroundReload() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.store.Reload(); err != nil {
			log.Printf("db reload failed: %v", err)
		}
		s.cleanupRateBuckets()
	}
}

func (s *Server) authorize(r *http.Request) (store.APIKey, error) {
	settings := s.store.GetSettings()
	if !settings.RequireAPIKey {
		return store.APIKey{}, nil
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" || !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return store.APIKey{}, fmt.Errorf("missing bearer token")
	}
	key := strings.TrimSpace(auth[len("Bearer "):])
	apiKey, ok := s.store.GetAPIKeyByValue(key)
	if !ok {
		return store.APIKey{}, fmt.Errorf("invalid api key")
	}
	if !s.allowAPIKey(apiKey, settings.DefaultRequestsPerMinute) {
		return store.APIKey{}, fmt.Errorf("rate limit exceeded")
	}
	return apiKey, nil
}

func (s *Server) allowAPIKey(apiKey store.APIKey, defaultLimit int) bool {
	limit := apiKey.RequestsPerMinute
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit <= 0 {
		return true
	}
	now := time.Now()
	s.limitMu.Lock()
	defer s.limitMu.Unlock()
	bucket := s.limits[apiKey.ID]
	if bucket == nil || now.Sub(bucket.windowStart) >= time.Minute {
		s.limits[apiKey.ID] = &rateBucket{windowStart: now, count: 1}
		return true
	}
	if bucket.count >= limit {
		return false
	}
	bucket.count++
	return true
}

func (s *Server) cleanupRateBuckets() {
	cutoff := time.Now().Add(-2 * time.Minute)
	s.limitMu.Lock()
	defer s.limitMu.Unlock()
	for key, bucket := range s.limits {
		if bucket.windowStart.Before(cutoff) {
			delete(s.limits, key)
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "status": "ok", "timestamp": time.Now().UTC().Format(time.RFC3339), "runtime": map[string]interface{}{"goVersion": runtime.Version(), "goroutines": runtime.NumGoroutine(), "heapAlloc": m.HeapAlloc, "heapInuse": m.HeapInuse, "nextGC": m.NextGC, "loadedFromData": store.DataDir()}})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.GetSettings())
	case http.MethodPatch:
		if !isLocalOnlyRequest(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "settings update is restricted to localhost"})
			return
		}
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if _, forbidden := patch["password"]; forbidden {
			writeJSON(w, http.StatusGone, map[string]string{"error": "Password auth has been removed. Use OAuth QR login."})
			return
		}
		settings, err := s.store.UpdateSettings(patch)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, settings)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
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

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *Server) handleOAuthProviderStart(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	provider, entry, ok := parseOAuthProviderPath(r.URL.Path, "/start")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown oauth provider"})
		return
	}
	var body struct {
		RedirectURI  string   `json:"redirectUri"`
		ClientID     string   `json:"clientId"`
		ClientSecret string   `json:"clientSecret"`
		AuthorizeURL string   `json:"authorizeUrl"`
		TokenURL     string   `json:"tokenUrl"`
		Scopes       []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	redirectURI := strings.TrimSpace(body.RedirectURI)
	if redirectURI == "" {
		redirectURI = "http://localhost:1213/api/oauth/callback"
	}
	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(entry.ClientID)
	}
	authorizeURL := strings.TrimSpace(body.AuthorizeURL)
	if authorizeURL == "" {
		authorizeURL = strings.TrimSpace(entry.AuthorizeURL)
	}
	tokenURL := strings.TrimSpace(body.TokenURL)
	if tokenURL == "" {
		tokenURL = strings.TrimSpace(entry.TokenURL)
	}
	scopes := body.Scopes
	if len(scopes) == 0 {
		scopes = entry.Scopes
	}
	if clientID == "" || authorizeURL == "" || tokenURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "clientId, authorizeUrl and tokenUrl are required"})
		return
	}
	verifier := randomHex(32)
	state := randomHex(16)
	u, err := url.Parse(authorizeURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid authorizeUrl"})
		return
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", pkceChallenge(verifier))
	q.Set("code_challenge_method", "S256")
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	u.RawQuery = q.Encode()

	s.oauthMu.Lock()
	s.oauth[state] = oauthSession{
		Provider:     provider,
		State:        state,
		CodeVerifier: verifier,
		RedirectURI:  redirectURI,
		ClientID:     clientID,
		ClientSecret: strings.TrimSpace(body.ClientSecret),
		TokenURL:     tokenURL,
		CreatedAt:    time.Now(),
	}
	s.oauthMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":         provider,
		"state":            state,
		"authorizationUrl": u.String(),
		"redirectUri":      redirectURI,
	})
}

func (s *Server) handleOAuthProviderExchange(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	provider, _, ok := parseOAuthProviderPath(r.URL.Path, "/exchange")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown oauth provider"})
		return
	}
	var body struct {
		State string `json:"state"`
		Code  string `json:"code"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	session, ok := s.popOAuthSession(strings.TrimSpace(body.State), provider)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "oauth session not found"})
		return
	}
	result, err := s.exchangeOAuthCode(r.Context(), session, strings.TrimSpace(body.Code))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = provider + " oauth"
	}
	created, err := s.store.CreateProviderConnection(store.ProviderConnection{
		Provider:     provider,
		Name:         name,
		AuthType:     "oauth",
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenExpiry:  result.TokenExpiry,
		IsActive:     true,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	created.APIKey, created.AccessToken, created.RefreshToken = "", "", ""
	writeJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "connection": created})
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing state"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": state, "code": code})
}

func parseOAuthProviderPath(path, suffix string) (string, store.ProviderCatalogEntry, bool) {
	raw := strings.TrimPrefix(strings.TrimSpace(path), "/api/oauth/providers/")
	provider := strings.TrimSuffix(raw, suffix)
	provider = strings.TrimSuffix(provider, "/")
	entry, ok := store.GetProviderCatalogEntry(provider)
	if !ok || entry.AuthType != "oauth" {
		return "", store.ProviderCatalogEntry{}, false
	}
	return provider, entry, true
}

func (s *Server) popOAuthSession(state, provider string) (oauthSession, bool) {
	s.oauthMu.Lock()
	defer s.oauthMu.Unlock()
	session, ok := s.oauth[state]
	if !ok || session.Provider != provider {
		return oauthSession{}, false
	}
	delete(s.oauth, state)
	return session, true
}

func (s *Server) exchangeOAuthCode(ctx context.Context, session oauthSession, code string) (proxy.OAuthRefreshResult, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", session.RedirectURI)
	form.Set("client_id", session.ClientID)
	form.Set("code_verifier", session.CodeVerifier)
	if session.ClientSecret != "" {
		form.Set("client_secret", session.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, session.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.forwarder.HTTPClient().Do(req)
	if err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return proxy.OAuthRefreshResult{}, fmt.Errorf("oauth exchange failed with status %d", resp.StatusCode)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	result := proxy.OAuthRefreshResult{
		AccessToken:  strings.TrimSpace(fmt.Sprint(payload["access_token"])),
		RefreshToken: strings.TrimSpace(fmt.Sprint(payload["refresh_token"])),
		TokenType:    strings.TrimSpace(fmt.Sprint(payload["token_type"])),
		Raw:          payload,
	}
	if expiresIn, ok := payload["expires_in"]; ok {
		if seconds, ok := parseInt64Loose(expiresIn); ok && seconds > 0 {
			result.TokenExpiry = time.Now().UTC().Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
		}
	}
	if result.AccessToken == "" || result.AccessToken == "<nil>" {
		return proxy.OAuthRefreshResult{}, fmt.Errorf("oauth exchange response missing access_token")
	}
	if result.RefreshToken == "<nil>" {
		result.RefreshToken = ""
	}
	return result, nil
}

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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
	modelMap := map[string]map[string]string{}
	for model, alias := range aliases {
		modelMap[model] = map[string]string{"fullModel": model, "alias": alias}
	}
	for _, model := range store.GetFallbackModels() {
		fullModel := strings.TrimSpace(model["fullModel"])
		if fullModel == "" {
			continue
		}
		if _, ok := modelMap[fullModel]; !ok {
			modelMap[fullModel] = map[string]string{"fullModel": fullModel, "alias": model["alias"]}
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

func (s *Server) handleManagementModelMappings(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"mappings": s.store.GetForcedModelMappings()})
	case http.MethodPut:
		var body struct {
			Mappings map[string]string `json:"mappings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		mappings, err := s.store.ReplaceForcedModelMappings(body.Mappings)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "mappings": mappings})
	case http.MethodPatch:
		var body struct {
			Mappings map[string]string `json:"mappings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		mappings, err := s.store.PatchForcedModelMappings(body.Mappings)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "mappings": mappings})
	case http.MethodDelete:
		var body struct {
			Aliases []string `json:"aliases"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		mappings, err := s.store.DeleteForcedModelMappingKeys(body.Aliases)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "mappings": mappings})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleMonitoringHealth(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "monitoring api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.monitoringHealthPayload())
	case http.MethodPost:
		cleared, err := s.store.ClearAllCooldowns()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		payload := s.monitoringHealthPayload()
		payload["success"] = true
		payload["clearedCooldowns"] = cleared
		writeJSON(w, http.StatusOK, payload)
	case http.MethodDelete:
		connectionID := strings.TrimSpace(r.URL.Query().Get("connectionId"))
		provider := strings.TrimSpace(r.URL.Query().Get("provider"))
		if connectionID == "" && provider == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "connectionId or provider is required"})
			return
		}
		cleared, err := s.store.ClearHealthState(connectionID, provider)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		payload := s.monitoringHealthPayload()
		payload["success"] = true
		payload["clearedCooldowns"] = cleared
		writeJSON(w, http.StatusOK, payload)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) monitoringHealthPayload() map[string]interface{} {
	now := time.Now().UTC()
	connections := s.store.GetAllConnectionsRaw()
	providers := map[string]map[string]interface{}{}
	cooldowns := make([]map[string]interface{}, 0)
	totals := map[string]int{
		"connections":  len(connections),
		"active":       0,
		"inactive":     0,
		"cooldown":     0,
		"unavailable":  0,
		"providerKeys": 0,
	}

	for _, conn := range connections {
		providerName := strings.TrimSpace(conn.Provider)
		if providerName == "" {
			providerName = "unknown"
		}
		provider := providers[providerName]
		if provider == nil {
			provider = map[string]interface{}{
				"provider":    providerName,
				"connections": 0,
				"active":      0,
				"inactive":    0,
				"cooldown":    0,
				"unavailable": 0,
			}
			providers[providerName] = provider
			totals["providerKeys"]++
		}

		provider["connections"] = provider["connections"].(int) + 1
		if !conn.IsActive {
			totals["inactive"]++
			provider["inactive"] = provider["inactive"].(int) + 1
			continue
		}

		inCooldown := false
		if conn.RateLimitedUntil != "" {
			if until, err := time.Parse(time.RFC3339, conn.RateLimitedUntil); err == nil && until.After(now) {
				inCooldown = true
			}
		}
		if inCooldown {
			totals["cooldown"]++
			provider["cooldown"] = provider["cooldown"].(int) + 1
			cooldowns = append(cooldowns, map[string]interface{}{
				"id":               conn.ID,
				"provider":         conn.Provider,
				"name":             conn.Name,
				"rateLimitedUntil": conn.RateLimitedUntil,
				"backoffLevel":     conn.BackoffLevel,
				"errorCode":        conn.ErrorCode,
				"lastError":        conn.LastError,
				"circuitOpenUntil": conn.CircuitOpenUntil,
			})
			continue
		}
		if conn.CircuitOpenUntil != "" {
			if until, err := time.Parse(time.RFC3339, conn.CircuitOpenUntil); err == nil && until.After(now) {
				totals["unavailable"]++
				provider["unavailable"] = provider["unavailable"].(int) + 1
				cooldowns = append(cooldowns, map[string]interface{}{
					"id":               conn.ID,
					"provider":         conn.Provider,
					"name":             conn.Name,
					"rateLimitedUntil": conn.RateLimitedUntil,
					"backoffLevel":     conn.BackoffLevel,
					"errorCode":        conn.ErrorCode,
					"lastError":        conn.LastError,
					"circuitOpenUntil": conn.CircuitOpenUntil,
				})
				continue
			}
		}
		if conn.TestStatus == "unavailable" {
			totals["unavailable"]++
			provider["unavailable"] = provider["unavailable"].(int) + 1
			continue
		}
		totals["active"]++
		provider["active"] = provider["active"].(int) + 1
	}

	status := "healthy"
	if totals["active"] == 0 && totals["connections"] > 0 {
		status = "unavailable"
	} else if totals["cooldown"] > 0 || totals["unavailable"] > 0 || totals["inactive"] > 0 {
		status = "degraded"
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return map[string]interface{}{
		"name":        "xrouter",
		"status":      status,
		"generatedAt": now.Format(time.RFC3339),
		"uptimeSec":   int(time.Since(s.startedAt).Seconds()),
		"totals":      totals,
		"providers":   providers,
		"cooldowns":   cooldowns,
		"runtime": map[string]interface{}{
			"goVersion":  runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"heapAlloc":  mem.HeapAlloc,
			"heapInuse":  mem.HeapInuse,
		},
	}
}

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, s.store.GetUsageSummary())
}

func (s *Server) handleUsageLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	logs := s.store.GetRequestLogs(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": logs,
		"count": len(logs),
	})
}

func (s *Server) handleDebugDB(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.DBSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
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
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	resp, err := s.forwarder.Forward(r.Context(), apiKey.ID, r.URL.Path, body)
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

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
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
	var body proxy.SearchRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	result, err := s.forwarder.Search(r.Context(), body)
	if err != nil {
		_ = s.store.RecordRequestLog(store.RequestLog{
			Path:       r.URL.Path,
			APIKeyID:   apiKey.ID,
			StatusCode: http.StatusBadGateway,
			LatencyMs:  time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordRequestLog(store.RequestLog{
		Path:       r.URL.Path,
		Provider:   result.Provider,
		APIKeyID:   apiKey.ID,
		StatusCode: http.StatusOK,
		LatencyMs:  time.Since(start).Milliseconds(),
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleMediaProxy(w http.ResponseWriter, r *http.Request) {
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
	body, err := io.ReadAll(io.LimitReader(r.Body, 24*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	providerHint := ""
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") || r.URL.Path == "/v1/embeddings" || r.URL.Path == "/v1/audio/speech" {
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err == nil {
			if v, ok := payload["provider"].(string); ok {
				providerHint = strings.TrimSpace(v)
				delete(payload, "provider")
			}
			if providerHint == "" {
				if model, ok := payload["model"].(string); ok && strings.Contains(model, "/") {
					parts := strings.SplitN(model, "/", 2)
					providerHint = strings.TrimSpace(parts[0])
					payload["model"] = strings.TrimSpace(parts[1])
				}
			}
			if nextBody, err := json.Marshal(payload); err == nil {
				body = nextBody
			}
		}
	}
	resp, err := s.forwarder.ForwardMedia(r.Context(), proxy.MediaRequest{
		Path:     r.URL.Path,
		Body:     body,
		Provider: providerHint,
		Headers:  r.Header.Clone(),
	})
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
	for _, k := range []string{"Content-Type", "Cache-Control", "X-Request-Id", "X-XRouter-Provider"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	rawResp, readErr := io.ReadAll(io.LimitReader(resp.Body, 12*1024*1024))
	if readErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(rawResp)
	_ = s.store.RecordRequestLog(store.RequestLog{
		Path:          r.URL.Path,
		Provider:      strings.TrimSpace(resp.Header.Get("X-XRouter-Provider")),
		APIKeyID:      apiKey.ID,
		StatusCode:    resp.StatusCode,
		LatencyMs:     time.Since(start).Milliseconds(),
		RequestBytes:  len(body),
		ResponseBytes: len(rawResp),
	})
}

func asInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func extractUsageEntry(raw []byte, provider, model string) (store.UsageEntry, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return store.UsageEntry{}, false
	}
	usageRaw, ok := payload["usage"].(map[string]interface{})
	if !ok {
		return store.UsageEntry{}, false
	}
	entry := store.UsageEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Provider:  provider,
		Model:     model,
		TotalCost: 0,
	}
	if v, ok := asInt64(usageRaw["prompt_tokens"]); ok {
		entry.PromptTokens = v
	}
	if v, ok := asInt64(usageRaw["input_tokens"]); ok && entry.PromptTokens == 0 {
		entry.PromptTokens = v
	}
	if v, ok := asInt64(usageRaw["completion_tokens"]); ok {
		entry.CompletionTokens = v
	}
	if v, ok := asInt64(usageRaw["output_tokens"]); ok && entry.CompletionTokens == 0 {
		entry.CompletionTokens = v
	}
	if v, ok := asInt64(usageRaw["total_tokens"]); ok {
		if entry.PromptTokens == 0 && entry.CompletionTokens == 0 {
			entry.PromptTokens = v
		}
	}
	return entry, true
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(data)
}
