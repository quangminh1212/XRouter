package app

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"xrouter/internal/proxy"
	"xrouter/internal/store"
)

type Server struct {
	store     *store.Store
	forwarder *proxy.Forwarder
	mux       *http.ServeMux
	startedAt time.Time
	limits    map[string]*rateBucket
	limitMu   sync.Mutex
	oauthMu   sync.Mutex
	oauth     map[string]oauthSession
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
	s := &Server{store: st, forwarder: proxy.NewForwarder(st), mux: http.NewServeMux(), startedAt: time.Now(), limits: map[string]*rateBucket{}, oauth: map[string]oauthSession{}}
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
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
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

func (s *Server) handleAuthFiles(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "auth file api is restricted to localhost"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"files": s.store.ListAuthFiles(r.URL.Query().Get("provider"), r.URL.Query().Get("account"))})
	case http.MethodPost:
		var body struct {
			Name         string `json:"name"`
			Provider     string `json:"provider"`
			AccountName  string `json:"accountName"`
			AccountEmail string `json:"accountEmail"`
			ContentB64   string `json:"contentB64"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 3*1024*1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		name := strings.TrimSpace(body.Name)
		contentB64 := strings.TrimSpace(body.ContentB64)
		if name == "" || contentB64 == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and contentB64 are required"})
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(contentB64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contentB64 must be valid base64"})
			return
		}
		if len(decoded) > 2*1024*1024 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth file too large"})
			return
		}
		created, err := s.store.CreateAuthFile(store.AuthFile{
			Name:         name,
			Provider:     strings.TrimSpace(body.Provider),
			AccountName:  strings.TrimSpace(body.AccountName),
			AccountEmail: strings.TrimSpace(body.AccountEmail),
			ContentB64:   contentB64,
			Size:         len(decoded),
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		created.ContentB64 = ""
		writeJSON(w, http.StatusCreated, created)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAuthFileByID(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "auth file api is restricted to localhost"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/auth-files/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing auth file id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, ok := s.store.GetAuthFile(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "auth file not found"})
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(item.ContentB64)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stored auth file is invalid"})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", item.Name))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(decoded)
	case http.MethodDelete:
		if err := s.store.DeleteAuthFile(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
