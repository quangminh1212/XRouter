package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func resetKiloFreeModelsCache() {
	kiloFreeModelsCache.mu.Lock()
	defer kiloFreeModelsCache.mu.Unlock()
	kiloFreeModelsCache.models = nil
	kiloFreeModelsCache.updatedAt = time.Time{}
}

func TestProviderKiloFreeModelsFetchAndCache(t *testing.T) {
	resetKiloFreeModelsCache()
	srv := newTestServer(t)
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []map[string]interface{}{
			{"id": "pro", "name": "Pro", "isFree": false, "context_length": 111},
			{"id": "free-a", "name": "Free A", "isFree": true, "context_length": 12345},
		}})
	}))
	defer upstream.Close()

	path := "/api/providers/kilo/free-models?url=" + url.QueryEscape(upstream.URL)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Models []map[string]interface{} `json:"models"`
			Cached bool                     `json:"cached"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.Models) != 1 || payload.Models[0]["id"] != "free-a" {
			t.Fatalf("unexpected payload: %#v", payload.Models)
		}
		if i == 0 && payload.Cached {
			t.Fatalf("first response should not be cached")
		}
		if i == 1 && !payload.Cached {
			t.Fatalf("second response should be cached")
		}
	}
	if hits != 1 {
		t.Fatalf("expected single upstream hit, got %d", hits)
	}
}

func TestProviderKiloFreeModelsFallbackToCache(t *testing.T) {
	resetKiloFreeModelsCache()
	kiloFreeModelsCache.mu.Lock()
	kiloFreeModelsCache.models = []map[string]interface{}{{"id": "cached-free", "name": "Cached", "isFree": true, "context_length": float64(7)}}
	kiloFreeModelsCache.updatedAt = time.Now().Add(-2 * kiloFreeModelsCacheTTL)
	kiloFreeModelsCache.mu.Unlock()
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/kilo/free-models?url=http://127.0.0.1:1", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Models  []map[string]interface{} `json:"models"`
		Cached  bool                     `json:"cached"`
		Warning string                   `json:"warning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Cached || len(payload.Models) != 1 || payload.Models[0]["id"] != "cached-free" || payload.Warning == "" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestProviderKiloFreeModelsFailureWithoutCache(t *testing.T) {
	resetKiloFreeModelsCache()
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers/kilo/free-models?url=http://127.0.0.1:1", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rec.Code, rec.Body.String())
	}
}
