package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"xrouter/internal/version"
)

func (s *Server) handleMonitoringHealth(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "monitoring api is restricted to localhost"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.monitoringHealthPayload())
	case http.MethodPost:
		cleared, err := s.store.ClearAllCooldowns()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		payload := s.monitoringHealthPayload()
		payload["success"] = true
		payload["clearedCooldowns"] = cleared
		writeJSON(w, http.StatusOK, payload)
	case http.MethodDelete:
		connectionID := strings.TrimSpace(r.URL.Query().Get("connectionId"))
		provider := strings.TrimSpace(r.URL.Query().Get("provider"))
		if connectionID == "" && provider == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "connectionId or provider is required"})
			return
		}
		cleared, err := s.store.ClearHealthState(connectionID, provider)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		payload := s.monitoringHealthPayload()
		payload["success"] = true
		payload["clearedCooldowns"] = cleared
		writeJSON(w, http.StatusOK, payload)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "dashboard is restricted to localhost"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>XRouter Dashboard</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, system-ui, sans-serif; background: #0f172a; color: #e2e8f0; }
    body { margin: 0; padding: 24px; }
    header { display: flex; justify-content: space-between; gap: 16px; align-items: center; margin-bottom: 20px; }
    h1 { margin: 0; font-size: 24px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 18px; }
    .card { background: #111827; border: 1px solid #1f2937; border-radius: 14px; padding: 14px; box-shadow: 0 10px 30px rgba(0,0,0,.18); }
    .label { color: #94a3b8; font-size: 12px; text-transform: uppercase; letter-spacing: .08em; }
    .value { margin-top: 8px; font-size: 26px; font-weight: 700; }
    .toolbar { display: grid; grid-template-columns: 1fr 180px 180px; gap: 10px; margin: 14px 0; }
    .toolbar input, .toolbar select { background: #0b1220; color: #e2e8f0; border: 1px solid #334155; border-radius: 10px; padding: 10px 12px; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { border-bottom: 1px solid #1f2937; padding: 10px 8px; text-align: left; vertical-align: top; }
    th { color: #93c5fd; font-weight: 600; }
    tr[data-id] { cursor: pointer; }
    tr[data-id]:hover { background: #0b1220; }
    code { color: #a7f3d0; }
    pre { white-space: pre-wrap; word-break: break-word; background: #020617; border: 1px solid #1f2937; border-radius: 12px; padding: 12px; max-height: 360px; overflow: auto; }
    .status { color: #22c55e; }
    .error { color: #fb7185; }
  </style>
</head>
<body>
  <header>
    <div>
      <h1>XRouter Dashboard</h1>
      <div class="label">Local realtime usage, logs, stats</div>
    </div>
    <div id="streamStatus" class="status">connecting</div>
  </header>
  <section class="grid">
    <div class="card"><div class="label">Requests</div><div id="totalRequests" class="value">0</div></div>
    <div class="card"><div class="label">Prompt Tokens</div><div id="promptTokens" class="value">0</div></div>
    <div class="card"><div class="label">Completion Tokens</div><div id="completionTokens" class="value">0</div></div>
    <div class="card"><div class="label">Cost</div><div id="totalCost" class="value">$0.0000</div></div>
  </section>
  <section class="card">
    <h2>Recent Requests</h2>
    <div class="toolbar">
      <input id="searchInput" type="text" placeholder="Search model, path, error...">
      <select id="providerFilter">
        <option value="">All providers</option>
      </select>
      <select id="statusFilter">
        <option value="">All statuses</option>
        <option value="success">Success</option>
        <option value="error">Error</option>
      </select>
    </div>
    <table>
      <thead><tr><th>Time</th><th>Status</th><th>Provider</th><th>Model</th><th>Path</th><th>Error</th></tr></thead>
      <tbody id="logs"><tr><td colspan="6">Loading...</td></tr></tbody>
    </table>
  </section>
  <section class="card">
    <h2>Request Detail</h2>
    <pre id="logDetail">Click a request row to inspect details.</pre>
  </section>
  <script>
    const fmt = new Intl.NumberFormat();
    const money = new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', maximumFractionDigits: 4 });
    let currentLogs = [];
    function setStats(stats = {}) {
      totalRequests.textContent = fmt.format(stats.totalRequests || 0);
      promptTokens.textContent = fmt.format(stats.promptTokens || 0);
      completionTokens.textContent = fmt.format(stats.completionTokens || 0);
      totalCost.textContent = money.format(stats.totalCost || 0);
    }
    function renderProviderFilter(items = []) {
      const selected = providerFilter.value;
      const providers = [...new Set(items.map(item => item.provider).filter(Boolean))].sort();
      providerFilter.innerHTML = '<option value=\"\">All providers</option>' + providers.map(v => '<option value=\"' + v + '\">' + v + '</option>').join('');
      providerFilter.value = providers.includes(selected) ? selected : '';
    }
    function applyFilters(items = []) {
      const query = (searchInput.value || '').trim().toLowerCase();
      const provider = providerFilter.value;
      const status = statusFilter.value;
      return items.filter(item => {
        if (provider && item.provider !== provider) return false;
        if (status === 'success' && Number(item.statusCode || 0) >= 400) return false;
        if (status === 'error' && Number(item.statusCode || 0) < 400) return false;
        if (!query) return true;
        const haystack = [item.provider, item.model, item.path, item.error, String(item.statusCode || '')].join(' ').toLowerCase();
        return haystack.includes(query);
      });
    }
    function setLogs(items = []) {
      currentLogs = items;
      renderProviderFilter(items);
      const filtered = applyFilters(items);
      logs.innerHTML = filtered.length ? filtered.map(item => '<tr data-id="' + (item.id || '') + '">' +
        '<td>' + (item.timestamp || '') + '</td>' +
        '<td>' + (item.statusCode || '') + '</td>' +
        '<td><code>' + (item.provider || '') + '</code></td>' +
        '<td><code>' + (item.model || '') + '</code></td>' +
        '<td>' + (item.path || '') + '</td>' +
        '<td class="error">' + (item.error || '') + '</td>' +
      '</tr>').join('') : '<tr><td colspan="6">No requests yet</td></tr>';
    }
    async function showLogDetail(id) {
      if (!id) return;
      logDetail.textContent = 'Loading ' + id + '...';
      try {
        const res = await fetch('/api/usage/logs/' + encodeURIComponent(id));
        const payload = await res.json();
        logDetail.textContent = JSON.stringify(payload, null, 2);
      } catch (err) {
        logDetail.textContent = err.message;
      }
    }
    async function loadInitial() {
      const [statsRes, logsRes] = await Promise.all([fetch('/api/usage/stats'), fetch('/api/usage/logs?limit=50')]);
      setStats(await statsRes.json());
      const payload = await logsRes.json();
      setLogs(payload.items || []);
    }
    loadInitial().catch(err => { streamStatus.textContent = err.message; streamStatus.className = 'error'; });
    const source = new EventSource('/api/usage/stream?limit=50');
    source.onopen = () => { streamStatus.textContent = 'live'; streamStatus.className = 'status'; };
    source.onerror = () => { streamStatus.textContent = 'disconnected'; streamStatus.className = 'error'; };
    for (const type of ['snapshot', 'update']) {
      source.addEventListener(type, event => {
        const payload = JSON.parse(event.data);
        setStats(payload.stats);
        setLogs(payload.logs || []);
      });
    }
    for (const element of [searchInput, providerFilter, statusFilter]) {
      element.addEventListener('input', () => setLogs(currentLogs));
      element.addEventListener('change', () => setLogs(currentLogs));
    }
    logs.addEventListener('click', event => {
      const row = event.target.closest('tr[data-id]');
      if (row) showLogDetail(row.dataset.id);
    });
  </script>
</body>
</html>`

func (s *Server) monitoringHealthPayload() map[string]interface{} {
	now := time.Now().UTC()
	connections := s.store.GetAllConnectionsRaw()
	providers := map[string]map[string]interface{}{}
	cooldowns := make([]map[string]interface{}, 0)
	totals := map[string]int{
		"connections":  len(connections),
		"active":       0,
		"inactive":     0,
		"cooldown":     0,
		"unavailable":  0,
		"providerKeys": 0,
	}

	for _, conn := range connections {
		providerName := strings.TrimSpace(conn.Provider)
		if providerName == "" {
			providerName = "unknown"
		}
		provider := providers[providerName]
		if provider == nil {
			provider = map[string]interface{}{
				"provider":    providerName,
				"connections": 0,
				"active":      0,
				"inactive":    0,
				"cooldown":    0,
				"unavailable": 0,
			}
			providers[providerName] = provider
			totals["providerKeys"]++
		}

		provider["connections"] = provider["connections"].(int) + 1
		if !conn.IsActive {
			totals["inactive"]++
			provider["inactive"] = provider["inactive"].(int) + 1
			continue
		}

		inCooldown := false
		if conn.RateLimitedUntil != "" {
			if until, err := time.Parse(time.RFC3339, conn.RateLimitedUntil); err == nil && until.After(now) {
				inCooldown = true
			}
		}
		if inCooldown {
			totals["cooldown"]++
			provider["cooldown"] = provider["cooldown"].(int) + 1
			cooldowns = append(cooldowns, map[string]interface{}{
				"id":               conn.ID,
				"provider":         conn.Provider,
				"name":             conn.Name,
				"rateLimitedUntil": conn.RateLimitedUntil,
				"backoffLevel":     conn.BackoffLevel,
				"errorCode":        conn.ErrorCode,
				"lastError":        conn.LastError,
				"circuitOpenUntil": conn.CircuitOpenUntil,
			})
			continue
		}
		if conn.CircuitOpenUntil != "" {
			if until, err := time.Parse(time.RFC3339, conn.CircuitOpenUntil); err == nil && until.After(now) {
				totals["unavailable"]++
				provider["unavailable"] = provider["unavailable"].(int) + 1
				cooldowns = append(cooldowns, map[string]interface{}{
					"id":               conn.ID,
					"provider":         conn.Provider,
					"name":             conn.Name,
					"rateLimitedUntil": conn.RateLimitedUntil,
					"backoffLevel":     conn.BackoffLevel,
					"errorCode":        conn.ErrorCode,
					"lastError":        conn.LastError,
					"circuitOpenUntil": conn.CircuitOpenUntil,
				})
				continue
			}
		}
		if conn.TestStatus == "unavailable" {
			totals["unavailable"]++
			provider["unavailable"] = provider["unavailable"].(int) + 1
			continue
		}
		totals["active"]++
		provider["active"] = provider["active"].(int) + 1
	}

	status := "healthy"
	if totals["active"] == 0 && totals["connections"] > 0 {
		status = "unavailable"
	} else if totals["cooldown"] > 0 || totals["unavailable"] > 0 || totals["inactive"] > 0 {
		status = "degraded"
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return map[string]interface{}{
		"name":        "xrouter",
		"status":      status,
		"generatedAt": now.Format(time.RFC3339),
		"uptimeSec":   int(time.Since(s.startedAt).Seconds()),
		"totals":      totals,
		"providers":   providers,
		"cooldowns":   cooldowns,
		"runtime": map[string]interface{}{
			"goVersion":  runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"heapAlloc":  mem.HeapAlloc,
			"heapInuse":  mem.HeapInuse,
		},
	}
}

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, s.store.GetUsageSummary())
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	payload := version.Info()
	payload["name"] = "xrouter"
	payload["uptimeSec"] = strconv.Itoa(int(time.Since(s.startedAt).Seconds()))
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleUsageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, s.store.GetUsageStats())
}

func (s *Server) handleUsageLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	logs := s.store.GetRequestLogs(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": logs,
		"count": len(logs),
	})
}

func (s *Server) handleUsageStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	lastHeadID := ""
	send := func(event string, payload map[string]interface{}) bool {
		raw, _ := json.Marshal(payload)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	initialLogs := s.store.GetRequestLogs(limit)
	if len(initialLogs) > 0 {
		lastHeadID = initialLogs[0].ID
	}
	if !send("snapshot", map[string]interface{}{
		"stats": s.store.GetUsageStats(),
		"logs":  initialLogs,
	}) {
		return
	}
	if r.URL.Query().Get("once") == "1" {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			currentLogs := s.store.GetRequestLogs(limit)
			headID := ""
			if len(currentLogs) > 0 {
				headID = currentLogs[0].ID
			}
			if headID != "" && headID != lastHeadID {
				lastHeadID = headID
				if !send("update", map[string]interface{}{
					"stats": s.store.GetUsageStats(),
					"logs":  currentLogs,
				}) {
					return
				}
				continue
			}
			if !send("heartbeat", map[string]interface{}{"ts": time.Now().UTC().Format(time.RFC3339)}) {
				return
			}
		}
	}
}

func (s *Server) handleUsageLogByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/usage/logs/"), "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing request log id"})
		return
	}
	item, ok := s.store.GetRequestLogByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "request log not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUsageHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	items := s.store.GetUsageHistory(limit, provider)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":    items,
		"count":    len(items),
		"provider": provider,
	})
}

func (s *Server) handleDebugDB(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.DBSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
