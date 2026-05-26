package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"xrouter/internal/store"
)

func TestWebsocketProxyMetadata(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"websocket":true`) {
		t.Fatalf("unexpected metadata: %s", rec.Body.String())
	}
}

func TestWebsocketProxyPingPong(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	ts := httptest.NewServer(srv)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(wsRelayMessage{ID: "ping-1", Type: "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	var msg wsRelayMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if msg.Type != "pong" || msg.ID != "ping-1" {
		t.Fatalf("unexpected pong: %#v", msg)
	}
}

func TestWebsocketProxyHTTPRequestResponse(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": "chatcmpl_ws_1"})
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{Provider: "openai", Name: "ws openai", AuthType: "apikey", APIKey: "x", IsActive: true, ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "openai"}})
	ts := httptest.NewServer(srv)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	payload := map[string]interface{}{"method": "POST", "url": "/v1/chat/completions", "body": mustJSONBody(map[string]interface{}{"model": "openai/gpt-4o-mini", "messages": []map[string]string{{"role": "user", "content": "hi"}}})}
	if err := conn.WriteJSON(wsRelayMessage{ID: "req-1", Type: "http_request", Payload: payload}); err != nil {
		t.Fatalf("write request: %v", err)
	}
	var msg wsRelayMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if msg.Type != "http_response" {
		t.Fatalf("unexpected ws message: %#v", msg)
	}
	if msg.Payload["status"] != float64(200) {
		t.Fatalf("unexpected status payload: %#v", msg.Payload)
	}
	if !strings.Contains(msg.Payload["body"].(string), "chatcmpl_ws_1") {
		t.Fatalf("unexpected body: %#v", msg.Payload)
	}
}

func TestWebsocketProxyStreamMessages(t *testing.T) {
	srv := newTestServer(t)
	_, _ = srv.store.UpdateSettings(map[string]interface{}{"requireApiKey": false})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chunk1\"}\n\n"))
	}))
	defer upstream.Close()
	_, _ = srv.store.CreateProviderConnection(store.ProviderConnection{Provider: "openai", Name: "ws stream", AuthType: "apikey", APIKey: "x", IsActive: true, ProviderSpecificData: map[string]interface{}{"baseUrl": upstream.URL, "apiType": "responses"}, DefaultModel: "openai/gpt-4o-mini"})
	ts := httptest.NewServer(srv)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	payload := map[string]interface{}{"method": "POST", "url": "/v1/responses/stream", "body": mustJSONBody(map[string]interface{}{"model": "openai/gpt-4o-mini", "input": "hi", "stream": true})}
	if err := conn.WriteJSON(wsRelayMessage{ID: "req-stream", Type: "http_request", Payload: payload}); err != nil {
		t.Fatalf("write request: %v", err)
	}
	seenStart, seenChunk, seenEnd := false, false, false
	for i := 0; i < 8; i++ {
		var msg wsRelayMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("read ws message: %v", err)
		}
		switch msg.Type {
		case "stream_start":
			seenStart = true
		case "stream_chunk":
			if data, ok := msg.Payload["data"].(string); ok && strings.Contains(data, "chunk1") {
				seenChunk = true
			}
		case "stream_end":
			seenEnd = true
		}
		if seenStart && seenChunk && seenEnd {
			break
		}
	}
	if !seenStart || !seenChunk || !seenEnd {
		t.Fatalf("unexpected stream flags start=%v chunk=%v end=%v", seenStart, seenChunk, seenEnd)
	}
}
