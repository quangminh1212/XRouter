package app

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
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

func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"servers": s.store.ListMCPServers(false)})
}

func (s *Server) handleA2AAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"agents": s.store.ListA2AAgents(false)})
}

func (s *Server) handleTunnels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tunnels": s.store.ListTunnelEndpoints(false)})
}

func (s *Server) handleCLIConfig(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cli config api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	tool := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tool")))
	if tool == "" {
		tool = "generic"
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	apiKeys := s.store.GetAPIKeysRaw()
	apiKeyValue := ""
	apiKeyName := ""
	if len(apiKeys) > 0 {
		apiKeyValue = apiKeys[0].Key
		apiKeyName = apiKeys[0].Name
	}
	model := ""
	provider := ""
	for _, conn := range s.store.GetAllConnectionsRaw() {
		if !conn.IsActive {
			continue
		}
		if model == "" && strings.TrimSpace(conn.DefaultModel) != "" {
			model = strings.TrimSpace(conn.DefaultModel)
			provider = conn.Provider
			break
		}
	}
	if model == "" {
		fallbacks := store.GetFallbackModels()
		if len(fallbacks) > 0 {
			model = fallbacks[0]["fullModel"]
		}
	}
	response := map[string]interface{}{
		"tool":         tool,
		"baseUrl":      baseURL,
		"chatPath":     "/v1/chat/completions",
		"modelsPath":   "/api/models",
		"apiKeyName":   apiKeyName,
		"apiKeyValue":  apiKeyValue,
		"defaultModel": model,
		"provider":     provider,
		"headers": map[string]string{
			"Authorization": "Bearer " + apiKeyValue,
			"Content-Type":  "application/json",
		},
		"env": map[string]string{
			"OPENAI_BASE_URL": baseURL + "/v1",
			"OPENAI_API_KEY":  apiKeyValue,
			"OPENAI_MODEL":    model,
		},
		"examples": map[string]string{
			"curl": fmt.Sprintf(`curl %s/v1/chat/completions -H "Authorization: Bearer %s" -H "Content-Type: application/json" -d "{\"model\":\"%s\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"`, baseURL, apiKeyValue, model),
		},
	}
	switch tool {
	case "openai", "openai-compatible", "generic":
	default:
		response["note"] = "tool preset not specialized; returned generic OpenAI-compatible config"
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleManagementMCPServers(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"servers": s.store.ListMCPServers(true)})
	case http.MethodPost:
		var body store.MCPServer
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		created, err := s.store.CreateMCPServer(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		created.Env = nil
		created.Headers = nil
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementA2AAgents(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"agents": s.store.ListA2AAgents(true)})
	case http.MethodPost:
		var body store.A2AAgent
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		if strings.TrimSpace(body.URL) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
			return
		}
		created, err := s.store.CreateA2AAgent(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		created.Env = nil
		created.Headers = nil
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementTunnels(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"tunnels": s.store.ListTunnelEndpoints(true)})
	case http.MethodPost:
		var body store.TunnelEndpoint
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		if strings.TrimSpace(body.PublicURL) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "publicUrl is required"})
			return
		}
		created, err := s.store.CreateTunnelEndpoint(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementMCPServerByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/management/mcp-servers/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing mcp server id"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateMCPServer(id, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		updated.Env = nil
		updated.Headers = nil
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteMCPServer(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementA2AAgentByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/management/a2a-agents/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing a2a agent id"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateA2AAgent(id, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		updated.Env = nil
		updated.Headers = nil
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteA2AAgent(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementTunnelByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/management/tunnels/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing tunnel id"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateTunnelEndpoint(id, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteTunnelEndpoint(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
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

func (s *Server) handleManagementModelAliases(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"aliases": s.store.GetModelAliases()})
	case http.MethodPut:
		var body struct {
			Aliases map[string]string `json:"aliases"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		aliases, err := s.store.ReplaceModelAliases(body.Aliases)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "aliases": aliases})
	case http.MethodPatch:
		var body struct {
			Aliases map[string]string `json:"aliases"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		aliases, err := s.store.PatchModelAliases(body.Aliases)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "aliases": aliases})
	case http.MethodDelete:
		var body struct {
			Models []string `json:"models"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		aliases, err := s.store.DeleteModelAliasKeys(body.Models)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "aliases": aliases})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementDisabledModels(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": s.store.GetDisabledModels()})
	case http.MethodPut:
		var body struct {
			Models []string `json:"models"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		models, err := s.store.ReplaceDisabledModels(body.Models)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "models": models})
	case http.MethodPatch:
		var body struct {
			Models []string `json:"models"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		models, err := s.store.PatchDisabledModels(body.Models)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "models": models})
	case http.MethodDelete:
		var body struct {
			Models []string `json:"models"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		models, err := s.store.DeleteDisabledModels(body.Models)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "models": models})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementModelAvailability(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"availability": s.store.GetModelAvailability()})
	case http.MethodPut:
		var body struct {
			Availability map[string]string `json:"availability"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		availability, err := s.store.ReplaceModelAvailability(body.Availability)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "availability": availability})
	case http.MethodPatch:
		var body struct {
			Availability map[string]string `json:"availability"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		availability, err := s.store.PatchModelAvailability(body.Availability)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "availability": availability})
	case http.MethodDelete:
		var body struct {
			Models []string `json:"models"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		availability, err := s.store.DeleteModelAvailabilityKeys(body.Models)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "availability": availability})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func validRoutingStrategy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fallback", "round_robin", "sticky_round_robin":
		return true
	default:
		return false
	}
}

func (s *Server) handleManagementRoutingStrategy(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings := s.store.GetSettings()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"comboStrategy":              settings.ComboStrategy,
			"stickyRoundRobinLimit":      settings.StickyRoundRobinLimit,
			"comboStickyRoundRobinLimit": settings.ComboStickyRoundRobinLimit,
		})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			ComboStrategy              string `json:"comboStrategy"`
			StickyRoundRobinLimit      *int   `json:"stickyRoundRobinLimit"`
			ComboStickyRoundRobinLimit *int   `json:"comboStickyRoundRobinLimit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.ComboStrategy) != "" && !validRoutingStrategy(body.ComboStrategy) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid comboStrategy"})
			return
		}
		patch := map[string]interface{}{}
		if strings.TrimSpace(body.ComboStrategy) != "" {
			patch["comboStrategy"] = strings.ToLower(strings.TrimSpace(body.ComboStrategy))
		}
		if body.StickyRoundRobinLimit != nil {
			if *body.StickyRoundRobinLimit < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stickyRoundRobinLimit must be >= 0"})
				return
			}
			patch["stickyRoundRobinLimit"] = *body.StickyRoundRobinLimit
		}
		if body.ComboStickyRoundRobinLimit != nil {
			if *body.ComboStickyRoundRobinLimit < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "comboStickyRoundRobinLimit must be >= 0"})
				return
			}
			patch["comboStickyRoundRobinLimit"] = *body.ComboStickyRoundRobinLimit
		}
		settings, err := s.store.UpdateSettings(patch)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"comboStrategy":              settings.ComboStrategy,
			"stickyRoundRobinLimit":      settings.StickyRoundRobinLimit,
			"comboStickyRoundRobinLimit": settings.ComboStickyRoundRobinLimit,
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRetryConfig(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings := s.store.GetSettings()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"maxRetries":         settings.MaxRetries,
			"maxCooldownSeconds": settings.MaxCooldownSeconds,
		})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			MaxRetries         *int `json:"maxRetries"`
			MaxCooldownSeconds *int `json:"maxCooldownSeconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		patch := map[string]interface{}{}
		if body.MaxRetries != nil {
			if *body.MaxRetries < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maxRetries must be >= 0"})
				return
			}
			patch["maxRetries"] = *body.MaxRetries
		}
		if body.MaxCooldownSeconds != nil {
			if *body.MaxCooldownSeconds < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maxCooldownSeconds must be >= 0"})
				return
			}
			patch["maxCooldownSeconds"] = *body.MaxCooldownSeconds
		}
		settings, err := s.store.UpdateSettings(patch)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"maxRetries":         settings.MaxRetries,
			"maxCooldownSeconds": settings.MaxCooldownSeconds,
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementProxyPools(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"pools": s.store.ListProxyPools()})
	case http.MethodPost:
		var body store.ProxyPool
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		created, err := s.store.CreateProxyPool(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementProxyPoolByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/management/proxy-pools/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing proxy pool id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, ok := s.store.GetProxyPool(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "proxy pool not found"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateProxyPool(id, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteProxyPool(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementProviderNodes(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"nodes": s.store.ListProviderNodes()})
	case http.MethodPost:
		var body store.ProviderNode
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		created, err := s.store.CreateProviderNode(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementProviderNodeByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/management/provider-nodes/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing provider node id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, ok := s.store.GetProviderNode(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider node not found"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateProviderNode(id, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteProviderNode(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementComboModels(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"comboModels": s.store.ListComboModels()})
	case http.MethodPost:
		var body store.ComboModel
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Alias) == "" || len(body.Targets) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "alias and targets are required"})
			return
		}
		created, err := s.store.CreateComboModel(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementComboModelByAlias(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	alias := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/management/combo-models/"), "/")
	if alias == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing combo model alias"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateComboModel(alias, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteComboModel(alias); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRoutePolicies(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"policies": s.store.ListRoutePolicies()})
	case http.MethodPost:
		var body store.RoutePolicy
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		created, err := s.store.CreateRoutePolicy(body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRoutePolicyByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/management/route-policies/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing route policy id"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var patch map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		updated, err := s.store.UpdateRoutePolicy(id, patch)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteRoutePolicy(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
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
	if r.URL.Path == "/v1/responses/stream" {
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

func isBlockedFetchHost(host string) bool {
	if os.Getenv("XR_ALLOW_PRIVATE_FETCH") == "1" {
		return false
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return true
	}
	name := strings.ToLower(host)
	if name == "localhost" || strings.HasSuffix(name, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return true
		}
		for _, item := range ips {
			if item.IsLoopback() || item.IsPrivate() || item.IsLinkLocalMulticast() || item.IsLinkLocalUnicast() {
				return true
			}
		}
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast()
}

func (s *Server) handleWebFetch(w http.ResponseWriter, r *http.Request) {
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
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	targetURL, err := url.Parse(strings.TrimSpace(body.URL))
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid url"})
		return
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported url scheme"})
		return
	}
	host := targetURL.Hostname()
	if isBlockedFetchHost(host) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target host is blocked"})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL.String(), nil)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to build request"})
		return
	}
	req.Header.Set("User-Agent", "xrouter-fetch/1.0")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
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
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	_ = s.store.RecordRequestLog(store.RequestLog{
		Path:          r.URL.Path,
		APIKeyID:      apiKey.ID,
		StatusCode:    http.StatusOK,
		LatencyMs:     time.Since(start).Milliseconds(),
		ResponseBytes: len(raw),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"url":         targetURL.String(),
		"status":      resp.StatusCode,
		"contentType": contentType,
		"content":     string(raw),
	})
}

func (s *Server) handleAudioVoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	voices := []map[string]string{
		{"id": "alloy", "provider": "openai-tts", "language": "multilingual"},
		{"id": "ash", "provider": "openai-tts", "language": "multilingual"},
		{"id": "ballad", "provider": "openai-tts", "language": "multilingual"},
		{"id": "coral", "provider": "openai-tts", "language": "multilingual"},
		{"id": "echo", "provider": "openai-tts", "language": "multilingual"},
		{"id": "fable", "provider": "openai-tts", "language": "multilingual"},
		{"id": "nova", "provider": "openai-tts", "language": "multilingual"},
		{"id": "onyx", "provider": "openai-tts", "language": "multilingual"},
		{"id": "sage", "provider": "openai-tts", "language": "multilingual"},
		{"id": "shimmer", "provider": "openai-tts", "language": "multilingual"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"voices": voices})
}

func geminiContentsToMessages(contents []map[string]interface{}) []map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(contents))
	for _, item := range contents {
		role := "user"
		if v, ok := item["role"].(string); ok && strings.TrimSpace(v) != "" {
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "model", "assistant":
				role = "assistant"
			default:
				role = "user"
			}
		}
		parts, _ := item["parts"].([]interface{})
		contentParts := make([]interface{}, 0, len(parts))
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]interface{})
			if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
				contentParts = append(contentParts, map[string]interface{}{"type": "text", "text": text})
			}
		}
		if len(contentParts) == 0 {
			continue
		}
		messages = append(messages, map[string]interface{}{"role": role, "content": contentParts})
	}
	return messages
}

func openAIResponseToGemini(raw map[string]interface{}) map[string]interface{} {
	text := ""
	if choices, ok := raw["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					text = content
				}
			}
		}
	}
	return map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"role":  "model",
					"parts": []map[string]string{{"text": text}},
				},
				"finishReason": "STOP",
			},
		},
	}
}

func (s *Server) handleGeminiAction(w http.ResponseWriter, r *http.Request) {
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
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1beta/models/")
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid gemini action path"})
		return
	}
	modelName := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	var input map[string]interface{}
	if err := json.Unmarshal(raw, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	contentItems := []map[string]interface{}{}
	if items, ok := input["contents"].([]interface{}); ok {
		for _, item := range items {
			if mapped, ok := item.(map[string]interface{}); ok {
				contentItems = append(contentItems, mapped)
			}
		}
	}
	converted := map[string]interface{}{
		"model":    "gemini/" + modelName,
		"messages": geminiContentsToMessages(contentItems),
	}
	if cfg, ok := input["generationConfig"].(map[string]interface{}); ok {
		if v, ok := cfg["maxOutputTokens"]; ok {
			converted["max_tokens"] = v
		}
		if v, ok := cfg["temperature"]; ok {
			converted["temperature"] = v
		}
	}
	if action == "streamGenerateContent" {
		converted["stream"] = true
	}
	body, _ := json.Marshal(converted)
	resp, err := s.forwarder.Forward(r.Context(), apiKey.ID, "/v1/chat/completions", body)
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
	if action == "streamGenerateContent" {
		for _, k := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
			if v := resp.Header.Get(k); v != "" {
				w.Header().Set(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(responseBody)
		return
	}
	var openAIResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &openAIResponse); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid upstream response"})
		return
	}
	writeJSON(w, http.StatusOK, openAIResponseToGemini(openAIResponse))
}

func resolveConnectionBaseURL(conn store.ProviderConnection) (string, bool) {
	if conn.ProviderSpecificData != nil {
		if v, ok := conn.ProviderSpecificData["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimRight(strings.TrimSpace(v), "/"), true
		}
	}
	entry, ok := store.GetProviderCatalogEntry(conn.Provider)
	if !ok {
		return "", false
	}
	return strings.TrimRight(strings.TrimSpace(entry.BaseURL), "/"), true
}

func (s *Server) handleVideoByID(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodGet {
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
	videoID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/videos/"), "/")
	if videoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing video id"})
		return
	}
	connections := s.store.GetActiveConnections("")
	var targetConn *store.ProviderConnection
	for i := range connections {
		conn := &connections[i]
		entry, ok := store.GetProviderCatalogEntry(conn.Provider)
		if !ok {
			continue
		}
		if entry.APIType == "video" || entry.APIType == "openai" {
			targetConn = conn
			break
		}
	}
	if targetConn == nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "no active video provider connections"})
		return
	}
	baseURL, ok := resolveConnectionBaseURL(*targetConn)
	if !ok || baseURL == "" {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "video provider base url is missing"})
		return
	}
	upstreamURL := baseURL + "/v1/videos/" + url.PathEscape(videoID)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to build upstream request"})
		return
	}
	if strings.TrimSpace(targetConn.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(targetConn.APIKey))
	} else if strings.TrimSpace(targetConn.AccessToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(targetConn.AccessToken))
	}
	resp, err := s.forwarder.HTTPClient().Do(req)
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
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read upstream response"})
		return
	}
	_ = s.store.RecordRequestLog(store.RequestLog{
		Path:          r.URL.Path,
		APIKeyID:      apiKey.ID,
		StatusCode:    resp.StatusCode,
		LatencyMs:     time.Since(start).Milliseconds(),
		ResponseBytes: len(body),
	})
	for _, k := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
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
