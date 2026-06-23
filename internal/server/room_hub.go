package server

import "github.com/simonski/ticket/internal/store"

// broadcastRoomMessage delivers a freshly-posted message to live room
// subscribers. It is a no-op until the room WebSocket hub is implemented in
// TK-118 / S4, which will replace this with real fan-out.
func broadcastRoomMessage(roomID int64, msg store.RoomMessage) {
	_ = roomID
	_ = msg
}
