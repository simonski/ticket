package server

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

// registerRoomHandlers wires the multiplayer-rooms REST API (TK-118 / S3).
func (r *router) registerRoomHandlers() {
	db := r.db
	mux := r.mux

	// Collection: list + create.
	mux.HandleFunc("/api/rooms", func(w http.ResponseWriter, req *http.Request) {
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		switch req.Method {
		case http.MethodGet:
			filter := store.RoomFilter{MemberID: user.ID}
			q := req.URL.Query()
			if q.Get("scope") == "global" {
				filter.GlobalOnly = true
			}
			if pid := strings.TrimSpace(q.Get("project_id")); pid != "" {
				if v, perr := strconv.ParseInt(pid, 10, 64); perr == nil {
					filter.ProjectID = &v
				}
			}
			if tk := strings.TrimSpace(q.Get("ticket_id")); tk != "" {
				filter.TicketID = tk
			}
			rooms, lerr := store.ListRooms(req.Context(), db, filter)
			if lerr != nil {
				writeStoreError(w, lerr)
				return
			}
			if rooms == nil {
				rooms = []store.Room{}
			}
			writeJSON(w, http.StatusOK, rooms)
		case http.MethodPost:
			var body struct {
				Name       string `json:"name"`
				Topic      string `json:"topic"`
				Visibility string `json:"visibility"`
				ProjectID  *int64 `json:"project_id"`
				TicketID   string `json:"ticket_id"`
			}
			if derr := decodeJSONBody(req, &body); derr != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			room, cerr := store.CreateRoom(req.Context(), db, store.Room{
				Name:       body.Name,
				Topic:      body.Topic,
				Visibility: body.Visibility,
				ProjectID:  body.ProjectID,
				TicketID:   body.TicketID,
				CreatedBy:  user.ID,
			})
			if cerr != nil {
				writeError(w, http.StatusBadRequest, cerr.Error())
				return
			}
			writeJSON(w, http.StatusCreated, room)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Item + subresources.
	mux.HandleFunc("/api/rooms/", func(w http.ResponseWriter, req *http.Request) {
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/rooms/"), "/")
		parts := strings.Split(trimmed, "/")
		roomID, perr := strconv.ParseInt(parts[0], 10, 64)
		if perr != nil || roomID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		room, gerr := store.GetRoom(req.Context(), db, roomID)
		if gerr != nil {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		isMember, _ := store.IsRoomMember(req.Context(), db, roomID, user.ID)
		if room.Visibility == "private" && !isMember {
			writeError(w, http.StatusForbidden, "not a member of this room")
			return
		}

		sub := ""
		if len(parts) > 1 {
			sub = parts[1]
		}
		switch sub {
		case "":
			handleRoomItem(w, req, db, user, room)
		case "join":
			if req.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if jerr := store.JoinRoom(req.Context(), db, roomID, user.ID, "member"); jerr != nil {
				writeStoreError(w, jerr)
				return
			}
			fresh, _ := store.GetRoom(req.Context(), db, roomID)
			writeJSON(w, http.StatusOK, fresh)
		case "leave":
			if req.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if lerr := store.LeaveRoom(req.Context(), db, roomID, user.ID); lerr != nil {
				writeStoreError(w, lerr)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
		case "members":
			handleRoomMembers(w, req, db, roomID)
		case "messages":
			handleRoomMessages(w, req, db, user, roomID, r.live)
		default:
			writeError(w, http.StatusNotFound, "unknown room subresource")
		}
	})
}

func handleRoomItem(w http.ResponseWriter, req *http.Request, db *sql.DB, user store.User, room store.Room) {
	switch req.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, room)
	case http.MethodPatch:
		if room.CreatedBy != user.ID && user.Role != "admin" {
			writeError(w, http.StatusForbidden, "only the room owner or an admin can edit a room")
			return
		}
		var body struct {
			Name       *string `json:"name"`
			Topic      *string `json:"topic"`
			Visibility *string `json:"visibility"`
		}
		if derr := decodeJSONBody(req, &body); derr != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		name, topic, visibility := room.Name, room.Topic, room.Visibility
		if body.Name != nil {
			name = *body.Name
		}
		if body.Topic != nil {
			topic = *body.Topic
		}
		if body.Visibility != nil {
			visibility = *body.Visibility
		}
		updated, uerr := store.UpdateRoom(req.Context(), db, room.ID, name, topic, visibility)
		if uerr != nil {
			writeError(w, http.StatusBadRequest, uerr.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if room.CreatedBy != user.ID && user.Role != "admin" {
			writeError(w, http.StatusForbidden, "only the room owner or an admin can archive a room")
			return
		}
		if aerr := store.ArchiveRoom(req.Context(), db, room.ID); aerr != nil {
			writeStoreError(w, aerr)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleRoomMembers(w http.ResponseWriter, req *http.Request, db *sql.DB, roomID int64) {
	switch req.Method {
	case http.MethodGet:
		members, err := store.ListRoomMembers(req.Context(), db, roomID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if members == nil {
			members = []store.RoomMember{}
		}
		writeJSON(w, http.StatusOK, members)
	case http.MethodPost:
		var body struct {
			MemberID string `json:"member_id"`
		}
		if derr := decodeJSONBody(req, &body); derr != nil || strings.TrimSpace(body.MemberID) == "" {
			writeError(w, http.StatusBadRequest, "member_id is required")
			return
		}
		if jerr := store.JoinRoom(req.Context(), db, roomID, body.MemberID, "member"); jerr != nil {
			writeStoreError(w, jerr)
			return
		}
		members, _ := store.ListRoomMembers(req.Context(), db, roomID)
		writeJSON(w, http.StatusCreated, members)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleRoomMessages(w http.ResponseWriter, req *http.Request, db *sql.DB, user store.User, roomID int64, hub *liveHub) {
	switch req.Method {
	case http.MethodGet:
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		before, _ := strconv.ParseInt(req.URL.Query().Get("before"), 10, 64)
		msgs, err := store.ListRoomMessages(req.Context(), db, roomID, limit, before)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if msgs == nil {
			msgs = []store.RoomMessage{}
		}
		writeJSON(w, http.StatusOK, msgs)
	case http.MethodPost:
		var body struct {
			Body string `json:"body"`
			Kind string `json:"kind"`
		}
		if derr := decodeJSONBody(req, &body); derr != nil || strings.TrimSpace(body.Body) == "" {
			writeError(w, http.StatusBadRequest, "message body is required")
			return
		}
		msg, err := store.PostRoomMessage(req.Context(), db, store.RoomMessage{
			RoomID:   roomID,
			SenderID: user.ID,
			Kind:     body.Kind,
			Body:     body.Body,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Fan the message out to live room subscribers (S4 hub).
		if hub != nil {
			hub.broadcastRoomMessage(roomID, msg)
		}
		writeJSON(w, http.StatusCreated, msg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
