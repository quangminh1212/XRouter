package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"xrouter/internal/store"
)

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
	id := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/management"), "/api/proxy-pools/"), "/")
	id = strings.TrimPrefix(id, "proxy-pools/")
	id = strings.TrimSpace(id)
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
