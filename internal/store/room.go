package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// Room is a multiplayer chat room (TK-118). Scope is derived from ProjectID and
// TicketID: both nil/empty = global; ProjectID set = project room; ProjectID +
// TicketID set = a breakout room around an epic/story.
type Room struct {
	ID         int64  `json:"room_id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Topic      string `json:"topic"`
	Visibility string `json:"visibility"` // public | private
	ProjectID  *int64 `json:"project_id,omitempty"`
	TicketID   string `json:"ticket_id,omitempty"` // breakout ticket key, "" when not a breakout
	Archived   bool   `json:"archived"`
	CreatedBy  string `json:"created_by"`
	Attrs      Attrs  `json:"attrs,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Scope returns "global", "project", or "breakout".
func (r Room) Scope() string {
	if r.ProjectID == nil {
		return "global"
	}
	if strings.TrimSpace(r.TicketID) != "" {
		return "breakout"
	}
	return "project"
}

// RoomMember is a participant (human or agent) in a room.
type RoomMember struct {
	RoomID     int64  `json:"room_id"`
	MemberID   string `json:"member_id"`
	Role       string `json:"role"` // owner | member
	JoinedAt   string `json:"joined_at"`
	LastReadAt string `json:"last_read_at"`
}

// RoomMessage is a single message in a room.
type RoomMessage struct {
	ID        int64  `json:"message_id"`
	RoomID    int64  `json:"room_id"`
	SenderID  string `json:"sender_id"`
	Kind      string `json:"kind"` // text | system | task | agent_event
	Body      string `json:"body"`
	Attrs     Attrs  `json:"attrs,omitempty"`
	CreatedAt string `json:"created_at"`
}

// RoomFilter narrows ListRooms. A zero filter lists all non-archived rooms.
type RoomFilter struct {
	ProjectID  *int64 // nil = any scope; non-nil = rooms for that project (incl. breakouts)
	GlobalOnly bool   // only rooms with no project
	TicketID   string // only breakout rooms for this ticket
	MemberID   string // if set, private rooms are included only when this member belongs
}

var roomSlugInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

// SlugifyRoomName produces a url-safe slug from a room name.
func SlugifyRoomName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = roomSlugInvalid.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "room"
	}
	return s
}

func normalizeVisibility(v string) string {
	if strings.ToLower(strings.TrimSpace(v)) == "private" {
		return "private"
	}
	return "public"
}

// CreateRoom inserts a room and joins the creator as its owner.
func CreateRoom(ctx context.Context, db *sql.DB, room Room) (Room, error) {
	name := strings.TrimSpace(room.Name)
	if name == "" {
		return Room{}, fmt.Errorf("room name is required")
	}
	slug := strings.TrimSpace(room.Slug)
	if slug == "" {
		slug = SlugifyRoomName(name)
	}
	attrsJSON, err := marshalAttrs(room.Attrs)
	if err != nil {
		return Room{}, err
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO rooms (slug, name, topic, visibility, project_id, ticket_id, archived, created_by, attrs)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		slug, name, strings.TrimSpace(room.Topic), normalizeVisibility(room.Visibility),
		room.ProjectID, strings.TrimSpace(room.TicketID), strings.TrimSpace(room.CreatedBy), attrsJSON)
	if err != nil {
		return Room{}, fmt.Errorf("create room: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Room{}, err
	}
	if creator := strings.TrimSpace(room.CreatedBy); creator != "" {
		if err := JoinRoom(ctx, db, id, creator, "owner"); err != nil {
			return Room{}, err
		}
	}
	return GetRoom(ctx, db, id)
}

const roomColumns = `room_id, slug, name, topic, visibility, project_id, ticket_id, archived, created_by, attrs, created_at, updated_at`

func scanRoom(s interface{ Scan(...any) error }) (Room, error) {
	var (
		r         Room
		projectID sql.NullInt64
		ticketID  sql.NullString
		archived  int
		attrsJSON string
	)
	if err := s.Scan(&r.ID, &r.Slug, &r.Name, &r.Topic, &r.Visibility, &projectID, &ticketID, &archived, &r.CreatedBy, &attrsJSON, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return Room{}, err
	}
	if projectID.Valid {
		pid := projectID.Int64
		r.ProjectID = &pid
	}
	r.TicketID = ticketID.String
	r.Archived = archived != 0
	attrs, err := parseAttrs(attrsJSON)
	if err != nil {
		return Room{}, err
	}
	r.Attrs = attrs
	return r, nil
}

// GetRoom returns a single room by id.
func GetRoom(ctx context.Context, db *sql.DB, id int64) (Room, error) {
	row := db.QueryRowContext(ctx, `SELECT `+roomColumns+` FROM rooms WHERE room_id = ?`, id)
	r, err := scanRoom(row)
	if err == sql.ErrNoRows {
		return Room{}, fmt.Errorf("room %d not found", id)
	}
	return r, err
}

// ListRooms returns non-archived rooms matching the filter, newest activity first.
func ListRooms(ctx context.Context, db *sql.DB, filter RoomFilter) ([]Room, error) {
	var clauses []string
	var args []any
	clauses = append(clauses, "archived = 0")
	if filter.GlobalOnly {
		clauses = append(clauses, "project_id IS NULL")
	} else if filter.ProjectID != nil {
		clauses = append(clauses, "project_id = ?")
		args = append(args, *filter.ProjectID)
	}
	if strings.TrimSpace(filter.TicketID) != "" {
		clauses = append(clauses, "ticket_id = ?")
		args = append(args, strings.TrimSpace(filter.TicketID))
	}
	// Private rooms are only visible to members.
	if member := strings.TrimSpace(filter.MemberID); member != "" {
		clauses = append(clauses, "(visibility = 'public' OR room_id IN (SELECT room_id FROM room_members WHERE member_id = ?))")
		args = append(args, member)
	} else {
		clauses = append(clauses, "visibility = 'public'")
	}
	query := `SELECT ` + roomColumns + ` FROM rooms WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY updated_at DESC, room_id DESC`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rooms []Room
	for rows.Next() {
		r, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// UpdateRoom updates the mutable room fields (name/topic/visibility).
func UpdateRoom(ctx context.Context, db *sql.DB, id int64, name, topic, visibility string) (Room, error) {
	if strings.TrimSpace(name) == "" {
		return Room{}, fmt.Errorf("room name is required")
	}
	_, err := db.ExecContext(ctx, `UPDATE rooms SET name = ?, topic = ?, visibility = ?, updated_at = CURRENT_TIMESTAMP WHERE room_id = ?`,
		strings.TrimSpace(name), strings.TrimSpace(topic), normalizeVisibility(visibility), id)
	if err != nil {
		return Room{}, err
	}
	return GetRoom(ctx, db, id)
}

// ArchiveRoom soft-deletes a room.
func ArchiveRoom(ctx context.Context, db *sql.DB, id int64) error {
	_, err := db.ExecContext(ctx, `UPDATE rooms SET archived = 1, updated_at = CURRENT_TIMESTAMP WHERE room_id = ?`, id)
	return err
}

// JoinRoom adds a member (idempotent). role defaults to "member".
func JoinRoom(ctx context.Context, db *sql.DB, roomID int64, memberID, role string) error {
	memberID = strings.TrimSpace(memberID)
	if memberID == "" {
		return fmt.Errorf("member id is required")
	}
	if role != "owner" {
		role = "member"
	}
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO room_members (room_id, member_id, role) VALUES (?, ?, ?)`, roomID, memberID, role)
	return err
}

// LeaveRoom removes a member from a room.
func LeaveRoom(ctx context.Context, db *sql.DB, roomID int64, memberID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM room_members WHERE room_id = ? AND member_id = ?`, roomID, strings.TrimSpace(memberID))
	return err
}

// IsRoomMember reports whether a member belongs to a room.
func IsRoomMember(ctx context.Context, db *sql.DB, roomID int64, memberID string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM room_members WHERE room_id = ? AND member_id = ?`, roomID, strings.TrimSpace(memberID)).Scan(&n)
	return n > 0, err
}

// ListRoomMembers returns a room's members, owners first.
func ListRoomMembers(ctx context.Context, db *sql.DB, roomID int64) ([]RoomMember, error) {
	rows, err := db.QueryContext(ctx, `SELECT room_id, member_id, role, joined_at, last_read_at FROM room_members WHERE room_id = ? ORDER BY (role = 'owner') DESC, joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []RoomMember
	for rows.Next() {
		var m RoomMember
		if err := rows.Scan(&m.RoomID, &m.MemberID, &m.Role, &m.JoinedAt, &m.LastReadAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// PostRoomMessage appends a message to a room and bumps the room's updated_at.
func PostRoomMessage(ctx context.Context, db *sql.DB, msg RoomMessage) (RoomMessage, error) {
	if strings.TrimSpace(msg.SenderID) == "" {
		return RoomMessage{}, fmt.Errorf("sender id is required")
	}
	kind := strings.TrimSpace(msg.Kind)
	if kind == "" {
		kind = "text"
	}
	attrsJSON, err := marshalAttrs(msg.Attrs)
	if err != nil {
		return RoomMessage{}, err
	}
	res, err := db.ExecContext(ctx, `INSERT INTO room_messages (room_id, sender_id, kind, body, attrs) VALUES (?, ?, ?, ?, ?)`,
		msg.RoomID, strings.TrimSpace(msg.SenderID), kind, msg.Body, attrsJSON)
	if err != nil {
		return RoomMessage{}, fmt.Errorf("post room message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return RoomMessage{}, err
	}
	if _, err := db.ExecContext(ctx, `UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE room_id = ?`, msg.RoomID); err != nil {
		return RoomMessage{}, err
	}
	return GetRoomMessage(ctx, db, id)
}

// GetRoomMessage returns a single message by id.
func GetRoomMessage(ctx context.Context, db *sql.DB, id int64) (RoomMessage, error) {
	row := db.QueryRowContext(ctx, `SELECT message_id, room_id, sender_id, kind, body, attrs, created_at FROM room_messages WHERE message_id = ?`, id)
	return scanRoomMessage(row)
}

func scanRoomMessage(s interface{ Scan(...any) error }) (RoomMessage, error) {
	var (
		m         RoomMessage
		attrsJSON string
	)
	if err := s.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Kind, &m.Body, &attrsJSON, &m.CreatedAt); err != nil {
		return RoomMessage{}, err
	}
	attrs, err := parseAttrs(attrsJSON)
	if err != nil {
		return RoomMessage{}, err
	}
	m.Attrs = attrs
	return m, nil
}

// ListRoomMessages returns up to limit messages for a room in chronological
// order. When beforeID > 0 it returns the page of messages strictly older than
// beforeID (for "load earlier" pagination).
func ListRoomMessages(ctx context.Context, db *sql.DB, roomID int64, limit int, beforeID int64) ([]RoomMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []any{roomID}
	where := "room_id = ?"
	if beforeID > 0 {
		where += " AND message_id < ?"
		args = append(args, beforeID)
	}
	args = append(args, limit)
	// Fetch newest-first with the limit, then reverse to chronological order.
	rows, err := db.QueryContext(ctx, `SELECT message_id, room_id, sender_id, kind, body, attrs, created_at FROM room_messages WHERE `+where+` ORDER BY message_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []RoomMessage
	for rows.Next() {
		m, err := scanRoomMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}
