package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"xrouter/internal/store"
)

func (s *Server) handleAmpCodeRoot(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ampcode": s.store.GetSettings().AmpCode})
}

func (s *Server) updateAmpCode(mut func(amp *store.AmpCode)) (store.AmpCode, error) {
	current := s.store.GetSettings().AmpCode
	mut(&current)
	settings, err := s.store.UpdateSettings(map[string]interface{}{"ampcode": current})
	if err != nil {
		return store.AmpCode{}, err
	}
	return settings.AmpCode, nil
}

func (s *Server) handleAmpUpstreamURL(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"upstream-url": s.store.GetSettings().AmpCode.UpstreamURL})
	case http.MethodPut:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.UpstreamURL = strings.TrimSpace(body.Value)
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"upstream-url": amp.UpstreamURL})
	case http.MethodDelete:
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.UpstreamURL = ""
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"upstream-url": amp.UpstreamURL})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAmpUpstreamAPIKey(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"upstream-api-key": s.store.GetSettings().AmpCode.UpstreamAPIKey})
	case http.MethodPut:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.UpstreamAPIKey = strings.TrimSpace(body.Value)
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"upstream-api-key": amp.UpstreamAPIKey})
	case http.MethodDelete:
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.UpstreamAPIKey = ""
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"upstream-api-key": amp.UpstreamAPIKey})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func sanitizeAmpUpstreamKeys(items []store.AmpUpstreamKey) []store.AmpUpstreamKey {
	cleaned := make([]store.AmpUpstreamKey, 0, len(items))
	for _, item := range items {
		entry := store.AmpUpstreamKey{UpstreamAPIKey: strings.TrimSpace(item.UpstreamAPIKey)}
		for _, key := range item.APIKeys {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			entry.APIKeys = append(entry.APIKeys, trimmed)
		}
		cleaned = append(cleaned, entry)
	}
	return cleaned
}

func (s *Server) handleAmpUpstreamAPIKeys(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"upstream-api-keys": s.store.GetSettings().AmpCode.UpstreamAPIKeys})
	case http.MethodPut:
		var body struct {
			Value []store.AmpUpstreamKey `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.UpstreamAPIKeys = sanitizeAmpUpstreamKeys(body.Value)
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"upstream-api-keys": amp.UpstreamAPIKeys})
	case http.MethodPatch:
		var body struct {
			Value []store.AmpUpstreamKey `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			merged := append([]store.AmpUpstreamKey{}, amp.UpstreamAPIKeys...)
			for _, item := range sanitizeAmpUpstreamKeys(body.Value) {
				replaced := false
				for i, existing := range merged {
					if existing.UpstreamAPIKey == item.UpstreamAPIKey {
						merged[i] = item
						replaced = true
						break
					}
				}
				if !replaced {
					merged = append(merged, item)
				}
			}
			amp.UpstreamAPIKeys = merged
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"upstream-api-keys": amp.UpstreamAPIKeys})
	case http.MethodDelete:
		var body struct {
			Value []string `json:"value"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			if len(body.Value) == 0 {
				amp.UpstreamAPIKeys = nil
				return
			}
			remove := map[string]struct{}{}
			for _, key := range body.Value {
				remove[strings.TrimSpace(key)] = struct{}{}
			}
			kept := make([]store.AmpUpstreamKey, 0, len(amp.UpstreamAPIKeys))
			for _, item := range amp.UpstreamAPIKeys {
				if _, drop := remove[item.UpstreamAPIKey]; drop {
					continue
				}
				kept = append(kept, item)
			}
			amp.UpstreamAPIKeys = kept
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"upstream-api-keys": amp.UpstreamAPIKeys})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAmpRestrictManagementToLocalhost(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"restrict-management-to-localhost": s.store.GetSettings().AmpCode.RestrictManagementToLocalhost})
	case http.MethodPut:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.RestrictManagementToLocalhost = body.Value
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"restrict-management-to-localhost": amp.RestrictManagementToLocalhost})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAmpForceModelMappings(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"force-model-mappings": s.store.GetSettings().AmpCode.ForceModelMappings})
	case http.MethodPut:
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.ForceModelMappings = body.Value
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"force-model-mappings": amp.ForceModelMappings})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAmpModelMappings(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"model-mappings": s.store.GetSettings().AmpCode.ModelMappings})
	case http.MethodPut:
		var body struct {
			Value []store.AmpModelMapping `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			amp.ModelMappings = sanitizeAmpModelMappings(body.Value)
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"model-mappings": amp.ModelMappings})
	case http.MethodPatch:
		var body struct {
			Value []store.AmpModelMapping `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			merged := append([]store.AmpModelMapping{}, amp.ModelMappings...)
			for _, item := range sanitizeAmpModelMappings(body.Value) {
				replaced := false
				for i, existing := range merged {
					if existing.From == item.From {
						merged[i] = item
						replaced = true
						break
					}
				}
				if !replaced {
					merged = append(merged, item)
				}
			}
			amp.ModelMappings = merged
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"model-mappings": amp.ModelMappings})
	case http.MethodDelete:
		var body struct {
			Value []string `json:"value"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		amp, err := s.updateAmpCode(func(amp *store.AmpCode) {
			if len(body.Value) == 0 {
				amp.ModelMappings = nil
				return
			}
			drop := map[string]struct{}{}
			for _, k := range body.Value {
				drop[strings.TrimSpace(k)] = struct{}{}
			}
			kept := make([]store.AmpModelMapping, 0, len(amp.ModelMappings))
			for _, m := range amp.ModelMappings {
				if _, skip := drop[m.From]; skip {
					continue
				}
				kept = append(kept, m)
			}
			amp.ModelMappings = kept
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"model-mappings": amp.ModelMappings})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func sanitizeAmpModelMappings(items []store.AmpModelMapping) []store.AmpModelMapping {
	cleaned := make([]store.AmpModelMapping, 0, len(items))
	for _, item := range items {
		from := strings.TrimSpace(item.From)
		to := strings.TrimSpace(item.To)
		if from == "" {
			continue
		}
		cleaned = append(cleaned, store.AmpModelMapping{From: from, To: to})
	}
	return cleaned
}
