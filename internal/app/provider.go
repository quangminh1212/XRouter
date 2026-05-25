package app

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"xrouter/internal/store"
)

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"connections": s.store.GetAllConnections()})
	case http.MethodPost:
		var body store.ProviderConnection
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
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

func (s *Server) handleProviderSuggestedModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	url := strings.TrimSpace(r.URL.Query().Get("url"))
	filterType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if url == "" || filterType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing url or type"})
		return
	}
	filter, ok := suggestedModelFilters[filterType]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown filter type"})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
		return
	}
	var payload interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": filter(extractSuggestedModelList(payload))})
}

type suggestedModelFilter func([]map[string]interface{}) []map[string]interface{}

var suggestedModelFilters = map[string]suggestedModelFilter{
	"openrouter-free": filterOpenRouterFreeModels,
	"opencode-free":   filterOpenCodeFreeModels,
}

func extractSuggestedModelList(payload interface{}) []map[string]interface{} {
	if root, ok := payload.(map[string]interface{}); ok {
		if data, ok := root["data"]; ok {
			return normalizeSuggestedModelList(data)
		}
		if models, ok := root["models"]; ok {
			return normalizeSuggestedModelList(models)
		}
	}
	return normalizeSuggestedModelList(payload)
}

func normalizeSuggestedModelList(value interface{}) []map[string]interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}
	models := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		model, ok := item.(map[string]interface{})
		if ok {
			models = append(models, model)
		}
	}
	return models
}

func filterOpenRouterFreeModels(models []map[string]interface{}) []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, model := range models {
		pricing, _ := model["pricing"].(map[string]interface{})
		if pricingString(pricing["prompt"]) != "0" || pricingString(pricing["completion"]) != "0" {
			continue
		}
		contextLength, ok := numericValue(model["context_length"])
		if !ok || contextLength < 200000 {
			continue
		}
		result = append(result, map[string]interface{}{"id": stringValue(model["id"]), "name": stringValue(model["name"]), "contextLength": contextLength})
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, _ := numericValue(result[i]["contextLength"])
		right, _ := numericValue(result[j]["contextLength"])
		return left > right
	})
	return result
}

func filterOpenCodeFreeModels(models []map[string]interface{}) []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, model := range models {
		id := stringValue(model["id"])
		if strings.HasSuffix(id, "-free") {
			result = append(result, map[string]interface{}{"id": id, "name": id})
		}
	}
	return result
}

func pricingString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

func stringValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func numericValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

const kiloFreeModelsDefaultURL = "https://api.kilo.ai/api/gateway/models"
const kiloFreeModelsCacheTTL = time.Hour

type cachedKiloFreeModels struct {
	mu        sync.Mutex
	models    []map[string]interface{}
	updatedAt time.Time
}

var kiloFreeModelsCache = &cachedKiloFreeModels{}

func (s *Server) handleProviderKiloFreeModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	url := strings.TrimSpace(r.URL.Query().Get("url"))
	if url == "" {
		url = strings.TrimSpace(getenvDefault("XR_KILO_MODELS_URL", kiloFreeModelsDefaultURL))
	}
	now := time.Now()
	kiloFreeModelsCache.mu.Lock()
	if len(kiloFreeModelsCache.models) > 0 && now.Sub(kiloFreeModelsCache.updatedAt) < kiloFreeModelsCacheTTL {
		models := cloneSuggestedModels(kiloFreeModelsCache.models)
		kiloFreeModelsCache.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": models, "cached": true})
		return
	}
	kiloFreeModelsCache.mu.Unlock()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"models": []map[string]interface{}{}, "error": "invalid kilo models url"})
		return
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		handleKiloFreeModelsFallback(w, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		handleKiloFreeModelsFallback(w, "kilo api returned "+strconv.Itoa(resp.StatusCode))
		return
	}
	var payload interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&payload); err != nil {
		handleKiloFreeModelsFallback(w, "invalid kilo models response")
		return
	}
	models := extractKiloFreeModels(extractSuggestedModelList(payload))
	kiloFreeModelsCache.mu.Lock()
	kiloFreeModelsCache.models = cloneSuggestedModels(models)
	kiloFreeModelsCache.updatedAt = now
	kiloFreeModelsCache.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": models, "cached": false})
}

func handleKiloFreeModelsFallback(w http.ResponseWriter, warning string) {
	kiloFreeModelsCache.mu.Lock()
	models := cloneSuggestedModels(kiloFreeModelsCache.models)
	kiloFreeModelsCache.mu.Unlock()
	if len(models) > 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": models, "cached": true, "warning": warning})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]interface{}{"models": []map[string]interface{}{}, "error": "Failed to fetch Kilo models: " + warning})
}

func extractKiloFreeModels(models []map[string]interface{}) []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, model := range models {
		isFree, _ := model["isFree"].(bool)
		if !isFree {
			continue
		}
		contextLength, _ := numericValue(model["context_length"])
		result = append(result, map[string]interface{}{
			"id":             stringValue(model["id"]),
			"name":           stringValue(model["name"]),
			"isFree":         true,
			"context_length": contextLength,
		})
	}
	return result
}

func cloneSuggestedModels(models []map[string]interface{}) []map[string]interface{} {
	if len(models) == 0 {
		return []map[string]interface{}{}
	}
	result := make([]map[string]interface{}, 0, len(models))
	for _, model := range models {
		copyItem := make(map[string]interface{}, len(model))
		for k, v := range model {
			copyItem[k] = v
		}
		result = append(result, copyItem)
	}
	return result
}

func getenvDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func (s *Server) handleProviderTestBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		ConnectionIDs []string `json:"connectionIds"`
		IDs           []string `json:"ids"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	ids := body.ConnectionIDs
	if len(ids) == 0 {
		ids = body.IDs
	}
	connections := []store.ProviderConnection{}
	if len(ids) == 0 {
		connections = s.store.GetAllConnections()
	} else {
		seen := map[string]bool{}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			if conn, ok := s.store.GetConnectionByIDRaw(id); ok {
				connections = append(connections, conn)
			}
		}
	}
	results := make([]map[string]interface{}, 0, len(connections))
	passed := 0
	failed := 0
	for _, conn := range connections {
		probe, err := s.forwarder.ProbeConnection(r.Context(), conn)
		status := "active"
		if err != nil || !probe.Healthy {
			status = "unavailable"
			failed++
		} else {
			passed++
		}
		updated, _ := s.store.UpdateConnectionTestStatus(conn.ID, status, probe.Message, probe.StatusCode)
		updated.APIKey, updated.AccessToken, updated.RefreshToken = "", "", ""
		item := map[string]interface{}{"id": conn.ID, "provider": conn.Provider, "result": probe, "connection": updated, "success": err == nil && probe.Healthy}
		if err != nil {
			item["error"] = err.Error()
		}
		results = append(results, item)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary": map[string]int{"total": len(results), "passed": passed, "failed": failed},
		"results": results,
	})
}

func (s *Server) handleProviderMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	type metric struct {
		Provider      string  `json:"provider"`
		Requests      int64   `json:"requests"`
		Failures      int64   `json:"failures"`
		SuccessRate   float64 `json:"successRate"`
		LatencyMsAvg  int64   `json:"latencyMsAvg"`
		ResponseBytes int64   `json:"responseBytes"`
	}

	logs := s.store.GetRequestLogs(0)
	buckets := map[string]*metric{}
	latencyTotals := map[string]int64{}
	for _, item := range logs {
		provider := strings.TrimSpace(item.Provider)
		if provider == "" {
			provider = "unknown"
		}
		current := buckets[provider]
		if current == nil {
			current = &metric{Provider: provider}
			buckets[provider] = current
		}
		current.Requests++
		if item.StatusCode < 200 || item.StatusCode >= 400 {
			current.Failures++
		}
		latencyTotals[provider] += item.LatencyMs
		current.ResponseBytes += int64(item.ResponseBytes)
	}

	items := make([]metric, 0, len(buckets))
	for provider, current := range buckets {
		if current.Requests > 0 {
			current.SuccessRate = float64(current.Requests-current.Failures) / float64(current.Requests)
			current.LatencyMsAvg = latencyTotals[provider] / current.Requests
		}
		items = append(items, *current)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Requests == items[j].Requests {
			return items[i].Provider < items[j].Provider
		}
		return items[i].Requests > items[j].Requests
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(items),
		"metrics": items,
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
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&patch); err != nil {
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
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil && err != io.EOF {
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
