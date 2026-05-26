package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementRoutingStrategyEndpointUpdatesSettings(t *testing.T) {
	srv := newTestServer(t)

	body := bytes.NewBufferString(`{"comboStrategy":"sticky_round_robin","stickyRoundRobinLimit":5,"comboStickyRoundRobinLimit":2}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/management/routing-strategy", body)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		ComboStrategy              string `json:"comboStrategy"`
		StickyRoundRobinLimit      int    `json:"stickyRoundRobinLimit"`
		ComboStickyRoundRobinLimit int    `json:"comboStickyRoundRobinLimit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.ComboStrategy != "sticky_round_robin" || payload.StickyRoundRobinLimit != 5 || payload.ComboStickyRoundRobinLimit != 2 {
		t.Fatalf("unexpected routing strategy payload: %#v", payload)
	}

	settings := srv.store.GetSettings()
	if settings.ComboStrategy != "sticky_round_robin" || settings.StickyRoundRobinLimit != 5 || settings.ComboStickyRoundRobinLimit != 2 {
		t.Fatalf("settings not updated: %#v", settings)
	}
}

func TestManagementRoutingStrategyRejectsInvalidStrategy(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/management/routing-strategy", bytes.NewBufferString(`{"comboStrategy":"bad"}`))
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestManagementRoutingStrategyAcceptsAutoAndCostOptimized(t *testing.T) {
	srv := newTestServer(t)
	for _, strategy := range []string{"auto", "cost_optimized", "last_known_good"} {
		t.Run(strategy, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/api/management/routing-strategy", bytes.NewBufferString(`{"comboStrategy":"`+strategy+`"}`))
			req.Host = "localhost"
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
