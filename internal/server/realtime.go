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
	"sort"
	"strings"

	"github.com/simonski/ticket/internal/store"
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
	RoomID     int64  `json:"room_id,omitempty"`
	Payload    any    `json:"payload,omitempty"`
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
	projectID int64  // if set, only receive events for this project
	roomID    int64  // if set, also receive room events for this room
	userID    string // identity of the connected user (for room presence)
	userName  string // display name of the connected user (for room presence)
}

// presenceUser is one occupant of a room, derived purely from live connections.
type presenceUser struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
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
	roomID := client.roomID
	delete(h.clients, client)
	h.mu.Unlock()
	// A departing client changes who is present in its room.
	if roomID != 0 {
		h.broadcastRoomPresence(roomID)
	}
}

// setSubscription updates a client's project/room subscription under the hub lock
// and returns the room the client was previously in (0 if none). Mutating the
// fields under the lock keeps them consistent with concurrent presence reads.
func (h *liveHub) setSubscription(client *liveClient, projectID, roomID int64) (oldRoom int64) {
	h.mu.Lock()
	oldRoom = client.roomID
	if projectID > 0 {
		client.projectID = projectID
	}
	if roomID > 0 {
		client.roomID = roomID
	}
	h.mu.Unlock()
	return oldRoom
}

// presenceFor returns the distinct users currently subscribed to a room, sorted
// for stable output. Presence is ephemeral — derived only from live connections.
func (h *liveHub) presenceFor(roomID int64) []presenceUser {
	h.mu.RLock()
	seen := make(map[string]string)
	for client := range h.clients {
		if client.roomID == roomID && client.userID != "" {
			if _, ok := seen[client.userID]; !ok {
				seen[client.userID] = client.userName
			}
		}
	}
	h.mu.RUnlock()
	users := make([]presenceUser, 0, len(seen))
	for id, name := range seen {
		users = append(users, presenceUser{UserID: id, Name: name})
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].Name != users[j].Name {
			return users[i].Name < users[j].Name
		}
		return users[i].UserID < users[j].UserID
	})
	return users
}

// broadcastRoomPresence sends the current occupant list to a room's subscribers.
func (h *liveHub) broadcastRoomPresence(roomID int64) {
	if roomID == 0 {
		return
	}
	h.broadcast(liveEvent{Type: "room_presence", RoomID: roomID, Payload: h.presenceFor(roomID)})
}

// presenceName picks the best human-readable label for a user in presence lists.
func presenceName(u store.User) string {
	if strings.TrimSpace(u.DisplayName) != "" {
		return u.DisplayName
	}
	return u.Username
}

// broadcastTyping fans a transient typing notice to a room's subscribers.
func (h *liveHub) broadcastTyping(roomID int64, u presenceUser) {
	if roomID == 0 {
		return
	}
	h.broadcast(liveEvent{Type: "typing", RoomID: roomID, Payload: u})
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
		if !deliverToClient(client, event) {
			continue
		}
		select {
		case client.send <- payload:
		default:
		}
	}
}

// deliverToClient decides whether a live event should be delivered to a client.
// Room events go only to clients subscribed to that room; other events respect
// the optional project subscription filter.
func deliverToClient(client *liveClient, event liveEvent) bool {
	if event.RoomID != 0 {
		return client.roomID == event.RoomID
	}
	if client.projectID != 0 && event.ProjectID != 0 && client.projectID != event.ProjectID {
		return false
	}
	return true
}

// broadcastRoomMessage fans a newly-posted room message out to subscribers.
func (h *liveHub) broadcastRoomMessage(roomID int64, msg store.RoomMessage) {
	h.broadcast(liveEvent{Type: "room_message", RoomID: roomID, Payload: msg})
}

func (c *liveClient) close() {
	c.once.Do(func() {
		close(c.done)
		if err := c.conn.Close(); err != nil {
			log.Printf("server: close live websocket connection: %v", err)
		}
	})
}

func websocketServe(hub *liveHub, w http.ResponseWriter, r *http.Request, userID, userName string) error {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		return err
	}
	client := hub.add(conn)
	client.userID = userID
	client.userName = userName

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
		case 0x1: // text frame — handle subscribe / typing messages
			var msg struct {
				Type      string `json:"type"`
				ProjectID int64  `json:"project_id"`
				RoomID    int64  `json:"room_id"`
			}
			if json.Unmarshal(payload, &msg) != nil {
				continue
			}
			switch msg.Type {
			case "subscribe":
				oldRoom := hub.setSubscription(client, msg.ProjectID, msg.RoomID)
				if msg.RoomID > 0 {
					// Leaving the previous room and joining the new one both
					// change presence; refresh occupants for each affected room.
					if oldRoom != 0 && oldRoom != msg.RoomID {
						hub.broadcastRoomPresence(oldRoom)
					}
					hub.broadcastRoomPresence(msg.RoomID)
				}
			case "typing":
				roomID := msg.RoomID
				if roomID == 0 {
					roomID = client.roomID
				}
				hub.broadcastTyping(roomID, presenceUser{UserID: client.userID, Name: client.userName})
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

func readWebSocketFrame(conn net.Conn) (opcode byte, payload []byte, err error) {
	var header [2]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	opcode = header[0] & 0x0F
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
	payload = make([]byte, payloadLen)
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
