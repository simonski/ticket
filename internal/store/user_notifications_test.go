package store

import (
	"context"
	"testing"
)

func TestCreateListAndReadUserNotifications(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	user, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	created, err := CreateUserNotification(context.Background(), db, UserNotificationCreateParams{
		UserID:  user.ID,
		Kind:    UserNotificationKindProjectAccessApproved,
		Title:   "Project access approved",
		Message: "Your request for GATE was approved.",
		Payload: map[string]any{"project_prefix": "GATE"},
	})
	if err != nil {
		t.Fatalf("CreateUserNotification() error = %v", err)
	}
	if created.Status != UserNotificationStatusUnread {
		t.Fatalf("CreateUserNotification().Status = %q, want %q", created.Status, UserNotificationStatusUnread)
	}

	notifications, err := ListUserNotifications(context.Background(), db, user.ID, UserNotificationStatusUnread, 10)
	if err != nil {
		t.Fatalf("ListUserNotifications() error = %v", err)
	}
	if len(notifications) != 1 || notifications[0].ID != created.ID {
		t.Fatalf("ListUserNotifications() = %#v", notifications)
	}

	read, err := MarkUserNotificationRead(context.Background(), db, created.ID, user.ID)
	if err != nil {
		t.Fatalf("MarkUserNotificationRead() error = %v", err)
	}
	if read.Status != UserNotificationStatusRead || read.ReadAt == "" {
		t.Fatalf("MarkUserNotificationRead() = %#v", read)
	}

	readNotifications, err := ListUserNotifications(context.Background(), db, user.ID, UserNotificationStatusRead, 10)
	if err != nil {
		t.Fatalf("ListUserNotifications(read) error = %v", err)
	}
	if len(readNotifications) != 1 || readNotifications[0].ID != created.ID {
		t.Fatalf("ListUserNotifications(read) = %#v", readNotifications)
	}
}
