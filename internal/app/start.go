package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"xrouter/internal/proxy"
	"xrouter/internal/store"
)

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *Server) handleOAuthProviderStart(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	provider, entry, ok := parseOAuthProviderPath(r.URL.Path, "/start")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown oauth provider"})
		return
	}
	var body struct {
		RedirectURI  string   `json:"redirectUri"`
		ClientID     string   `json:"clientId"`
		ClientSecret string   `json:"clientSecret"`
		AuthorizeURL string   `json:"authorizeUrl"`
		TokenURL     string   `json:"tokenUrl"`
		Scopes       []string `json:"scopes"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	redirectURI := strings.TrimSpace(body.RedirectURI)
	if redirectURI == "" {
		redirectURI = "http://localhost:1213/api/oauth/callback"
	}
	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(entry.ClientID)
	}
	if clientID == "" {
		envName := "XROUTER_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_OAUTH_CLIENT_ID"
		clientID = strings.TrimSpace(os.Getenv(envName))
	}
	authorizeURL := strings.TrimSpace(body.AuthorizeURL)
	if authorizeURL == "" {
		authorizeURL = strings.TrimSpace(entry.AuthorizeURL)
	}
	if authorizeURL == "" {
		envName := "XROUTER_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_OAUTH_AUTHORIZE_URL"
		authorizeURL = strings.TrimSpace(os.Getenv(envName))
	}
	tokenURL := strings.TrimSpace(body.TokenURL)
	if tokenURL == "" {
		tokenURL = strings.TrimSpace(entry.TokenURL)
	}
	if tokenURL == "" {
		envName := "XROUTER_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_OAUTH_TOKEN_URL"
		tokenURL = strings.TrimSpace(os.Getenv(envName))
	}
	scopes := body.Scopes
	if len(scopes) == 0 {
		scopes = entry.Scopes
	}
	if clientID == "" || authorizeURL == "" || tokenURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "clientId, authorizeUrl and tokenUrl are required"})
		return
	}
	verifier := randomHex(32)
	state := randomHex(16)
	u, err := url.Parse(authorizeURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid authorizeUrl"})
		return
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", pkceChallenge(verifier))
	q.Set("code_challenge_method", "S256")
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	u.RawQuery = q.Encode()

	s.oauthMu.Lock()
	s.oauth[state] = oauthSession{
		Provider:     provider,
		State:        state,
		CodeVerifier: verifier,
		RedirectURI:  redirectURI,
		ClientID:     clientID,
		ClientSecret: strings.TrimSpace(body.ClientSecret),
		TokenURL:     tokenURL,
		CreatedAt:    time.Now(),
	}
	s.oauthMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":         provider,
		"state":            state,
		"authorizationUrl": u.String(),
		"redirectUri":      redirectURI,
	})
}

func (s *Server) handleOAuthProviderExchange(w http.ResponseWriter, r *http.Request) {
	if !isLocalOnlyRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "oauth provider management is restricted to localhost"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	provider, _, ok := parseOAuthProviderPath(r.URL.Path, "/exchange")
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown oauth provider"})
		return
	}
	var body struct {
		State string `json:"state"`
		Code  string `json:"code"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1*1024*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	session, ok := s.popOAuthSession(strings.TrimSpace(body.State), provider)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "oauth session not found"})
		return
	}
	result, err := s.exchangeOAuthCode(r.Context(), session, strings.TrimSpace(body.Code))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = provider + " oauth"
	}
	created, err := s.store.CreateProviderConnection(store.ProviderConnection{
		Provider:     provider,
		Name:         name,
		AuthType:     "oauth",
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenExpiry:  result.TokenExpiry,
		IsActive:     true,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	created.APIKey, created.AccessToken, created.RefreshToken = "", "", ""
	writeJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "connection": created})
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing state"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": state, "code": code})
}

func parseOAuthProviderPath(path, suffix string) (string, store.ProviderCatalogEntry, bool) {
	raw := strings.TrimPrefix(strings.TrimSpace(path), "/api/oauth/providers/")
	provider := strings.TrimSuffix(raw, suffix)
	provider = strings.TrimSuffix(provider, "/")
	entry, ok := store.GetProviderCatalogEntry(provider)
	if !ok || entry.AuthType != "oauth" {
		return "", store.ProviderCatalogEntry{}, false
	}
	return provider, entry, true
}

func (s *Server) popOAuthSession(state, provider string) (oauthSession, bool) {
	s.oauthMu.Lock()
	defer s.oauthMu.Unlock()
	session, ok := s.oauth[state]
	if !ok || session.Provider != provider {
		return oauthSession{}, false
	}
	delete(s.oauth, state)
	return session, true
}

func (s *Server) exchangeOAuthCode(ctx context.Context, session oauthSession, code string) (proxy.OAuthRefreshResult, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", session.RedirectURI)
	form.Set("client_id", session.ClientID)
	form.Set("code_verifier", session.CodeVerifier)
	if session.ClientSecret != "" {
		form.Set("client_secret", session.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, session.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.forwarder.HTTPClient().Do(req)
	if err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return proxy.OAuthRefreshResult{}, fmt.Errorf("oauth exchange failed with status %d", resp.StatusCode)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return proxy.OAuthRefreshResult{}, err
	}
	result := proxy.OAuthRefreshResult{
		AccessToken:  strings.TrimSpace(fmt.Sprint(payload["access_token"])),
		RefreshToken: strings.TrimSpace(fmt.Sprint(payload["refresh_token"])),
		TokenType:    strings.TrimSpace(fmt.Sprint(payload["token_type"])),
		Raw:          payload,
	}
	if expiresIn, ok := payload["expires_in"]; ok {
		if seconds, ok := parseInt64Loose(expiresIn); ok && seconds > 0 {
			result.TokenExpiry = time.Now().UTC().Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
		}
	}
	if result.AccessToken == "" || result.AccessToken == "<nil>" {
		return proxy.OAuthRefreshResult{}, fmt.Errorf("oauth exchange response missing access_token")
	}
	if result.RefreshToken == "<nil>" {
		result.RefreshToken = ""
	}
	return result, nil
}
