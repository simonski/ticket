package server

import "testing"

func TestBuildLiveChangeEvent(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		projectID  int64
		ticketID   int64
		wantChange string
		wantEntity string
		wantID     int64
	}{
		{
			name:       "ticket created",
			eventType:  "ticket_created",
			projectID:  12,
			ticketID:   34,
			wantChange: "created",
			wantEntity: "ticket",
			wantID:     34,
		},
		{
			name:       "ticket updated",
			eventType:  "ticket_updated",
			projectID:  12,
			ticketID:   34,
			wantChange: "updated",
			wantEntity: "ticket",
			wantID:     34,
		},
		{
			name:       "ticket deleted",
			eventType:  "ticket_deleted",
			projectID:  12,
			ticketID:   34,
			wantChange: "deleted",
			wantEntity: "ticket",
			wantID:     34,
		},
		{
			name:       "project created",
			eventType:  "project_created",
			projectID:  12,
			ticketID:   34,
			wantChange: "created",
			wantEntity: "project",
			wantID:     12,
		},
		{
			name:       "project updated",
			eventType:  "project_updated",
			projectID:  12,
			ticketID:   34,
			wantChange: "updated",
			wantEntity: "project",
			wantID:     12,
		},
		{
			name:       "project users updated",
			eventType:  "project_users_updated",
			projectID:  12,
			ticketID:   34,
			wantChange: "users_updated",
			wantEntity: "project",
			wantID:     12,
		},
		{
			name:       "unknown",
			eventType:  "agent_ping",
			projectID:  12,
			ticketID:   34,
			wantChange: "",
			wantEntity: "",
			wantID:     0,
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
				t.Fatalf("TicketID = %d, want %d", got.TicketID, tc.ticketID)
			}
			if got.ChangeType != tc.wantChange {
				t.Fatalf("ChangeType = %q, want %q", got.ChangeType, tc.wantChange)
			}
			if got.EntityType != tc.wantEntity {
				t.Fatalf("EntityType = %q, want %q", got.EntityType, tc.wantEntity)
			}
			if got.EntityID != tc.wantID {
				t.Fatalf("EntityID = %d, want %d", got.EntityID, tc.wantID)
			}
		})
	}
}
