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
	"strconv"
	"strings"
	"sync"
	"time"
)

type ProviderCatalogEntry struct {
	Provider       string   `json:"provider"`
	AuthType       string   `json:"authType,omitempty"`
	APIType        string   `json:"apiType"`
	BaseURL        string   `json:"baseUrl"`
	AuthorizeURL   string   `json:"authorizeUrl,omitempty"`
	TokenURL       string   `json:"tokenUrl,omitempty"`
	ClientID       string   `json:"clientId,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	FallbackModels []string `json:"fallbackModels,omitempty"`
}

var providerCatalog = map[string]ProviderCatalogEntry{
	"openai":            {Provider: "openai", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.openai.com"},
	"anthropic":         {Provider: "anthropic", AuthType: "apikey", APIType: "anthropic", BaseURL: "https://api.anthropic.com"},
	"openrouter":        {Provider: "openrouter", AuthType: "apikey", APIType: "openai", BaseURL: "https://openrouter.ai/api"},
	"deepseek":          {Provider: "deepseek", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.deepseek.com", FallbackModels: []string{"deepseek/deepseek-chat"}},
	"groq":              {Provider: "groq", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.groq.com/openai", FallbackModels: []string{"groq/llama-3.1-70b-versatile"}},
	"mistral":           {Provider: "mistral", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.mistral.ai", FallbackModels: []string{"mistral/mistral-large-latest"}},
	"cerebras":          {Provider: "cerebras", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.cerebras.ai", FallbackModels: []string{"cerebras/llama3.1-70b"}},
	"fireworks":         {Provider: "fireworks", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.fireworks.ai/inference/v1", FallbackModels: []string{"fireworks/accounts/fireworks/models/llama-v3p1-70b-instruct"}},
	"together":          {Provider: "together", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.together.xyz/v1", FallbackModels: []string{"together/meta-llama/Llama-3.1-70B-Instruct-Turbo"}},
	"siliconflow":       {Provider: "siliconflow", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.siliconflow.cn/v1", FallbackModels: []string{"siliconflow/Qwen/Qwen2.5-Coder-32B-Instruct"}},
	"vercel-ai-gateway": {Provider: "vercel-ai-gateway", AuthType: "apikey", APIType: "openai", BaseURL: "https://ai-gateway.vercel.sh/v1", FallbackModels: []string{"vercel-ai-gateway/openai/gpt-4o-mini"}},
	"cohere":            {Provider: "cohere", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.cohere.com/compatibility/v1", FallbackModels: []string{"cohere/command-r-plus"}},
	"perplexity":        {Provider: "perplexity", AuthType: "apikey", APIType: "openai", BaseURL: "https://api.perplexity.ai", FallbackModels: []string{"perplexity/sonar-pro"}},
	"voyage-ai":         {Provider: "voyage-ai", AuthType: "apikey", APIType: "embedding", BaseURL: "https://api.voyageai.com/v1"},
	"jina-ai":           {Provider: "jina-ai", AuthType: "apikey", APIType: "embedding", BaseURL: "https://api.jina.ai/v1"},
	"openai-tts":        {Provider: "openai-tts", AuthType: "apikey", APIType: "tts", BaseURL: "https://api.openai.com/v1"},
	"deepgram":          {Provider: "deepgram", AuthType: "apikey", APIType: "stt", BaseURL: "https://api.deepgram.com/v1"},
	"brave-search":      {Provider: "brave-search", AuthType: "apikey", APIType: "search", BaseURL: "https://api.search.brave.com/res/v1"},
	"serper":            {Provider: "serper", AuthType: "apikey", APIType: "search", BaseURL: "https://google.serper.dev"},
	"tavily":            {Provider: "tavily", AuthType: "apikey", APIType: "search", BaseURL: "https://api.tavily.com"},
	"exa":               {Provider: "exa", AuthType: "apikey", APIType: "search", BaseURL: "https://api.exa.ai"},
	"perplexity-search": {Provider: "perplexity-search", AuthType: "apikey", APIType: "search", BaseURL: "https://api.perplexity.ai"},
	"claude":            {Provider: "claude", AuthType: "oauth", APIType: "anthropic", BaseURL: "https://api.anthropic.com", TokenURL: "https://console.anthropic.com/v1/oauth/token", FallbackModels: []string{"claude/claude-3-5-sonnet-latest"}},
	"codex":             {Provider: "codex", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.openai.com", AuthorizeURL: "https://auth.openai.com/oauth/authorize", TokenURL: "https://auth.openai.com/oauth/token", ClientID: "codex-cli", Scopes: []string{"openid", "profile", "email", "offline_access"}, FallbackModels: []string{"codex/gpt-4o-mini"}},
	"github":            {Provider: "github", AuthType: "oauth", APIType: "openai", BaseURL: "https://models.inference.ai.azure.com", FallbackModels: []string{"github/gpt-4o-mini"}},
	"gemini":            {Provider: "gemini", AuthType: "oauth", APIType: "openai", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", ClientID: "gemini-cli", Scopes: []string{"https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/userinfo.email"}, FallbackModels: []string{"gemini/gemini-1.5-flash"}},
	"xai":               {Provider: "xai", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.x.ai", TokenURL: "https://accounts.x.ai/oauth/token", FallbackModels: []string{"xai/grok-2-latest"}},
	"antigravity":       {Provider: "antigravity", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.antigravity.ai/v1"},
	"kimi":              {Provider: "kimi", AuthType: "oauth", APIType: "openai", BaseURL: "https://api.moonshot.ai", TokenURL: "https://www.kimi.com/api/oauth/token", FallbackModels: []string{"kimi/moonshot-v1-8k"}},
}

type ProviderConnection struct {
	ID                   string                 `json:"id"`
	Provider             string                 `json:"provider"`
	Name                 string                 `json:"name"`
	AuthType             string                 `json:"authType"`
	APIKey               string                 `json:"apiKey,omitempty"`
	AccessToken          string                 `json:"accessToken,omitempty"`
	RefreshToken         string                 `json:"refreshToken,omitempty"`
	TokenExpiry          string                 `json:"tokenExpiry,omitempty"`
	IsActive             bool                   `json:"isActive"`
	Priority             int                    `json:"priority"`
	GlobalPriority       *int                   `json:"globalPriority,omitempty"`
	DefaultModel         string                 `json:"defaultModel,omitempty"`
	ProviderSpecificData map[string]interface{} `json:"providerSpecificData,omitempty"`
	RateLimitedUntil     string                 `json:"rateLimitedUntil,omitempty"`
	BackoffLevel         int                    `json:"backoffLevel,omitempty"`
	ConsecutiveFailures  int                    `json:"consecutiveFailures,omitempty"`
	CircuitOpenUntil     string                 `json:"circuitOpenUntil,omitempty"`
	LastError            string                 `json:"lastError,omitempty"`
	ErrorCode            int                    `json:"errorCode,omitempty"`
	TestStatus           string                 `json:"testStatus,omitempty"`
	CreatedAt            string                 `json:"createdAt,omitempty"`
	UpdatedAt            string                 `json:"updatedAt,omitempty"`
}

type APIKey struct {
	ID                string `json:"id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	RequestsPerMinute int    `json:"requestsPerMinute,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
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

type RequestLog struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp,omitempty"`
	Path          string `json:"path,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	APIKeyID      string `json:"apiKeyId,omitempty"`
	StatusCode    int    `json:"statusCode,omitempty"`
	LatencyMs     int64  `json:"latencyMs,omitempty"`
	RequestBytes  int    `json:"requestBytes,omitempty"`
	ResponseBytes int    `json:"responseBytes,omitempty"`
	Error         string `json:"error,omitempty"`
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
	DefaultRequestsPerMinute   int               `json:"defaultRequestsPerMinute,omitempty"`
}

type DB struct {
	ProviderConnections []ProviderConnection   `json:"providerConnections"`
	APIKeys             []APIKey               `json:"apiKeys"`
	Settings            Settings               `json:"settings"`
	ModelAliases        map[string]string      `json:"modelAliases"`
	Pricing             map[string]interface{} `json:"pricing"`
	UsageData           UsageData              `json:"usageData"`
	RequestLogs         []RequestLog           `json:"requestLogs"`
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
	if db.RequestLogs == nil {
		db.RequestLogs = []RequestLog{}
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
		RequestLogs:  []RequestLog{},
	}
}

func GetProviderCatalogEntry(provider string) (ProviderCatalogEntry, bool) {
	entry, ok := providerCatalog[strings.ToLower(strings.TrimSpace(provider))]
	return entry, ok
}

func GetFallbackModels() []map[string]string {
	out := make([]map[string]string, 0, 16)
	for _, entry := range providerCatalog {
		for _, model := range entry.FallbackModels {
			out = append(out, map[string]string{"fullModel": model, "alias": ""})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["fullModel"] < out[j]["fullModel"]
	})
	return out
}

func ListProviderCatalogEntries() []ProviderCatalogEntry {
	out := make([]ProviderCatalogEntry, 0, len(providerCatalog))
	for _, entry := range providerCatalog {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Provider < out[j].Provider
	})
	return out
}

func applyProviderDefaults(c ProviderConnection) ProviderConnection {
	entry, ok := GetProviderCatalogEntry(c.Provider)
	if !ok {
		return c
	}
	if c.ProviderSpecificData == nil {
		c.ProviderSpecificData = map[string]interface{}{}
	}
	if strings.TrimSpace(c.AuthType) == "" {
		c.AuthType = entry.AuthType
		if c.AuthType == "" {
			c.AuthType = "apikey"
		}
	}
	if _, ok := c.ProviderSpecificData["baseUrl"]; !ok || strings.TrimSpace(fmt.Sprint(c.ProviderSpecificData["baseUrl"])) == "" {
		c.ProviderSpecificData["baseUrl"] = entry.BaseURL
	}
	if _, ok := c.ProviderSpecificData["apiType"]; !ok || strings.TrimSpace(fmt.Sprint(c.ProviderSpecificData["apiType"])) == "" {
		c.ProviderSpecificData["apiType"] = entry.APIType
	}
	if strings.TrimSpace(c.DefaultModel) == "" && len(entry.FallbackModels) > 0 {
		c.DefaultModel = entry.FallbackModels[0]
	}
	return c
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
	if s.rawRoot["requestLogs"], err = mustJSON(s.db.RequestLogs); err != nil {
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
		if c.CircuitOpenUntil != "" {
			until, err := time.Parse(time.RFC3339, c.CircuitOpenUntil)
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
		s.db.ProviderConnections[i].ConsecutiveFailures++
		s.db.ProviderConnections[i].LastError = message
		s.db.ProviderConnections[i].ErrorCode = status
		s.db.ProviderConnections[i].TestStatus = "unavailable"
		if shouldOpenCircuit(status, s.db.ProviderConnections[i].ConsecutiveFailures) {
			s.db.ProviderConnections[i].CircuitOpenUntil = until.UTC().Add(getCircuitBreakDuration(s.db.ProviderConnections[i].ConsecutiveFailures)).Format(time.RFC3339)
			s.db.ProviderConnections[i].TestStatus = "circuit-open"
		}
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
		s.db.ProviderConnections[i].ConsecutiveFailures = 0
		s.db.ProviderConnections[i].CircuitOpenUntil = ""
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

func (s *Store) GetConnectionByIDRaw(id string) (ProviderConnection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.db.ProviderConnections {
		if c.ID == id {
			return c, true
		}
	}
	return ProviderConnection{}, false
}

func (s *Store) UpdateConnectionTestStatus(id, status, message string, code int) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		s.db.ProviderConnections[i].TestStatus = strings.TrimSpace(status)
		s.db.ProviderConnections[i].LastError = strings.TrimSpace(message)
		s.db.ProviderConnections[i].ErrorCode = code
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.persistLocked(); err != nil {
			return ProviderConnection{}, err
		}
		return s.db.ProviderConnections[i], nil
	}
	return ProviderConnection{}, fmt.Errorf("provider connection not found")
}

func (s *Store) ClearAllCooldowns() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := 0
	for i := range s.db.ProviderConnections {
		conn := &s.db.ProviderConnections[i]
		if conn.RateLimitedUntil == "" && conn.BackoffLevel == 0 && conn.ConsecutiveFailures == 0 && conn.CircuitOpenUntil == "" && conn.LastError == "" && conn.ErrorCode == 0 && conn.TestStatus == "" {
			continue
		}
		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.ConsecutiveFailures = 0
		conn.CircuitOpenUntil = ""
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

func shouldOpenCircuit(status, failures int) bool {
	if failures < 3 {
		return false
	}
	return status == 0 || status >= 500
}

func getCircuitBreakDuration(failures int) time.Duration {
	switch {
	case failures >= 6:
		return 90 * time.Second
	case failures >= 4:
		return 45 * time.Second
	default:
		return 20 * time.Second
	}
}

func (s *Store) ClearHealthState(connectionID, provider string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := 0
	for i := range s.db.ProviderConnections {
		conn := &s.db.ProviderConnections[i]
		if connectionID != "" && conn.ID != connectionID {
			continue
		}
		if provider != "" && conn.Provider != provider {
			continue
		}
		if conn.RateLimitedUntil == "" && conn.BackoffLevel == 0 && conn.ConsecutiveFailures == 0 && conn.CircuitOpenUntil == "" && conn.LastError == "" && conn.ErrorCode == 0 && conn.TestStatus == "" {
			continue
		}
		conn.RateLimitedUntil = ""
		conn.BackoffLevel = 0
		conn.ConsecutiveFailures = 0
		conn.CircuitOpenUntil = ""
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
	c = applyProviderDefaults(c)
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
	next = applyProviderDefaults(next)
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

func (s *Store) UpdateOAuthTokens(id, accessToken, refreshToken, tokenExpiry string) (ProviderConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.ProviderConnections {
		if s.db.ProviderConnections[i].ID != id {
			continue
		}
		if strings.TrimSpace(accessToken) != "" {
			s.db.ProviderConnections[i].AccessToken = strings.TrimSpace(accessToken)
		}
		if strings.TrimSpace(refreshToken) != "" {
			s.db.ProviderConnections[i].RefreshToken = strings.TrimSpace(refreshToken)
		}
		if tokenExpiry != "" {
			s.db.ProviderConnections[i].TokenExpiry = tokenExpiry
		}
		s.db.ProviderConnections[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.persistLocked(); err != nil {
			return ProviderConnection{}, err
		}
		return s.db.ProviderConnections[i], nil
	}
	return ProviderConnection{}, fmt.Errorf("provider connection not found")
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

func (s *Store) GetAPIKeyByValue(key string) (APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.db.APIKeys {
		if item.Key == key {
			return item, true
		}
	}
	return APIKey{}, false
}

func maskAPIKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= 8 {
		return "********"
	}
	return trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
}

func (s *Store) GetAPIKeys() []APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]APIKey, len(s.db.APIKeys))
	for i, key := range s.db.APIKeys {
		key.Key = maskAPIKey(key.Key)
		out[i] = key
	}
	return out
}

func (s *Store) CreateAPIKey(name, key string, requestsPerMinute int) (APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	key = strings.TrimSpace(key)
	if key == "" {
		return APIKey{}, fmt.Errorf("api key is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	item := APIKey{
		ID:                randID("key_"),
		Key:               key,
		Name:              name,
		RequestsPerMinute: requestsPerMinute,
		CreatedAt:         now,
	}
	s.db.APIKeys = append(s.db.APIKeys, item)
	return item, s.persistLocked()
}

func (s *Store) DeleteAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.db.APIKeys[:0]
	found := false
	for _, item := range s.db.APIKeys {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("api key not found")
	}
	s.db.APIKeys = next
	return s.persistLocked()
}

func (s *Store) RotateAPIKey(id string, newKey string) (APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	newKey = strings.TrimSpace(newKey)
	if newKey == "" {
		return APIKey{}, fmt.Errorf("new api key is required")
	}
	for i := range s.db.APIKeys {
		if s.db.APIKeys[i].ID != id {
			continue
		}
		s.db.APIKeys[i].Key = newKey
		if err := s.persistLocked(); err != nil {
			return APIKey{}, err
		}
		return s.db.APIKeys[i], nil
	}
	return APIKey{}, fmt.Errorf("api key not found")
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

func (s *Store) RecordUsage(entry UsageEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if entry.Timestamp == "" {
		entry.Timestamp = now.Format(time.RFC3339)
	}
	if entry.TotalCost == 0 {
		entry.TotalCost = estimateCost(s.db.Pricing, entry.Model, entry.PromptTokens, entry.CompletionTokens)
	}
	s.db.UsageData.TotalRequestsLifetime++
	s.db.UsageData.History = append(s.db.UsageData.History, entry)
	limit := s.db.Settings.ObservabilityMaxRecords
	if limit <= 0 {
		limit = 1000
	}
	if len(s.db.UsageData.History) > limit {
		s.db.UsageData.History = s.db.UsageData.History[len(s.db.UsageData.History)-limit:]
	}
	day := now.Format("2006-01-02")
	daily := s.db.UsageData.DailySummary[day]
	daily.Requests++
	daily.Cost += entry.TotalCost
	s.db.UsageData.DailySummary[day] = daily
	return s.persistLocked()
}

func (s *Store) RecordRequestLog(entry RequestLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = randID("rlog_")
	}
	if strings.TrimSpace(entry.Timestamp) == "" {
		entry.Timestamp = now.Format(time.RFC3339)
	}
	s.db.RequestLogs = append(s.db.RequestLogs, entry)
	limit := s.db.Settings.ObservabilityMaxRecords
	if limit <= 0 {
		limit = 1000
	}
	if len(s.db.RequestLogs) > limit {
		s.db.RequestLogs = s.db.RequestLogs[len(s.db.RequestLogs)-limit:]
	}
	return s.persistLocked()
}

func (s *Store) GetRequestLogs(limit int) []RequestLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.db.RequestLogs) {
		limit = len(s.db.RequestLogs)
	}
	out := make([]RequestLog, 0, limit)
	for i := len(s.db.RequestLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.db.RequestLogs[i])
	}
	return out
}

func (s *Store) GetRequestLogByID(id string) (RequestLog, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = strings.TrimSpace(id)
	for _, item := range s.db.RequestLogs {
		if item.ID == id {
			return item, true
		}
	}
	return RequestLog{}, false
}

func (s *Store) GetUsageHistory(limit int, provider string) []UsageEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UsageEntry, 0, len(s.db.UsageData.History))
	provider = strings.TrimSpace(provider)
	for i := len(s.db.UsageData.History) - 1; i >= 0; i-- {
		item := s.db.UsageData.History[i]
		if provider != "" && item.Provider != provider {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Store) GetUsageStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byProvider := map[string]DailySummary{}
	byModel := map[string]DailySummary{}
	byDay := map[string]DailySummary{}
	var totalRequests int64
	totalCost := 0.0
	var promptTokens int64
	var completionTokens int64
	for _, item := range s.db.UsageData.History {
		totalRequests++
		totalCost += item.TotalCost
		promptTokens += item.PromptTokens
		completionTokens += item.CompletionTokens
		addUsageStat(byProvider, item.Provider, item.TotalCost)
		addUsageStat(byModel, item.Model, item.TotalCost)
		day := usageDay(item.Timestamp)
		if day != "" {
			addUsageStat(byDay, day, item.TotalCost)
		}
	}
	return map[string]interface{}{
		"totalRequests":    totalRequests,
		"totalCost":        totalCost,
		"promptTokens":     promptTokens,
		"completionTokens": completionTokens,
		"totalTokens":      promptTokens + completionTokens,
		"byProvider":       byProvider,
		"byModel":          byModel,
		"byDay":            byDay,
	}
}

func addUsageStat(bucket map[string]DailySummary, key string, cost float64) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	cur := bucket[key]
	cur.Requests++
	cur.Cost += cost
	bucket[key] = cur
}

func usageDay(timestamp string) string {
	timestamp = strings.TrimSpace(timestamp)
	if timestamp == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, timestamp); err == nil {
		return parsed.UTC().Format("2006-01-02")
	}
	if len(timestamp) >= len("2006-01-02") {
		return timestamp[:len("2006-01-02")]
	}
	return timestamp
}

func pricingNumber(node map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := node[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case float64:
			return v, true
		case int64:
			return float64(v), true
		case int:
			return float64(v), true
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func estimateCost(pricing map[string]interface{}, model string, promptTokens, completionTokens int64) float64 {
	if pricing == nil || strings.TrimSpace(model) == "" {
		return 0
	}
	candidates := []string{model}
	if idx := strings.Index(model, "/"); idx >= 0 && idx+1 < len(model) {
		candidates = append(candidates, model[idx+1:])
	}
	for _, key := range candidates {
		raw, ok := pricing[key]
		if !ok {
			continue
		}
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		promptRate, _ := pricingNumber(entry, "prompt", "promptCostPer1k", "promptCostPer1K", "input", "inputCostPer1k", "inputCostPer1K")
		completionRate, _ := pricingNumber(entry, "completion", "completionCostPer1k", "completionCostPer1K", "output", "outputCostPer1k", "outputCostPer1K")
		cost := float64(promptTokens)/1000.0*promptRate + float64(completionTokens)/1000.0*completionRate
		if cost < 0 {
			return 0
		}
		return cost
	}
	return 0
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
