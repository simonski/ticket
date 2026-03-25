package server

import (
	"fmt"
	"testing"
)

func TestBuildLiveChangeEvent(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		projectID  int64
		ticketID   string
		wantChange string
		wantEntity string
		wantID     any
	}{
		{
			name:       "ticket created",
			eventType:  "ticket_created",
			projectID:  12,
			ticketID:   "TK-34",
			wantChange: "created",
			wantEntity: "ticket",
			wantID:     "TK-34",
		},
		{
			name:       "ticket updated",
			eventType:  "ticket_updated",
			projectID:  12,
			ticketID:   "TK-34",
			wantChange: "updated",
			wantEntity: "ticket",
			wantID:     "TK-34",
		},
		{
			name:       "ticket deleted",
			eventType:  "ticket_deleted",
			projectID:  12,
			ticketID:   "TK-34",
			wantChange: "deleted",
			wantEntity: "ticket",
			wantID:     "TK-34",
		},
		{
			name:       "project created",
			eventType:  "project_created",
			projectID:  12,
			ticketID:   "TK-34",
			wantChange: "created",
			wantEntity: "project",
			wantID:     int64(12),
		},
		{
			name:       "project updated",
			eventType:  "project_updated",
			projectID:  12,
			ticketID:   "TK-34",
			wantChange: "updated",
			wantEntity: "project",
			wantID:     int64(12),
		},
		{
			name:       "project users updated",
			eventType:  "project_users_updated",
			projectID:  12,
			ticketID:   "TK-34",
			wantChange: "users_updated",
			wantEntity: "project",
			wantID:     int64(12),
		},
		{
			name:       "unknown",
			eventType:  "agent_ping",
			projectID:  12,
			ticketID:   "TK-34",
			wantChange: "",
			wantEntity: "",
			wantID:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildLiveChangeEvent(tc.eventType, tc.projectID, tc.ticketID)
			if got.Type != tc.eventType {
				t.Fatalf("Type = %q, want %q", got.Type, tc.eventType)
			}
			if got.ProjectID != tc.projectID {
				t.Fatalf("ProjectID = %d, want %d", got.ProjectID, tc.projectID)
			}
			if got.TicketID != tc.ticketID {
				t.Fatalf("TicketID = %q, want %q", got.TicketID, tc.ticketID)
			}
			if got.ChangeType != tc.wantChange {
				t.Fatalf("ChangeType = %q, want %q", got.ChangeType, tc.wantChange)
			}
			if got.EntityType != tc.wantEntity {
				t.Fatalf("EntityType = %q, want %q", got.EntityType, tc.wantEntity)
			}
			if fmt.Sprintf("%v", got.EntityID) != fmt.Sprintf("%v", tc.wantID) {
				t.Fatalf("EntityID = %v, want %v", got.EntityID, tc.wantID)
			}
		})
	}
}
