package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	UserNotificationKindProjectAccessApproved = "project_access_approved"
	UserNotificationKindProjectAccessRejected = "project_access_rejected"
	UserNotificationKindPRReady               = "pr_ready_for_review"
	UserNotificationKindRefinerRecommended    = "refiner_recommended_ready"

	UserNotificationStatusUnread = "unread"
	UserNotificationStatusRead   = "read"
)

var ErrUserNotificationNotFound = errors.New("user notification not found")

type UserNotification struct {
	ID        int64  `json:"notification_id"`
	UserID    string `json:"user_id"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ReadAt    string `json:"read_at,omitempty"`
}

type UserNotificationCreateParams struct {
	UserID  string
	Kind    string
	Title   string
	Message string
	Payload any
}

func encodeNotificationPayload(payload any) (string, error) {
	if payload == nil {
		return "{}", nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func CreateUserNotification(ctx context.Context, db *sql.DB, params UserNotificationCreateParams) (UserNotification, error) {
	userID := strings.TrimSpace(params.UserID)
	if userID == "" {
		return UserNotification{}, errors.New("user_id is required")
	}
	if strings.TrimSpace(params.Kind) == "" {
		return UserNotification{}, errors.New("notification kind is required")
	}
	if strings.TrimSpace(params.Title) == "" {
		return UserNotification{}, errors.New("notification title is required")
	}
	if _, err := GetUserByID(ctx, db, userID); err != nil {
		return UserNotification{}, err
	}
	payload, err := encodeNotificationPayload(params.Payload)
	if err != nil {
		return UserNotification{}, err
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO user_notifications (user_id, kind, status, title, message, payload_json, read_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL, CURRENT_TIMESTAMP)
	`, userID, strings.TrimSpace(params.Kind), UserNotificationStatusUnread, strings.TrimSpace(params.Title), strings.TrimSpace(params.Message), payload)
	if err != nil {
		return UserNotification{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return UserNotification{}, err
	}
	return GetUserNotificationByID(ctx, db, id)
}

func GetUserNotificationByID(ctx context.Context, db *sql.DB, notificationID int64) (UserNotification, error) {
	row := db.QueryRowContext(ctx, `
		SELECT notification_id, user_id, kind, status, title, message, payload_json,
		       created_at, updated_at, COALESCE(read_at, '')
		FROM user_notifications
		WHERE notification_id = ?
	`, notificationID)
	var notification UserNotification
	err := row.Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Kind,
		&notification.Status,
		&notification.Title,
		&notification.Message,
		&notification.Payload,
		&notification.CreatedAt,
		&notification.UpdatedAt,
		&notification.ReadAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return UserNotification{}, ErrUserNotificationNotFound
	}
	return notification, err
}

func ListUserNotifications(ctx context.Context, db *sql.DB, userID, status string, limit int) ([]UserNotification, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "", UserNotificationStatusUnread, UserNotificationStatusRead:
	default:
		return nil, fmt.Errorf("invalid notification status %q", status)
	}
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT notification_id, user_id, kind, status, title, message, payload_json,
		       created_at, updated_at, COALESCE(read_at, '')
		FROM user_notifications
		WHERE user_id = ?
	`
	args := []any{userID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY notification_id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notifications := make([]UserNotification, 0)
	for rows.Next() {
		var notification UserNotification
		if err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Kind,
			&notification.Status,
			&notification.Title,
			&notification.Message,
			&notification.Payload,
			&notification.CreatedAt,
			&notification.UpdatedAt,
			&notification.ReadAt,
		); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func MarkUserNotificationRead(ctx context.Context, db *sql.DB, notificationID int64, userID string) (UserNotification, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UserNotification{}, errors.New("user_id is required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE user_notifications
		SET status = ?, read_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE notification_id = ? AND user_id = ?
	`, UserNotificationStatusRead, notificationID, userID)
	if err != nil {
		return UserNotification{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return UserNotification{}, err
	}
	if affected == 0 {
		return UserNotification{}, ErrUserNotificationNotFound
	}
	return GetUserNotificationByID(ctx, db, notificationID)
}

func BuildProjectAccessDecisionNotification(request ProjectAccessRequest, decidedByUsername string) UserNotificationCreateParams {
	kind := UserNotificationKindProjectAccessApproved
	title := "Project access approved"
	if request.Status == "rejected" {
		kind = UserNotificationKindProjectAccessRejected
		title = "Project access rejected"
	}
	projectRef := strings.TrimSpace(request.ProjectPrefix)
	if projectRef == "" && request.ProjectID > 0 {
		projectRef = fmt.Sprintf("%d", request.ProjectID)
	}
	message := "Your request"
	if request.ID > 0 {
		message += fmt.Sprintf(" #%d", request.ID)
	}
	message += " for " + projectRef
	if request.ProjectTitle != "" {
		message += " (" + request.ProjectTitle + ")"
	}
	if request.Status == "rejected" {
		message += " was rejected"
	} else {
		message += " was approved"
	}
	if strings.TrimSpace(decidedByUsername) != "" {
		message += " by " + strings.TrimSpace(decidedByUsername)
	}
	if strings.TrimSpace(request.DecisionMessage) != "" {
		message += ". Decision: " + strings.TrimSpace(request.DecisionMessage)
	}
	if strings.TrimSpace(request.Message) != "" {
		message += ". Original request: " + strings.TrimSpace(request.Message)
	}
	return UserNotificationCreateParams{
		UserID:  request.UserID,
		Kind:    kind,
		Title:   title,
		Message: message,
		Payload: map[string]any{
			"request_id":       request.ID,
			"project_id":       request.ProjectID,
			"project_prefix":   request.ProjectPrefix,
			"project_title":    request.ProjectTitle,
			"status":           request.Status,
			"message":          request.Message,
			"decision_message": request.DecisionMessage,
			"decided_by":       strings.TrimSpace(decidedByUsername),
		},
	}
}
