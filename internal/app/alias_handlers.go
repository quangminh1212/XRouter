package app

import (
	"encoding/json"
	"net/http"
	"time"

	"xrouter/internal/store"
)

func (s *Server) handleRateLimitsAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	type entry struct {
		ConnectionID string `json:"connectionId"`
		Provider     string `json:"provider"`
		Name         string `json:"name"`
		LimitedUntil string `json:"limitedUntil,omitempty"`
		CircuitUntil string `json:"circuitUntil,omitempty"`
		BackoffLevel int    `json:"backoffLevel"`
		Failures     int    `json:"consecutiveFailures"`
	}
	conns := s.store.GetAllConnectionsRaw()
	items := make([]entry, 0, len(conns))
	for _, c := range conns {
		if c.RateLimitedUntil == "" && c.CircuitOpenUntil == "" && c.BackoffLevel == 0 {
			continue
		}
		items = append(items, entry{
			ConnectionID: c.ID,
			Provider:     c.Provider,
			Name:         c.Name,
			LimitedUntil: c.RateLimitedUntil,
			CircuitUntil: c.CircuitOpenUntil,
			BackoffLevel: c.BackoffLevel,
			Failures:     c.ConsecutiveFailures,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"rateLimits": items, "count": len(items)})
}

func (s *Server) handleCacheStatsAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "cache cleared"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cacheEnabled": false,
		"entries":      0,
		"hitRate":      0,
		"note":         "XRouter uses in-process dedup; no persistent cache layer",
	})
}

func (s *Server) handlePricingAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	type priceEntry struct {
		Model           string  `json:"model"`
		Provider        string  `json:"provider"`
		InputPerMToken  float64 `json:"inputPerMToken"`
		OutputPerMToken float64 `json:"outputPerMToken"`
	}
	items := []priceEntry{}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pricing": items,
		"count":   len(items),
		"note":    "pricing table is provided through usage cost estimation only",
	})
}

func (s *Server) handleFallbackChainsAlias(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := s.store.ListRoutePolicies()
		chains := make([]map[string]interface{}, 0, len(policies))
		for _, p := range policies {
			chains = append(chains, map[string]interface{}{
				"id":           p.ID,
				"name":         p.Name,
				"modelPrefix":  p.ModelPrefix,
				"providers":    p.Providers,
				"accounts":     p.Accounts,
				"targetPoolId": p.TargetPoolID,
				"targetNodeId": p.TargetNodeID,
				"forceModel":   p.ForceModel,
			})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"chains": chains, "count": len(chains)})
	case http.MethodPost:
		if !isLocalOnlyRequest(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "management api is restricted to localhost"})
			return
		}
		var body store.RoutePolicy
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
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

func (s *Server) handleTelemetrySummaryAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	summary := s.store.GetUsageSummary()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"telemetry": summary,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleUsageBudgetAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":         false,
		"monthlyLimitUSD": 0,
		"note":            "budget enforcement is not configured in current settings schema",
	})
}
