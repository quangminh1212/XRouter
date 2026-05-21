package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func cursorDBCandidates() []string {
	if override := strings.TrimSpace(os.Getenv("XROUTER_CURSOR_DB")); override != "" {
		return []string{override}
	}
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return []string{
			filepath.Join(appData, "Cursor", "User", "globalStorage", "state.vscdb"),
			filepath.Join(appData, "Cursor - Insiders", "User", "globalStorage", "state.vscdb"),
			filepath.Join(localAppData, "Cursor", "User", "globalStorage", "state.vscdb"),
			filepath.Join(localAppData, "Programs", "Cursor", "User", "globalStorage", "state.vscdb"),
		}
	}
	if runtime.GOOS == "darwin" {
		return []string{
			filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb"),
			filepath.Join(home, "Library", "Application Support", "Cursor - Insiders", "User", "globalStorage", "state.vscdb"),
		}
	}
	return []string{
		filepath.Join(home, ".config", "Cursor", "User", "globalStorage", "state.vscdb"),
		filepath.Join(home, ".config", "cursor", "User", "globalStorage", "state.vscdb"),
	}
}

func normalizeCursorValue(value string) string {
	value = strings.TrimSpace(value)
	var parsed string
	if err := json.Unmarshal([]byte(value), &parsed); err == nil {
		return strings.TrimSpace(parsed)
	}
	return value
}

func queryCursorSQLite(dbPath, key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := fmt.Sprintf("SELECT value FROM itemTable WHERE key='%s' LIMIT 1", strings.ReplaceAll(key, "'", "''"))
	out, err := exec.CommandContext(ctx, "sqlite3", dbPath, query).Output()
	if err != nil {
		return "", err
	}
	return normalizeCursorValue(string(out)), nil
}

func extractCursorTokens(dbPath string) (string, string, error) {
	accessKeys := []string{"cursorAuth/accessToken", "cursorAuth/token"}
	machineKeys := []string{"storage.serviceMachineId", "storage.machineId", "telemetry.machineId"}
	accessToken := ""
	machineID := ""
	var lastErr error
	for _, key := range accessKeys {
		value, err := queryCursorSQLite(dbPath, key)
		if err != nil {
			lastErr = err
			continue
		}
		if value != "" {
			accessToken = value
			break
		}
	}
	for _, key := range machineKeys {
		value, err := queryCursorSQLite(dbPath, key)
		if err != nil {
			lastErr = err
			continue
		}
		if value != "" {
			machineID = value
			break
		}
	}
	if accessToken == "" || machineID == "" {
		if lastErr != nil {
			return accessToken, machineID, lastErr
		}
		return accessToken, machineID, fmt.Errorf("cursor tokens not found")
	}
	return accessToken, machineID, nil
}

func (s *Server) handleCursorAutoImport(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	candidates := cursorDBCandidates()
	dbPath := ""
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			dbPath = candidate
			break
		}
	}
	if dbPath == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"found": false, "error": "Cursor database not found", "checked": candidates})
		return
	}
	accessToken, machineID, err := extractCursorTokens(dbPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"found": false, "windowsManual": true, "dbPath": dbPath, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"found": true, "accessToken": accessToken, "machineId": machineID, "dbPath": dbPath})
}
