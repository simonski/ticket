package server

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestDeliverToClientRoomFilter(t *testing.T) {
	cases := []struct {
		name   string
		client *liveClient
		event  liveEvent
		want   bool
	}{
		{"room event to matching room", &liveClient{roomID: 5}, liveEvent{RoomID: 5}, true},
		{"room event to other room", &liveClient{roomID: 9}, liveEvent{RoomID: 5}, false},
		{"room event to non-room client", &liveClient{}, liveEvent{RoomID: 5}, false},
		{"project event respects project filter", &liveClient{projectID: 2}, liveEvent{ProjectID: 3}, false},
		{"project event to matching project", &liveClient{projectID: 3}, liveEvent{ProjectID: 3}, true},
		{"unscoped event to room client still delivered", &liveClient{roomID: 5}, liveEvent{Type: "ping"}, true},
	}
	for _, tc := range cases {
		if got := deliverToClient(tc.client, tc.event); got != tc.want {
			t.Errorf("%s: deliverToClient = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRoomHubBroadcastReachesOnlySubscribers(t *testing.T) {
	hub := newLiveHub()
	connA, _ := net.Pipe()
	connB, _ := net.Pipe()
	defer connA.Close()
	defer connB.Close()

	inRoom := hub.add(connA)
	inRoom.roomID = 5
	elsewhere := hub.add(connB)
	elsewhere.roomID = 9

	hub.broadcastRoomMessage(5, store.RoomMessage{ID: 1, RoomID: 5, SenderID: "alice", Body: "hello"})

	select {
	case payload := <-inRoom.send:
		var ev struct {
			Type    string `json:"type"`
			RoomID  int64  `json:"room_id"`
			Payload struct {
				Body string `json:"body"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(payload, &ev); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if ev.Type != "room_message" || ev.RoomID != 5 || ev.Payload.Body != "hello" {
			t.Fatalf("subscriber event = %+v", ev)
		}
	default:
		t.Fatal("room-5 subscriber did not receive the message")
	}

	select {
	case <-elsewhere.send:
		t.Fatal("room-9 subscriber should not have received the room-5 message")
	default:
	}
}
