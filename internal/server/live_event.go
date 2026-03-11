package server

import "strings"

func buildLiveChangeEvent(eventType string, projectID, ticketID int64) liveEvent {
	eventType = strings.TrimSpace(eventType)
	changeType := ""
	entityType := ""
	entityID := int64(0)

	switch eventType {
	case "ticket_created":
		changeType = "created"
		entityType = "ticket"
		entityID = ticketID
	case "ticket_updated":
		changeType = "updated"
		entityType = "ticket"
		entityID = ticketID
	case "ticket_deleted":
		changeType = "deleted"
		entityType = "ticket"
		entityID = ticketID
	case "project_created":
		changeType = "created"
		entityType = "project"
		entityID = projectID
	case "project_updated":
		changeType = "updated"
		entityType = "project"
		entityID = projectID
	case "project_users_updated":
		changeType = "users_updated"
		entityType = "project"
		entityID = projectID
	}

	return liveEvent{
		Type:       eventType,
		ChangeType: changeType,
		EntityType: entityType,
		EntityID:   entityID,
		ProjectID:  projectID,
		TicketID:   ticketID,
	}
}
