package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"slices"
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
}

type rateBucket struct {
	windowStart time.Time
	count       int
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
	s := &Server{store: st, forwarder: proxy.NewForwarder(st), mux: http.NewServeMux(), startedAt: time.Now(), limits: map[string]*rateBucket{}}
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
	s.mux.HandleFunc("/api/keys", s.handleAPIKeys)
	s.mux.HandleFunc("/api/keys/", s.handleAPIKeyByID)
	s.mux.HandleFunc("/api/models", s.handleModels)
	s.mux.HandleFunc("/api/management/model-mappings", s.handleManagementModelMappings)
	s.mux.HandleFunc("/api/quota", s.handleQuota)
	s.mux.HandleFunc("/api/usage/summary", s.handleQuota)
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
	id := strings.TrimPrefix(r.URL.Path, "/api/providers/")
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
	if usageEntry, ok := extractUsageEntry(rawResp, provider, model); ok {
		_ = s.store.RecordUsage(usageEntry)
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	_ = apiKey
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleMediaProxy(w http.ResponseWriter, r *http.Request) {
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
	_ = apiKey
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
