package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	id := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/management"), "/api/provider-nodes/"), "/")
	id = strings.TrimPrefix(id, "provider-nodes/")
	id = strings.TrimSpace(id)
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
	alias := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/management"), "/api/combos/"), "/")
	alias = strings.TrimPrefix(alias, "combo-models/")
	alias = strings.TrimSpace(alias)
	if alias == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing combo model alias"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		for _, item := range s.store.ListComboModels() {
			if item.Alias == alias {
				writeJSON(w, http.StatusOK, item)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "combo model not found"})
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

func (s *Server) handleManagementDebug(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"debug": s.store.GetSettings().Debug})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"debug": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"debug": settings.Debug})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRequestLog(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"request-log": s.store.GetSettings().RequestLog})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"requestLog": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"request-log": settings.RequestLog})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementUsageQueue(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	count := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("count")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			count = parsed
		}
	}
	logs := s.store.GetRequestLogs(count)
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": logs, "count": len(logs)})
}

func (s *Server) handleManagementProxyURL(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"proxy-url": s.store.GetSettings().OutboundProxyURL})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		value := strings.TrimSpace(body.Value)
		settings, err := s.store.UpdateSettings(map[string]interface{}{"outboundProxyUrl": value, "outboundProxyEnabled": value != ""})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy-url": settings.OutboundProxyURL})
	case http.MethodDelete:
		settings, err := s.store.UpdateSettings(map[string]interface{}{"outboundProxyUrl": "", "outboundProxyEnabled": false})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy-url": settings.OutboundProxyURL})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementRequestRetry(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]int{"request-retry": s.store.GetSettings().MaxRetries})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value int `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Value < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "value must be >= 0"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"maxRetries": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"request-retry": settings.MaxRetries})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementMaxRetryInterval(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]int{"max-retry-interval": s.store.GetSettings().MaxCooldownSeconds})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value int `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Value < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "value must be >= 0"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"maxCooldownSeconds": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"max-retry-interval": settings.MaxCooldownSeconds})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func cliProxyRoutingStrategy(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "round-robin", "roundrobin", "rr", "round_robin":
		return "round_robin", true
	case "fill-first", "fillfirst", "ff", "fallback":
		return "fallback", true
	default:
		return "", false
	}
}

func (s *Server) handleManagementRoutingStrategyAlias(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		strategy := s.store.GetSettings().ComboStrategy
		if strategy == "round_robin" {
			strategy = "round-robin"
		} else if strategy == "fallback" || strategy == "" {
			strategy = "fill-first"
		}
		writeJSON(w, http.StatusOK, map[string]string{"strategy": strategy})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		strategy, ok := cliProxyRoutingStrategy(body.Value)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid strategy"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"comboStrategy": strategy})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := settings.ComboStrategy
		if out == "round_robin" {
			out = "round-robin"
		} else if out == "fallback" || out == "" {
			out = "fill-first"
		}
		writeJSON(w, http.StatusOK, map[string]string{"strategy": out})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleManagementForceModelPrefix(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"force-model-prefix": s.store.GetSettings().ForceModelPrefix})
	case http.MethodPut, http.MethodPatch:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		settings, err := s.store.UpdateSettings(map[string]interface{}{"forceModelPrefix": body.Value})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"force-model-prefix": settings.ForceModelPrefix})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

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
