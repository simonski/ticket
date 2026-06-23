package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
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
			handleRoomMessages(w, req, db, user, room, r.live)
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

func handleRoomMessages(w http.ResponseWriter, req *http.Request, db *sql.DB, user store.User, room store.Room, hub *liveHub) {
	roomID := room.ID
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
		// "/task [@agent] <description>" creates a tracked ticket assigned to the
		// agent and posts a task message linking it (S7).
		if assignee, desc, isTask := parseTaskCommand(body.Body); isTask {
			msg, terr := createRoomTask(req.Context(), db, room, user, assignee, desc)
			if terr != nil {
				writeError(w, http.StatusBadRequest, terr.Error())
				return
			}
			if hub != nil {
				hub.broadcastRoomMessage(roomID, msg)
			}
			writeJSON(w, http.StatusCreated, msg)
			return
		}
		attrs := store.Attrs{}
		if mentions := parseMentions(body.Body); len(mentions) > 0 {
			attrs["mentions"] = mentions
		}
		msg, err := store.PostRoomMessage(req.Context(), db, store.RoomMessage{
			RoomID:   roomID,
			SenderID: user.ID,
			Kind:     body.Kind,
			Body:     body.Body,
			Attrs:    attrs,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Fan the message out to live room subscribers (S4 hub).
		if hub != nil {
			hub.broadcastRoomMessage(roomID, msg)
		}
		// If an agent member was @mentioned, let it reply asynchronously (TK-131).
		go replyAsAgents(context.Background(), db, room, msg, hub)
		writeJSON(w, http.StatusCreated, msg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// parseTaskCommand parses "/task [@assignee] <description>". ok is false when the
// body is not a /task command.
func parseTaskCommand(body string) (assignee, description string, ok bool) {
	s := strings.TrimSpace(body)
	if s != "/task" && !strings.HasPrefix(s, "/task ") {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, "/task"))
	if strings.HasPrefix(rest, "@") {
		fields := strings.SplitN(rest, " ", 2)
		assignee = strings.TrimPrefix(fields[0], "@")
		if len(fields) > 1 {
			rest = strings.TrimSpace(fields[1])
		} else {
			rest = ""
		}
	}
	return assignee, rest, true
}

// createRoomTask creates a tracked ticket from a /task command and posts a task
// message into the room linking it. The existing orchestrator/agent loop then
// picks the ticket up.
func createRoomTask(ctx context.Context, db *sql.DB, room store.Room, user store.User, assignee, description string) (store.RoomMessage, error) {
	if room.ProjectID == nil {
		return store.RoomMessage{}, fmt.Errorf("tasking requires a project or breakout room")
	}
	if strings.TrimSpace(description) == "" {
		return store.RoomMessage{}, fmt.Errorf("describe the work after /task")
	}
	var parent *string
	if strings.TrimSpace(room.TicketID) != "" {
		tid := room.TicketID
		parent = &tid
	}
	ticket, err := store.CreateTicket(ctx, db, store.TicketCreateParams{
		ProjectID: *room.ProjectID,
		ParentID:  parent,
		Type:      "task",
		Title:     description,
		Assignee:  assignee,
		CreatedBy: user.ID,
		Author:    user.Username,
	})
	if err != nil {
		return store.RoomMessage{}, err
	}
	label := "📋 Created " + ticket.ID + ": " + description
	if assignee != "" {
		label += " — assigned @" + assignee
	}
	return store.PostRoomMessage(ctx, db, store.RoomMessage{
		RoomID:   room.ID,
		SenderID: user.ID,
		Kind:     "task",
		Body:     label,
		Attrs:    store.Attrs{"task_id": ticket.ID, "assignee": assignee},
	})
}

var roomMentionRe = regexp.MustCompile(`@([A-Za-z0-9][A-Za-z0-9_-]*)`)

// parseMentions extracts unique @-mentioned names from a message body.
func parseMentions(body string) []string {
	matches := roomMentionRe.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}
