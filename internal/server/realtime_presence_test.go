package server

import (
	"encoding/json"
	"testing"
)

// addTestClient inserts a synthetic client into the hub without a real
// connection. Safe for presence/broadcast tests that never touch conn.
func addTestClient(h *liveHub, userID, name string, roomID int64) *liveClient {
	c := &liveClient{
		send:     make(chan []byte, 8),
		done:     make(chan struct{}),
		roomID:   roomID,
		userID:   userID,
		userName: name,
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func TestPresenceForDedupesAndSorts(t *testing.T) {
	h := newLiveHub()
	// Two connections for the same user in room 1 should collapse to one entry.
	addTestClient(h, "u-bob", "Bob", 1)
	addTestClient(h, "u-bob", "Bob", 1)
	addTestClient(h, "u-amy", "Amy", 1)
	addTestClient(h, "u-cid", "Cid", 2) // different room — excluded

	got := h.presenceFor(1)
	if len(got) != 2 {
		t.Fatalf("presenceFor(1) returned %d users, want 2: %+v", len(got), got)
	}
	if got[0].Name != "Amy" || got[1].Name != "Bob" {
		t.Fatalf("presenceFor not sorted by name: %+v", got)
	}
}

func TestPresenceForIgnoresAnonymousClients(t *testing.T) {
	h := newLiveHub()
	addTestClient(h, "", "", 1) // no identity (e.g. legacy connection)
	addTestClient(h, "u-amy", "Amy", 1)
	if got := h.presenceFor(1); len(got) != 1 {
		t.Fatalf("presenceFor(1) = %+v, want only the identified user", got)
	}
}

func TestBroadcastRoomPresenceReachesRoomSubscribers(t *testing.T) {
	h := newLiveHub()
	inRoom := addTestClient(h, "u-amy", "Amy", 7)
	other := addTestClient(h, "u-bob", "Bob", 9)

	h.broadcastRoomPresence(7)

	select {
	case payload := <-inRoom.send:
		var ev liveEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			t.Fatalf("unmarshal presence event: %v", err)
		}
		if ev.Type != "room_presence" || ev.RoomID != 7 {
			t.Fatalf("unexpected event: %+v", ev)
		}
	default:
		t.Fatal("room subscriber did not receive presence event")
	}

	select {
	case <-other.send:
		t.Fatal("client in a different room received the presence event")
	default:
	}
}

func TestBroadcastTypingReachesRoomSubscribers(t *testing.T) {
	h := newLiveHub()
	inRoom := addTestClient(h, "u-amy", "Amy", 3)

	h.broadcastTyping(3, presenceUser{UserID: "u-bob", Name: "Bob"})

	select {
	case payload := <-inRoom.send:
		var ev liveEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			t.Fatalf("unmarshal typing event: %v", err)
		}
		if ev.Type != "typing" || ev.RoomID != 3 {
			t.Fatalf("unexpected event: %+v", ev)
		}
	default:
		t.Fatal("room subscriber did not receive typing event")
	}
}
