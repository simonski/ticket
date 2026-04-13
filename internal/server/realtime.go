package server

import (
	"bufio"
	"crypto/sha1" // #nosec G505 -- SHA-1 is required by the WebSocket handshake spec (RFC 6455)
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type liveEvent struct {
	Type       string `json:"type"`
	EntityType string `json:"entity_type,omitempty"`
	EntityID   any    `json:"entity_id,omitempty"`
	ChangeType string `json:"change_type,omitempty"`
	ProjectID  int64  `json:"project_id,omitempty"`
	TicketID   string `json:"ticket_id,omitempty"`
	At         string `json:"at"`
}

type liveHub struct {
	mu      sync.RWMutex
	clients map[*liveClient]struct{}
}

type liveClient struct {
	conn      net.Conn
	send      chan []byte
	done      chan struct{}
	once      sync.Once
	projectID int64 // if set, only receive events for this project
}

func newLiveHub() *liveHub {
	return &liveHub{
		clients: make(map[*liveClient]struct{}),
	}
}

func (h *liveHub) add(conn net.Conn) *liveClient {
	client := &liveClient{
		conn: conn,
		send: make(chan []byte, 32),
		done: make(chan struct{}),
	}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	return client
}

func (h *liveHub) remove(client *liveClient) {
	client.close()
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
}

func (h *liveHub) broadcast(event liveEvent) {
	event.At = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		// Skip clients subscribed to a different project.
		if client.projectID != 0 && event.ProjectID != 0 && client.projectID != event.ProjectID {
			continue
		}
		select {
		case client.send <- payload:
		default:
		}
	}
}

func (c *liveClient) close() {
	c.once.Do(func() {
		close(c.done)
		if err := c.conn.Close(); err != nil {
			log.Printf("server: close live websocket connection: %v", err)
		}
	})
}

func websocketServe(hub *liveHub, w http.ResponseWriter, r *http.Request) error {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		return err
	}
	client := hub.add(conn)

	go func() {
		defer hub.remove(client)
		for {
			select {
			case <-client.done:
				return
			case payload := <-client.send:
				if err := writeWebSocketFrame(client.conn, 0x1, payload); err != nil {
					return
				}
			}
		}
	}()

	if err := writeWebSocketFrame(client.conn, 0x1, []byte(`{"type":"connected"}`)); err != nil {
		hub.remove(client)
		return err
	}

	for {
		opcode, payload, err := readWebSocketFrame(client.conn)
		if err != nil {
			hub.remove(client)
			return nil
		}
		switch opcode {
		case 0x8:
			if err := writeWebSocketFrame(client.conn, 0x8, nil); err != nil {
				log.Printf("server: write websocket close frame: %v", err)
			}
			hub.remove(client)
			return nil
		case 0x9:
			if err := writeWebSocketFrame(client.conn, 0xA, payload); err != nil {
				hub.remove(client)
				return nil
			}
		case 0x1: // text frame — handle subscribe messages
			var msg struct {
				Type      string `json:"type"`
				ProjectID int64  `json:"project_id"`
			}
			if json.Unmarshal(payload, &msg) == nil && msg.Type == "subscribe" && msg.ProjectID > 0 {
				client.projectID = msg.ProjectID
			}
		}
	}
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	// Validate Origin header to prevent cross-origin WebSocket hijacking.
	// Browsers always send Origin on WebSocket upgrades; reject if it doesn't
	// match the Host the server is serving.
	if origin := r.Header.Get("Origin"); origin != "" {
		originHost := origin
		if idx := strings.Index(origin, "://"); idx >= 0 {
			originHost = origin[idx+3:]
		}
		// Strip any path component from the origin host.
		if idx := strings.Index(originHost, "/"); idx >= 0 {
			originHost = originHost[:idx]
		}
		requestHost := r.Host
		// Strip port from requestHost if originHost has no port, to allow
		// browser connections where the port is implied by the scheme.
		if !strings.Contains(originHost, ":") {
			if idx := strings.LastIndex(requestHost, ":"); idx >= 0 {
				requestHost = requestHost[:idx]
			}
		}
		if !strings.EqualFold(originHost, requestHost) {
			http.Error(w, "forbidden: cross-origin WebSocket not allowed", http.StatusForbidden)
			return nil, fmt.Errorf("websocket origin %q does not match host %q", origin, r.Host)
		}
	}
	if !headerContainsToken(r.Header, "Connection", "upgrade") || !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return nil, errors.New("not a websocket upgrade request")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer does not support hijacking")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	accept := websocketAcceptKey(key)
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\n"); err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(rw, "Upgrade: websocket\r\n"); err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(rw, "Connection: Upgrade\r\n"); err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(rw, "Sec-WebSocket-Accept: %s\r\n\r\n", accept); err != nil {
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("server: close websocket conn after flush failure: %v", closeErr)
		}
		return nil, err
	}
	return conn, nil
}

func websocketAcceptKey(key string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(key) + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")) // #nosec G401 -- SHA-1 is mandated by the WebSocket protocol (RFC 6455 §4.2.2)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func headerContainsToken(header http.Header, key, token string) bool {
	values := header.Values(key)
	token = strings.ToLower(strings.TrimSpace(token))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if strings.ToLower(strings.TrimSpace(part)) == token {
				return true
			}
		}
	}
	return false
}

func readWebSocketFrame(conn net.Conn) (byte, []byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	payloadLen := int(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(conn, ext[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(conn, ext[:]); err != nil {
			return 0, nil, err
		}
		n := binary.BigEndian.Uint64(ext[:])
		if n > 1<<31-1 {
			return 0, nil, errors.New("websocket payload too large")
		}
		payloadLen = int(n)
	}
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(conn, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return opcode, payload, nil
}

func writeWebSocketFrame(conn net.Conn, opcode byte, payload []byte) error {
	var b strings.Builder
	b.WriteByte(0x80 | opcode)
	length := len(payload)
	switch {
	case length < 126:
		b.WriteByte(byte(length))
	case length <= 0xFFFF:
		b.WriteByte(126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(length))
		b.Write(ext[:])
	default:
		b.WriteByte(127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(length))
		b.Write(ext[:])
	}
	if _, err := io.Copy(conn, strings.NewReader(b.String())); err != nil {
		return err
	}
	if length == 0 {
		return nil
	}
	writer := bufio.NewWriter(conn)
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	return writer.Flush()
}
