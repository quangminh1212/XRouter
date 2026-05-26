package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func maskSecretPreview(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return ""
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func detectProviderFromCredentialPath(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(lower, "anthropic") || strings.Contains(lower, "claude"):
		return "claude"
	case strings.Contains(lower, "openai") || strings.Contains(lower, "codex"):
		return "codex"
	case strings.Contains(lower, "gemini") || strings.Contains(lower, "google"):
		return "gemini"
	case strings.Contains(lower, "kimi"):
		return "kimi"
	case strings.Contains(lower, "grok") || strings.Contains(lower, "xai"):
		return "xai"
	default:
		return ""
	}
}

func extractCredentialPreviews(content string) []string {
	var out []string
	var parsed interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		var walk func(interface{})
		walk = func(v interface{}) {
			switch item := v.(type) {
			case map[string]interface{}:
				for key, value := range item {
					lower := strings.ToLower(key)
					if str, ok := value.(string); ok && (strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "secret")) {
						if preview := maskSecretPreview(str); preview != "" {
							out = append(out, preview)
						}
					}
					walk(value)
				}
			case []interface{}:
				for _, value := range item {
					walk(value)
				}
			}
		}
		walk(parsed)
		return out
	}
	for _, line := range strings.Split(content, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "token") && !strings.Contains(lower, "key") && !strings.Contains(lower, "secret") {
			continue
		}
		parts := strings.FieldsFunc(line, func(r rune) bool { return r == '=' || r == ':' || r == '"' || r == '\'' || r == ' ' || r == '\t' })
		for _, part := range parts {
			if preview := maskSecretPreview(part); preview != "" {
				out = append(out, preview)
			}
		}
	}
	return out
}

func candidateCredentialFiles(home string) []string {
	return []string{
		filepath.Join(home, ".codex", "auth.json"),
		filepath.Join(home, ".config", "codex", "auth.json"),
		filepath.Join(home, ".claude", ".credentials.json"),
		filepath.Join(home, ".config", "claude", "credentials.json"),
		filepath.Join(home, ".gemini", "oauth_creds.json"),
		filepath.Join(home, ".config", "gemini", "oauth_creds.json"),
		filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"),
	}
}

func (s *Server) handleOAuthProviderScanLocal(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve home directory"})
		return
	}
	var matches []map[string]interface{}
	for _, path := range candidateCredentialFiles(home) {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() > 512*1024 {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		provider := detectProviderFromCredentialPath(path)
		matches = append(matches, map[string]interface{}{
			"path":           path,
			"provider":       provider,
			"size":           info.Size(),
			"secretPreviews": extractCredentialPreviews(string(raw)),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"matches": matches})
}
