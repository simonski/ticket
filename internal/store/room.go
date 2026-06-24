package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
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
	// Transient, populated per-request by the API layer.
	MemberCount int  `json:"member_count"`
	Unread      bool `json:"unread"`
	UnreadCount int  `json:"unread_count"`
	Permanent   bool `json:"permanent"` // global + project primary rooms cannot be left
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
	ID         int64  `json:"message_id"`
	RoomID     int64  `json:"room_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Kind       string `json:"kind"` // text | system | task | agent_event
	Body       string `json:"body"`
	Attrs      Attrs  `json:"attrs,omitempty"`
	CreatedAt  string `json:"created_at"`
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
	// #nosec G202 -- roomColumns is a fixed column list; clauses are constant predicate strings and all values are bound via ? placeholders.
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
// ErrRoomPermanent is returned when a member tries to leave a non-leavable room
// (the global public room or a project's primary room).
var ErrRoomPermanent = errors.New("this room cannot be left")

func LeaveRoom(ctx context.Context, db *sql.DB, roomID int64, memberID string) error {
	room, err := GetRoom(ctx, db, roomID)
	if err != nil {
		return err
	}
	permanent, perr := RoomIsPermanent(ctx, db, room)
	if perr != nil {
		return perr
	}
	if permanent {
		return ErrRoomPermanent
	}
	_, derr := db.ExecContext(ctx, `DELETE FROM room_members WHERE room_id = ? AND member_id = ?`, roomID, strings.TrimSpace(memberID))
	return derr
}

// RemoveRoomMember removes a member without the permanence guard. Used for
// administrative removal (kick), which is allowed even on permanent rooms —
// unlike voluntarily leaving via LeaveRoom.
func RemoveRoomMember(ctx context.Context, db *sql.DB, roomID int64, memberID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM room_members WHERE room_id = ? AND member_id = ?`, roomID, strings.TrimSpace(memberID))
	return err
}

// RoomIsPermanent reports whether a room is the primary (non-leavable) room for
// its scope: the lowest-id public global room, or a project's lowest-id public
// room. Breakout (ticket) and private rooms are always leavable.
func RoomIsPermanent(ctx context.Context, db *sql.DB, room Room) (bool, error) {
	if room.Archived || room.Visibility != "public" || strings.TrimSpace(room.TicketID) != "" {
		return false, nil
	}
	var primaryID int64
	var err error
	if room.ProjectID == nil {
		err = db.QueryRowContext(ctx, `SELECT room_id FROM rooms WHERE project_id IS NULL AND visibility = 'public' AND archived = 0 ORDER BY room_id LIMIT 1`).Scan(&primaryID)
	} else {
		err = db.QueryRowContext(ctx, `SELECT room_id FROM rooms WHERE project_id = ? AND ticket_id = '' AND visibility = 'public' AND archived = 0 ORDER BY room_id LIMIT 1`, *room.ProjectID).Scan(&primaryID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return primaryID == room.ID, nil
}

// RoomUnreadCounts returns, per room the member belongs to, how many messages
// arrived after their last read (for the unread badge).
func RoomUnreadCounts(ctx context.Context, db *sql.DB, memberID string) (map[int64]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rm.room_id, COUNT(msg.message_id)
		FROM room_members rm
		JOIN room_messages msg ON msg.room_id = rm.room_id
		WHERE rm.member_id = ? AND (rm.last_read_at = '' OR msg.created_at > rm.last_read_at)
		GROUP BY rm.room_id`, strings.TrimSpace(memberID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if serr := rows.Scan(&id, &n); serr != nil {
			return nil, serr
		}
		out[id] = n
	}
	return out, rows.Err()
}

// FindRoomByName resolves a visible room by name or slug (case-insensitive),
// preferring the current project scope, then global, that the member can see.
func FindRoomByName(ctx context.Context, db *sql.DB, name, memberID string, projectID *int64) (Room, error) {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		return Room{}, errors.New("room name is required")
	}
	rows, err := db.QueryContext(ctx, `SELECT `+roomColumns+` FROM rooms
		WHERE archived = 0 AND (LOWER(name) = ? OR LOWER(slug) = ?)
		  AND (visibility = 'public' OR room_id IN (SELECT room_id FROM room_members WHERE member_id = ?))
		ORDER BY (project_id IS NOT NULL) DESC, room_id ASC`, needle, needle, strings.TrimSpace(memberID))
	if err != nil {
		return Room{}, err
	}
	defer rows.Close()
	var match *Room
	for rows.Next() {
		r, serr := scanRoom(rows)
		if serr != nil {
			return Room{}, serr
		}
		// Prefer a room in the active project; otherwise take the first.
		if projectID != nil && r.ProjectID != nil && *r.ProjectID == *projectID {
			return r, nil
		}
		if match == nil {
			rc := r
			match = &rc
		}
	}
	if rerr := rows.Err(); rerr != nil {
		return Room{}, rerr
	}
	if match == nil {
		return Room{}, fmt.Errorf("no room named %q", name)
	}
	return *match, nil
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
	row := db.QueryRowContext(ctx, `SELECT m.message_id, m.room_id, m.sender_id, COALESCE(u.username, m.sender_id), m.kind, m.body, m.attrs, m.created_at FROM room_messages m LEFT JOIN users u ON u.user_id = m.sender_id WHERE m.message_id = ?`, id)
	return scanRoomMessage(row)
}

func scanRoomMessage(s interface{ Scan(...any) error }) (RoomMessage, error) {
	var (
		m         RoomMessage
		attrsJSON string
	)
	if err := s.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.SenderName, &m.Kind, &m.Body, &attrsJSON, &m.CreatedAt); err != nil {
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
	// #nosec G202 -- the SELECT column list is static; `where` is built from constant predicates and all values are bound via ? placeholders.
	rows, err := db.QueryContext(ctx, `SELECT m.message_id, m.room_id, m.sender_id, COALESCE(u.username, m.sender_id), m.kind, m.body, m.attrs, m.created_at FROM room_messages m LEFT JOIN users u ON u.user_id = m.sender_id WHERE `+strings.ReplaceAll(where, "room_id", "m.room_id")+` ORDER BY m.message_id DESC LIMIT ?`, args...)
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

// MarkRoomRead advances a member's read marker to now.
func MarkRoomRead(ctx context.Context, db *sql.DB, roomID int64, memberID string) error {
	_, err := db.ExecContext(ctx, `UPDATE room_members SET last_read_at = CURRENT_TIMESTAMP WHERE room_id = ? AND member_id = ?`, roomID, strings.TrimSpace(memberID))
	return err
}

// UnreadRoomIDs returns the set of rooms the member belongs to that have
// messages newer than the member's read marker.
func UnreadRoomIDs(ctx context.Context, db *sql.DB, memberID string) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT rm.room_id
		FROM room_members rm
		JOIN (SELECT room_id, MAX(created_at) AS last_at FROM room_messages GROUP BY room_id) lm ON lm.room_id = rm.room_id
		WHERE rm.member_id = ? AND (rm.last_read_at = '' OR lm.last_at > rm.last_read_at)`, strings.TrimSpace(memberID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// RoomMemberCounts returns member counts keyed by room id.
func RoomMemberCounts(ctx context.Context, db *sql.DB) (map[int64]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT room_id, COUNT(*) FROM room_members GROUP BY room_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// FindOrCreateDMRoom returns the private 1:1 room between two users, creating it
// (with both as members) if it does not exist.
func FindOrCreateDMRoom(ctx context.Context, db *sql.DB, a, b User) (Room, error) {
	ids := []string{a.ID, b.ID}
	sort.Strings(ids)
	slug := "dm-" + ids[0] + "-" + ids[1]
	row := db.QueryRowContext(ctx, `SELECT `+roomColumns+` FROM rooms WHERE slug = ? AND archived = 0`, slug)
	if room, err := scanRoom(row); err == nil {
		return room, nil
	} else if err != sql.ErrNoRows {
		return Room{}, err
	}
	name := "DM: " + a.Username + ", " + b.Username
	room, err := CreateRoom(ctx, db, Room{Slug: slug, Name: name, Visibility: "private", CreatedBy: a.ID})
	if err != nil {
		return Room{}, err
	}
	if jerr := JoinRoom(ctx, db, room.ID, b.ID, "member"); jerr != nil {
		return Room{}, jerr
	}
	return room, nil
}

// EnsureGlobalGeneralRoom creates a public global "general" room if none exists.
func EnsureGlobalGeneralRoom(ctx context.Context, db *sql.DB, createdBy string) (Room, bool, error) {
	var existingID int64
	err := db.QueryRowContext(ctx, `SELECT room_id FROM rooms WHERE project_id IS NULL AND visibility = 'public' AND archived = 0 ORDER BY room_id LIMIT 1`).Scan(&existingID)
	if err == nil {
		room, gerr := GetRoom(ctx, db, existingID)
		return room, false, gerr
	}
	if err != sql.ErrNoRows {
		return Room{}, false, err
	}
	room, cerr := CreateRoom(ctx, db, Room{Slug: "general", Name: "general", Topic: "Everyone", Visibility: "public", CreatedBy: createdBy})
	return room, cerr == nil, cerr
}

// EnsureProjectRoom creates a default public room for a project if it has none.
func EnsureProjectRoom(ctx context.Context, db *sql.DB, projectID int64, name, createdBy string) (Room, bool, error) {
	var existingID int64
	err := db.QueryRowContext(ctx, `SELECT room_id FROM rooms WHERE project_id = ? AND ticket_id = '' AND archived = 0 ORDER BY room_id LIMIT 1`, projectID).Scan(&existingID)
	if err == nil {
		room, gerr := GetRoom(ctx, db, existingID)
		return room, false, gerr
	}
	if err != sql.ErrNoRows {
		return Room{}, false, err
	}
	pid := projectID
	room, cerr := CreateRoom(ctx, db, Room{Name: name, Topic: "Project room", Visibility: "public", ProjectID: &pid, CreatedBy: createdBy})
	return room, cerr == nil, cerr
}
