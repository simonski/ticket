package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func noEntitiesAvailable(name string) string {
	return fmt.Sprintf("No %s available.", name)
}

func printNoEntitiesAvailable(name string) {
	fmt.Println(noEntitiesAvailable(name))
}

func entityPlural(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "story":
		return "stories"
	default:
		if strings.HasSuffix(normalized, "y") {
			return normalized[:len(normalized)-1] + "ies"
		}
		return normalized + "s"
	}
}

func requireCurrentProject(cfg config.Config, svc libticket.Service) (store.Project, error) {
	project, _, err := resolveProjectContext(context.Background(), cfg, svc, resolveConfiguredProjectReference(cfg))
	if err != nil {
		return store.Project{}, err
	}
	return project, nil
}

func mostRecentProject(svc libticket.Service) (store.Project, error) {
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		return store.Project{}, err
	}
	if len(projects) == 0 {
		return store.Project{}, errors.New(noEntitiesAvailable("projects"))
	}
	latest := projects[0]
	for _, project := range projects[1:] {
		if projectMoreRecent(project, latest) {
			latest = project
		}
	}
	return latest, nil
}

func projectMoreRecent(a, b store.Project) bool {
	if strings.TrimSpace(a.UpdatedAt) != strings.TrimSpace(b.UpdatedAt) {
		return a.UpdatedAt > b.UpdatedAt
	}
	return a.ID > b.ID
}

func mostRecentTicket(svc libticket.Service, projectID int64, ticketType string) (store.Ticket, error) {
	tickets, err := svc.ListTicketsFiltered(context.Background(), projectID, ticketType, "", "", "", "", "", 0, true)
	if err != nil {
		return store.Ticket{}, err
	}
	if len(tickets) == 0 {
		name := "tickets"
		if strings.TrimSpace(ticketType) != "" {
			name = entityPlural(ticketType)
		}
		return store.Ticket{}, errors.New(noEntitiesAvailable(name))
	}
	latest := tickets[0]
	for _, ticket := range tickets[1:] {
		if ticketMoreRecent(ticket, latest) {
			latest = ticket
		}
	}
	return latest, nil
}

func ticketMoreRecent(a, b store.Ticket) bool {
	if strings.TrimSpace(a.UpdatedAt) != strings.TrimSpace(b.UpdatedAt) {
		return a.UpdatedAt > b.UpdatedAt
	}
	return ticketSequence(a.ID) > ticketSequence(b.ID)
}

func ticketSequence(id string) int64 {
	idx := strings.LastIndex(strings.TrimSpace(id), "-")
	if idx < 0 {
		return 0
	}
	n, _ := strconv.ParseInt(id[idx+1:], 10, 64)
	return n
}

func resolveTypedTicketRef(cfg config.Config, svc libticket.Service, ticketType, rawID string) (string, error) {
	id := normalizeBareTicketRef(cfg, svc, strings.TrimSpace(rawID))
	if id != "" {
		return id, nil
	}
	project, err := requireCurrentProject(cfg, svc)
	if err != nil {
		return "", err
	}
	ticket, err := mostRecentTicket(svc, project.ID, ticketType)
	if err != nil {
		return "", err
	}
	return ticket.ID, nil
}

func mostRecentStory(svc libticket.Service, projectID int64) (store.Story, error) {
	stories, err := svc.ListStories(context.Background(), projectID)
	if err != nil {
		return store.Story{}, err
	}
	if len(stories) == 0 {
		return store.Story{}, errors.New(noEntitiesAvailable("stories"))
	}
	latest := stories[0]
	for _, story := range stories[1:] {
		if storyMoreRecent(story, latest) {
			latest = story
		}
	}
	return latest, nil
}

func storyMoreRecent(a, b store.Story) bool {
	if strings.TrimSpace(a.UpdatedAt) != strings.TrimSpace(b.UpdatedAt) {
		return a.UpdatedAt > b.UpdatedAt
	}
	return a.ID > b.ID
}

func resolveStoryID(svc libticket.Service, projectID int64, rawID string) (int64, error) {
	idStr := strings.TrimSpace(rawID)
	if idStr == "" {
		story, err := mostRecentStory(svc, projectID)
		if err != nil {
			return 0, err
		}
		return story.ID, nil
	}
	var id int64
	if _, err := fmt.Sscan(idStr, &id); err != nil {
		return 0, fmt.Errorf("invalid story id %q", rawID)
	}
	return id, nil
}

func mostRecentLabel(labels []store.Label) (store.Label, error) {
	if len(labels) == 0 {
		return store.Label{}, errors.New(noEntitiesAvailable("labels"))
	}
	latest := labels[0]
	for _, label := range labels[1:] {
		if labelMoreRecent(label, latest) {
			latest = label
		}
	}
	return latest, nil
}

func labelMoreRecent(a, b store.Label) bool {
	if strings.TrimSpace(a.CreatedAt) != strings.TrimSpace(b.CreatedAt) {
		return a.CreatedAt > b.CreatedAt
	}
	return a.ID > b.ID
}

func findLabelByID(labels []store.Label, id int64) (store.Label, bool) {
	for _, label := range labels {
		if label.ID == id {
			return label, true
		}
	}
	return store.Label{}, false
}
