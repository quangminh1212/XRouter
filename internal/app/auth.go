package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"xrouter/internal/store"
)

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
