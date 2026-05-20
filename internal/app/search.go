package app

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"xrouter/internal/proxy"
	"xrouter/internal/store"
)

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
