package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"xrouter/internal/proxy"
	"xrouter/internal/store"
)

type Server struct {
	store      *store.Store
	forwarder  *proxy.Forwarder
	mux        *http.ServeMux
	startedAt  time.Time
	limits     map[string]*rateBucket
	limitMu    sync.Mutex
	oauthMu    sync.Mutex
	oauth      map[string]oauthSession
	cloudMu    sync.Mutex
	cloudTasks map[string]map[string]interface{}
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
	s := &Server{store: st, forwarder: proxy.NewForwarder(st), mux: http.NewServeMux(), startedAt: time.Now(), limits: map[string]*rateBucket{}, oauth: map[string]oauthSession{}, cloudTasks: map[string]map[string]interface{}{}}
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
		if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&patch); err != nil {
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
