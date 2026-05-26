package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type wsRelayMessage struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

func (s *Server) handleWebsocketProxy(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"path":               "/api/v1/ws",
			"websocket":          true,
			"protocol":           "xrouter.wsrelay.v1",
			"supportedMessages":  []string{"http_request", "http_response", "stream_start", "stream_chunk", "stream_end", "error", "ping", "pong"},
			"upgradeRequired":    true,
			"streamingSupported": true,
		})
		return
	}
	apiKey, err := s.authorize(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(16 * 1024 * 1024)
	for {
		var msg wsRelayMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if strings.TrimSpace(msg.ID) == "" {
			msg.ID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
		}
		switch msg.Type {
		case "ping":
			_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "pong"})
		case "http_request":
			s.handleWSHTTPRelayMessage(conn, apiKey.ID, msg)
		default:
			_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "error", Payload: map[string]interface{}{"error": "unsupported websocket message type", "status": http.StatusBadRequest}})
		}
	}
}

func (s *Server) handleWSHTTPRelayMessage(conn *websocket.Conn, apiKeyID string, msg wsRelayMessage) {
	payload := msg.Payload
	method := strings.ToUpper(strings.TrimSpace(stringPayload(payload, "method", "POST")))
	if method == "" {
		method = http.MethodPost
	}
	path := strings.TrimSpace(stringPayload(payload, "url", "/v1/chat/completions"))
	if path == "" {
		path = "/v1/chat/completions"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	body := []byte(stringPayload(payload, "body", ""))
	if method != http.MethodPost {
		_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "error", Payload: map[string]interface{}{"error": "only POST is supported", "status": http.StatusMethodNotAllowed}})
		return
	}
	resp, err := s.forwarder.Forward(context.Background(), apiKeyID, path, body)
	if err != nil {
		_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "error", Payload: map[string]interface{}{"error": err.Error(), "status": http.StatusBadGateway}})
		return
	}
	defer resp.Body.Close()
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "stream_start", Payload: map[string]interface{}{"status": resp.StatusCode, "headers": headersToPayload(resp.Header)}})
		reader := bufio.NewReader(resp.Body)
		for {
			chunk, err := reader.ReadBytes('\n')
			if len(chunk) > 0 {
				_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "stream_chunk", Payload: map[string]interface{}{"data": string(chunk)}})
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "error", Payload: map[string]interface{}{"error": err.Error(), "status": http.StatusBadGateway}})
				return
			}
		}
		_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "stream_end"})
		return
	}
	rawResp, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "error", Payload: map[string]interface{}{"error": "failed to read upstream response", "status": http.StatusBadGateway}})
		return
	}
	_ = conn.WriteJSON(wsRelayMessage{ID: msg.ID, Type: "http_response", Payload: map[string]interface{}{"status": resp.StatusCode, "headers": headersToPayload(resp.Header), "body": string(rawResp)}})
}

func headersToPayload(headers http.Header) map[string]interface{} {
	out := make(map[string]interface{}, len(headers))
	for key, values := range headers {
		copyValues := make([]string, len(values))
		copy(copyValues, values)
		out[key] = copyValues
	}
	return out
}

func stringPayload(payload map[string]interface{}, key, fallback string) string {
	if payload == nil {
		return fallback
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return fallback
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func mustJSONBody(v interface{}) string {
	buf := bytes.Buffer{}
	_ = json.NewEncoder(&buf).Encode(v)
	return strings.TrimSpace(buf.String())
}
