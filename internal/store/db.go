package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type ProviderConnection struct {
	ID                   string                 `json:"id"`
	Provider             string                 `json:"provider"`
	Name                 string                 `json:"name"`
	AuthType             string                 `json:"authType"`
	APIKey               string                 `json:"apiKey,omitempty"`
	AccessToken          string                 `json:"accessToken,omitempty"`
	RefreshToken         string                 `json:"refreshToken,omitempty"`
	IsActive             bool                   `json:"isActive"`
	Priority             int                    `json:"priority"`
	GlobalPriority       *int                   `json:"globalPriority,omitempty"`
	DefaultModel         string                 `json:"defaultModel,omitempty"`
	ProviderSpecificData map[string]interface{} `json:"providerSpecificData,omitempty"`
	RateLimitedUntil     string                 `json:"rateLimitedUntil,omitempty"`
	BackoffLevel         int                    `json:"backoffLevel,omitempty"`
	LastError            string                 `json:"lastError,omitempty"`
	ErrorCode            int                    `json:"errorCode,omitempty"`
	TestStatus           string                 `json:"testStatus,omitempty"`
	CreatedAt            string                 `json:"createdAt,omitempty"`
	UpdatedAt            string                 `json:"updatedAt,omitempty"`
}

type APIKey struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type UsageEntry struct {
	Timestamp        string  `json:"timestamp,omitempty"`
	Provider         string  `json:"provider,omitempty"`
	Model            string  `json:"model,omitempty"`
	TotalCost        float64 `json:"totalCost,omitempty"`
	PromptTokens     int64   `json:"promptTokens,omitempty"`
	CompletionTokens int64   `json:"completionTokens,omitempty"`
}

type UsageData struct {
	History               []UsageEntry            `json:"history"`
	TotalRequestsLifetime int64                   `json:"totalRequestsLifetime"`
	DailySummary          map[string]DailySummary `json:"dailySummary"`
}

type DailySummary struct {
	Requests int64   `json:"requests"`
	Cost     float64 `json:"cost"`
}

type Settings struct {
	RequireAPIKey              bool              `json:"requireApiKey"`
	RequireLogin               bool              `json:"requireLogin"`
	StickyRoundRobinLimit      int               `json:"stickyRoundRobinLimit"`
	ComboStrategy              string            `json:"comboStrategy"`
	ComboStickyRoundRobinLimit int               `json:"comboStickyRoundRobinLimit"`
	OutboundProxyEnabled       bool              `json:"outboundProxyEnabled"`
	OutboundProxyURL           string            `json:"outboundProxyUrl"`
	OutboundNoProxy            string            `json:"outboundNoProxy"`
	ObservabilityEnabled       bool              `json:"observabilityEnabled"`
	ObservabilityMaxRecords    int               `json:"observabilityMaxRecords"`
	ForcedModelMappings        map[string]string `json:"forcedModelMappings,omitempty"`
}

type DB struct {
	ProviderConnections []ProviderConnection   `json:"providerConnections"`
	APIKeys             []APIKey               `json:"apiKeys"`
	Settings            Settings               `json:"settings"`
	ModelAliases        map[string]string      `json:"modelAliases"`
	Pricing             map[string]interface{} `json:"pricing"`
	UsageData           UsageData              `json:"usageData"`
}

type Store struct {
	mu       sync.RWMutex
	db       DB
	rawRoot  map[string]json.RawMessage
	dbPath   string
	loadedAt time.Time
}

func DataDir() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "xlabrouter")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".xlabrouter")
}

func NewStore() (*Store, error) {
	dir := DataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{dbPath: filepath.Join(dir, "db.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.dbPath)
	if os.IsNotExist(err) {
		s.db = defaultDB()
		s.rawRoot = map[string]json.RawMessage{}
		s.loadedAt = time.Now()
		return nil
	}
	if err != nil {
		return fmt.Errorf("read db.json: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse db.json root: %w", err)
	}
	var db DB
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("parse db.json typed: %w", err)
	}
	if db.ModelAliases == nil {
		db.ModelAliases = map[string]string{}
	}
	if db.Pricing == nil {
		db.Pricing = map[string]interface{}{}
	}
	if db.UsageData.DailySummary == nil {
		db.UsageData.DailySummary = map[string]DailySummary{}
	}
	if db.Settings.ForcedModelMappings == nil {
		db.Settings.ForcedModelMappings = map[string]string{}
	}
	s.db = db
	s.rawRoot = root
	s.loadedAt = time.Now()
	return nil
}

func defaultDB() DB {
	return DB{
		ProviderConnections: []ProviderConnection{},
		APIKeys:             []APIKey{},
		Settings: Settings{
			RequireLogin:               true,
			StickyRoundRobinLimit:      3,
			ComboStrategy:              "fallback",
			ComboStickyRoundRobinLimit: 1,
			ObservabilityEnabled:       true,
			ObservabilityMaxRecords:    1000,
			ForcedModelMappings:        map[string]string{},
		},
		ModelAliases: map[string]string{},
		Pricing:      map[string]interface{}{},
		UsageData:    UsageData{History: []UsageEntry{}, TotalRequestsLifetime: 0, DailySummary: map[string]DailySummary{}},
	}
}

func mustJSON(v interface{}) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func (s *Store) persistLocked() error {
	if s.rawRoot == nil {
		s.rawRoot = map[string]json.RawMessage{}
	}
	var err error
	if s.rawRoot["providerConnections"], err = mustJSON(s.db.ProviderConnections); err != nil {
		return err
	}
	if s.rawRoot["apiKeys"], err = mustJSON(s.db.APIKeys); err != nil {
		return err
	}
	if s.rawRoot["settings"], err = mustJSON(s.db.Settings); err != nil {
		return err
	}
	if s.rawRoot["modelAliases"], err = mustJSON(s.db.ModelAliases); err != nil {
		return err
	}
	if s.rawRoot["pricing"], err = mustJSON(s.db.Pricing); err != nil {
		return err
	}
	if s.rawRoot["usageData"], err = mustJSON(s.db.UsageData); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(s.rawRoot, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.dbPath + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.dbPath)
}

func (s *Store) Reload() error         { s.mu.Lock(); defer s.mu.Unlock(); return s.load() }
func (s *Store) GetSettings() Settings { s.mu.RLock(); defer s.mu.RUnlock(); return s.db.Settings }

func (s *Store) UpdateSettings(patch map[string]interface{}) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(s.db.Settings)
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	nextRaw, _ := json.Marshal(merged)
	var next Settings
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return Settings{}, err
	}
	s.db.Settings = next
	return next, s.persistLocked()
}

func sortConnections(conns []ProviderConnection) {
	sort.SliceStable(conns, func(i, j int) bool { return conns[i].Priority < conns[j].Priority })
}

func (s *Store) GetActiveConnections(provider string) []ProviderConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ProviderConnection
	for _, c := range s.db.ProviderConnections {
		if !c.IsActive {
			continue
		}
		if provider != "" && c.Provider != provider {
			continue
		}
		if c.RateLimitedUntil != "" {
			until, err := time.Parse(time.RFC3339, c.RateLimitedUntil)
			if err == nil && until.After(time.Now()) {
				continue
			}
		}
		out = append(out, c)
	}
	sortConnections(out)
	return out
}

func (s *Store) MarkConnectionCooldown(id string, until time.Time, status int, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		s.db.ProviderConnections[i].RateLimitedUntil = until.UTC().Format(time.RFC3339)
		s.db.ProviderConnections[i].BackoffLevel++
		s.db.ProviderConnections[i].LastError = message
		s.db.ProviderConnections[i].ErrorCode = status
		s.db.ProviderConnections[i].TestStatus = "unavailable"
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return s.persistLocked()
	}
	return fmt.Errorf("provider connection not found")
}

func (s *Store) ClearConnectionCooldown(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		s.db.ProviderConnections[i].RateLimitedUntil = ""
		s.db.ProviderConnections[i].BackoffLevel = 0
		s.db.ProviderConnections[i].LastError = ""
		s.db.ProviderConnections[i].ErrorCode = 0
		s.db.ProviderConnections[i].TestStatus = "active"
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return s.persistLocked()
	}
	return fmt.Errorf("provider connection not found")
}

func (s *Store) GetAllConnections() []ProviderConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProviderConnection, len(s.db.ProviderConnections))
	for i, c := range s.db.ProviderConnections {
		c.APIKey = ""
		c.AccessToken = ""
		c.RefreshToken = ""
		out[i] = c
	}
	sortConnections(out)
	return out
}

func (s *Store) GetAllConnectionsRaw() []ProviderConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProviderConnection, len(s.db.ProviderConnections))
	copy(out, s.db.ProviderConnections)
	sortConnections(out)
	return out
}

func (s *Store) ClearAllCooldowns() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := 0
	for i := range s.db.ProviderConnections {
		conn := &s.db.ProviderConnections[i]
		if conn.RateLimitedUntil == "" && conn.BackoffLevel == 0 && conn.LastError == "" && conn.ErrorCode == 0 && conn.TestStatus == "" {
			continue
		}
		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.LastError = ""
		conn.ErrorCode = 0
		conn.TestStatus = "active"
		conn.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		cleared++
	}
	if cleared == 0 {
		return 0, nil
	}
	return cleared, s.persistLocked()
}

func randID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return prefix + hex.EncodeToString(buf)
}

func (s *Store) CreateProviderConnection(c ProviderConnection) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	if c.ID == "" {
		c.ID = randID("pc_")
	}
	if c.Priority <= 0 {
		c.Priority = 1
	}
	c.CreatedAt = now
	c.UpdatedAt = now
	s.db.ProviderConnections = append(s.db.ProviderConnections, c)
	return c, s.persistLocked()
}

func (s *Store) UpdateProviderConnection(id string, patch map[string]interface{}) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, c := range s.db.ProviderConnections {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ProviderConnection{}, fmt.Errorf("provider connection not found")
	}
	raw, _ := json.Marshal(s.db.ProviderConnections[idx])
	merged := map[string]interface{}{}
	_ = json.Unmarshal(raw, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	merged["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	nextRaw, _ := json.Marshal(merged)
	var next ProviderConnection
	if err := json.Unmarshal(nextRaw, &next); err != nil {
		return ProviderConnection{}, err
	}
	s.db.ProviderConnections[idx] = next
	return next, s.persistLocked()
}

func (s *Store) DeleteProviderConnection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.ProviderConnections[:0]
	found := false
	for _, c := range s.db.ProviderConnections {
		if c.ID == id {
			found = true
			continue
		}
		next = append(next, c)
	}
	if !found {
		return fmt.Errorf("provider connection not found")
	}
	s.db.ProviderConnections = next
	return s.persistLocked()
}

func (s *Store) ValidateAPIKey(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.db.APIKeys {
		if k.Key == key {
			return true
		}
	}
	return false
}

func (s *Store) GetModelAliases() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.db.ModelAliases))
	for k, v := range s.db.ModelAliases {
		out[k] = v
	}
	return out
}

func sanitizeMappings(input map[string]string) map[string]string {
	out := map[string]string{}
	for source, target := range input {
		k := strings.TrimSpace(source)
		v := strings.TrimSpace(target)
		if k == "" || v == "" || !strings.Contains(v, "/") {
			continue
		}
		out[k] = v
	}
	return out
}

func (s *Store) GetForcedModelMappings() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return sanitizeMappings(out)
}

func (s *Store) ReplaceForcedModelMappings(mappings map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Settings.ForcedModelMappings = sanitizeMappings(mappings)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return out, nil
}

func (s *Store) PatchForcedModelMappings(mappings map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.Settings.ForcedModelMappings == nil {
		s.db.Settings.ForcedModelMappings = map[string]string{}
	}
	for k, v := range sanitizeMappings(mappings) {
		s.db.Settings.ForcedModelMappings[k] = v
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return out, nil
}

func (s *Store) DeleteForcedModelMappingKeys(keys []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.Settings.ForcedModelMappings == nil {
		s.db.Settings.ForcedModelMappings = map[string]string{}
	}
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		delete(s.db.Settings.ForcedModelMappings, trimmed)
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(s.db.Settings.ForcedModelMappings))
	for k, v := range s.db.Settings.ForcedModelMappings {
		out[k] = v
	}
	return out, nil
}

func (s *Store) GetUsageSummary() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	totalCost := 0.0
	for _, item := range s.db.UsageData.History {
		totalCost += item.TotalCost
	}
	providers := map[string]DailySummary{}
	for _, item := range s.db.UsageData.History {
		cur := providers[item.Provider]
		cur.Requests++
		cur.Cost += item.TotalCost
		providers[item.Provider] = cur
	}
	return map[string]interface{}{"totalRequests": s.db.UsageData.TotalRequestsLifetime, "totalCost": totalCost, "providers": providers, "days": s.db.UsageData.DailySummary, "historySize": len(s.db.UsageData.History)}
}

func (s *Store) DBSnapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	safe := s.db
	conns := make([]ProviderConnection, len(safe.ProviderConnections))
	for i, c := range safe.ProviderConnections {
		c.APIKey = ""
		c.AccessToken = ""
		c.RefreshToken = ""
		conns[i] = c
	}
	safe.ProviderConnections = conns
	safe.APIKeys = nil
	return json.Marshal(safe)
}
