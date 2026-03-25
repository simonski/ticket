package store

import (
	"database/sql"
	"errors"
)

var ErrTimeEntryNotFound = errors.New("time entry not found")

type TimeEntry struct {
	ID        int64  `json:"time_entry_id"`
	TicketID  string `json:"ticket_id"`
	UserID    string `json:"user_id"`
	Minutes   int    `json:"minutes"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

func LogTime(db *sql.DB, ticketID string, userID string, minutes int, note string) (TimeEntry, error) {
	if minutes <= 0 {
		return TimeEntry{}, errors.New("minutes must be positive")
	}
	result, err := db.Exec(`INSERT INTO time_entries (ticket_id, user_id, minutes, note) VALUES (?, ?, ?, ?)`, ticketID, userID, minutes, note)
	if err != nil {
		return TimeEntry{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return TimeEntry{}, err
	}
	return GetTimeEntry(db, id)
}

func GetTimeEntry(db *sql.DB, id int64) (TimeEntry, error) {
	var entry TimeEntry
	err := db.QueryRow(`SELECT time_entry_id, ticket_id, user_id, minutes, note, created_at FROM time_entries WHERE time_entry_id = ?`, id).
		Scan(&entry.ID, &entry.TicketID, &entry.UserID, &entry.Minutes, &entry.Note, &entry.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TimeEntry{}, ErrTimeEntryNotFound
	}
	return entry, err
}

func ListTimeEntries(db *sql.DB, ticketID string) ([]TimeEntry, error) {
	rows, err := db.Query(`SELECT time_entry_id, ticket_id, user_id, minutes, note, created_at FROM time_entries WHERE ticket_id = ? ORDER BY created_at`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]TimeEntry, 0)
	for rows.Next() {
		var entry TimeEntry
		if err := rows.Scan(&entry.ID, &entry.TicketID, &entry.UserID, &entry.Minutes, &entry.Note, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func DeleteTimeEntry(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM time_entries WHERE time_entry_id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTimeEntryNotFound
	}
	return nil
}

func TotalTimeForTicket(db *sql.DB, ticketID string) (int, error) {
	var total sql.NullInt64
	err := db.QueryRow(`SELECT SUM(minutes) FROM time_entries WHERE ticket_id = ?`, ticketID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return int(total.Int64), nil
}
