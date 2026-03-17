package libtickettest

import (
	"strings"
	"testing"

	"github.com/simonski/ticket/libticket"
)

type Factory func(t *testing.T) libticket.Service

type ContractOptions struct {
	RequireStatusOwnership bool
}

func RunServiceContractTests(t *testing.T, factory Factory, opts ContractOptions) {
	t.Helper()

	t.Run("project-task-request-clone-comment-dependency", func(t *testing.T) {
		svc := factory(t)

		projects, err := svc.ListProjects()
		if err != nil {
			t.Fatalf("ListProjects() error = %v", err)
		}
		if len(projects) == 0 {
			t.Fatal("ListProjects() returned no projects")
		}

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{
			Title:              "Contract Project",
			Description:        "Description",
			AcceptanceCriteria: "AC",
		})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}
		if project.ID == 0 {
			t.Fatalf("CreateProject() = %#v", project)
		}

		ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Contract Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}
		if ticket.ID == 0 {
			t.Fatalf("CreateTicket() = %#v", ticket)
		}

		response, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: project.ID})
		if err != nil {
			t.Fatalf("RequestTicket() error = %v", err)
		}
		if response.Status != "ASSIGNED" || response.Ticket == nil {
			t.Fatalf("RequestTicket() = %#v", response)
		}

		updated, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    response.Ticket.Assignee,
			State:       "active",
		})
		if err != nil {
			t.Fatalf("UpdateTicket() error = %v", err)
		}
		if updated.Status != "design/active" {
			t.Fatalf("UpdateTicket().Status = %q, want design/active", updated.Status)
		}

		comment, err := svc.AddComment(ticket.ID, "contract comment")
		if err != nil {
			t.Fatalf("AddComment() error = %v", err)
		}
		if comment.Text != "contract comment" || strings.TrimSpace(comment.Author) == "" {
			t.Fatalf("AddComment() = %#v", comment)
		}

		comments, err := svc.ListComments(ticket.ID)
		if err != nil {
			t.Fatalf("ListComments() error = %v", err)
		}
		if len(comments) != 1 {
			t.Fatalf("ListComments() len = %d, want 1", len(comments))
		}
		if comments[0].Text != "contract comment" || strings.TrimSpace(comments[0].Author) == "" {
			t.Fatalf("ListComments() = %#v", comments)
		}

		other, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Dependency Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket(other) error = %v", err)
		}

		dependency, err := svc.AddDependency(libticket.DependencyRequest{
			ProjectID: project.ID,
			TicketID:  ticket.ID,
			DependsOn: other.ID,
		})
		if err != nil {
			t.Fatalf("AddDependency() error = %v", err)
		}
		if dependency.ID == 0 {
			t.Fatalf("AddDependency() = %#v", dependency)
		}

		dependencies, err := svc.ListDependencies(ticket.ID)
		if err != nil {
			t.Fatalf("ListDependencies() error = %v", err)
		}
		if len(dependencies) != 1 {
			t.Fatalf("ListDependencies() len = %d, want 1", len(dependencies))
		}

		if err := svc.RemoveDependency(libticket.DependencyRequest{
			ProjectID: project.ID,
			TicketID:  ticket.ID,
			DependsOn: other.ID,
		}); err != nil {
			t.Fatalf("RemoveDependency() error = %v", err)
		}

		cloned, err := svc.CloneTicket(ticket.ID)
		if err != nil {
			t.Fatalf("CloneTicket() error = %v", err)
		}
		if cloned.CloneOf == nil || *cloned.CloneOf != ticket.ID {
			t.Fatalf("CloneTicket() = %#v", cloned)
		}
		if cloned.Status != "design/idle" {
			t.Fatalf("CloneTicket().Status = %q, want design/idle", cloned.Status)
		}

		status, err := svc.Status()
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Status != "ok" {
			t.Fatalf("Status() = %#v", status)
		}
	})

	t.Run("project-update-and-enable-disable", func(t *testing.T) {
		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{
			Title:              "Project A",
			Description:        "Before",
			AcceptanceCriteria: "AC1",
		})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}

		updated, err := svc.UpdateProject(project.ID, libticket.ProjectUpdateRequest{
			Title:              "Project B",
			Description:        "After",
			AcceptanceCriteria: "AC2",
		})
		if err != nil {
			t.Fatalf("UpdateProject() error = %v", err)
		}
		if updated.Title != "Project B" || updated.Description != "After" || updated.AcceptanceCriteria != "AC2" {
			t.Fatalf("UpdateProject() = %#v", updated)
		}

		disabled, err := svc.SetProjectEnabled(project.ID, false)
		if err != nil {
			t.Fatalf("SetProjectEnabled(false) error = %v", err)
		}
		if disabled.Status != "closed" {
			t.Fatalf("SetProjectEnabled(false).Status = %q, want closed", disabled.Status)
		}

		enabled, err := svc.SetProjectEnabled(project.ID, true)
		if err != nil {
			t.Fatalf("SetProjectEnabled(true) error = %v", err)
		}
		if enabled.Status != "open" {
			t.Fatalf("SetProjectEnabled(true).Status = %q, want open", enabled.Status)
		}
	})

	t.Run("task-filter-history-and-closed-ticket-rules", func(t *testing.T) {
		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Tasks"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}

		ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID:   project.ID,
			Type:        "bug",
			Title:       "Bug task",
			Description: "find me",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		// Advance from design/idle to develop/idle so ticket is claimable
		if _, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			State:       "success",
		}); err != nil {
			t.Fatalf("UpdateTicket(advance to develop) error = %v", err)
		}

		requested, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: project.ID, TicketID: &ticket.ID})
		if err != nil {
			t.Fatalf("RequestTicket() error = %v", err)
		}
		if requested.Status != "ASSIGNED" || requested.Ticket == nil {
			t.Fatalf("RequestTicket() = %#v", requested)
		}

		filtered, err := svc.ListTicketsFiltered(project.ID, "bug", "", "", "develop/active", "find", requested.Ticket.Assignee, 10, false)
		if err != nil {
			t.Fatalf("ListTicketsFiltered() error = %v", err)
		}
		if len(filtered) != 1 || filtered[0].ID != ticket.ID {
			t.Fatalf("ListTicketsFiltered() = %#v", filtered)
		}

		// Advance through remaining stages to reach done/success
		// develop/active -> state=success -> test/idle
		advancedToTest, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    requested.Ticket.Assignee,
			State:       "success",
		})
		if err != nil {
			t.Fatalf("UpdateTicket(develop->test) error = %v", err)
		}
		if advancedToTest.Status != "test/idle" {
			t.Fatalf("UpdateTicket(develop->test).Status = %q, want test/idle", advancedToTest.Status)
		}

		// test -> state=success -> done/idle (auto-advance to final stage)
		advancedToDone, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    requested.Ticket.Assignee,
			State:       "success",
		})
		if err != nil {
			t.Fatalf("UpdateTicket(test->done) error = %v", err)
		}
		if advancedToDone.Status != "done/idle" {
			t.Fatalf("UpdateTicket(test->done).Status = %q, want done/idle", advancedToDone.Status)
		}

		// done -> state=success -> done/success (final stage, no further advance)
		completed, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    requested.Ticket.Assignee,
			State:       "success",
		})
		if err != nil {
			t.Fatalf("UpdateTicket(done->success) error = %v", err)
		}
		if completed.Status != "done/success" {
			t.Fatalf("UpdateTicket(done->success).Status = %q, want done/success", completed.Status)
		}

		history, err := svc.ListHistory(ticket.ID)
		if err != nil {
			t.Fatalf("ListHistory() error = %v", err)
		}
		if len(history) == 0 {
			t.Fatal("ListHistory() returned no history")
		}

		if _, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    requested.Ticket.Assignee,
			State:       "idle",
		}); err == nil {
			t.Fatal("UpdateTicket(reopen) error = nil, want closed ticket error")
		}
	})

	t.Run("negative-paths", func(t *testing.T) {
		svc := factory(t)

		if _, err := svc.GetProject("999999"); err == nil {
			t.Fatal("GetProject(missing) error = nil")
		}
		if _, err := svc.GetTicket("999999"); err == nil {
			t.Fatal("GetTicket(missing) error = nil")
		}

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Negative"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}
		ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Negative Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		if _, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    ticket.Assignee,
			State:       "bogus",
		}); err == nil {
			t.Fatal("UpdateTicket(invalid state) error = nil")
		}

		if err := svc.RemoveDependency(libticket.DependencyRequest{
			ProjectID: project.ID,
			TicketID:  ticket.ID,
			DependsOn: 424242,
		}); err == nil {
			t.Fatal("RemoveDependency(missing) error = nil")
		}

		if _, err := svc.CreateUser("someone-else", "secret"); err != nil {
			t.Fatalf("CreateUser(someone-else) error = %v", err)
		}

		assignedElsewhere, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Assigned Elsewhere",
			Assignee:  "someone-else",
		})
		if err != nil {
			t.Fatalf("CreateTicket(assigned) error = %v", err)
		}
		rejected, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: project.ID, TicketID: &assignedElsewhere.ID})
		if err != nil {
			t.Fatalf("RequestTicket(rejected) error = %v", err)
		}
		if rejected.Status != "REJECTED" {
			t.Fatalf("RequestTicket(rejected) = %#v", rejected)
		}
	})

	t.Run("status-change-requires-assignee", func(t *testing.T) {
		if !opts.RequireStatusOwnership {
			t.Skip("service does not enforce status ownership")
		}

		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Assign Rules"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}
		if _, err := svc.CreateUser("bob", "secret"); err != nil {
			t.Fatalf("CreateUser(bob) error = %v", err)
		}
		ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Assigned to bob",
			Assignee:  "bob",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}
		if _, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    ticket.Assignee,
			State:       "active",
		}); err == nil {
			t.Fatal("UpdateTicket(lifecycle by non-assignee) error = nil")
		}
	})

	t.Run("labels", func(t *testing.T) {
		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Labels"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}

		label, err := svc.CreateLabel(project.ID, libticket.LabelRequest{Name: "bug", Color: "red"})
		if err != nil {
			t.Fatalf("CreateLabel() error = %v", err)
		}
		if label.Name != "bug" || label.Color != "red" {
			t.Fatalf("CreateLabel() = %#v", label)
		}

		label2, err := svc.CreateLabel(project.ID, libticket.LabelRequest{Name: "feature", Color: "blue"})
		if err != nil {
			t.Fatalf("CreateLabel(feature) error = %v", err)
		}

		labels, err := svc.ListLabels(project.ID)
		if err != nil {
			t.Fatalf("ListLabels() error = %v", err)
		}
		if len(labels) != 2 {
			t.Fatalf("ListLabels() len = %d, want 2", len(labels))
		}

		ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Labeled Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		if err := svc.AddTicketLabel(ticket.ID, label.ID); err != nil {
			t.Fatalf("AddTicketLabel() error = %v", err)
		}
		if err := svc.AddTicketLabel(ticket.ID, label2.ID); err != nil {
			t.Fatalf("AddTicketLabel(feature) error = %v", err)
		}

		ticketLabels, err := svc.ListTicketLabels(ticket.ID)
		if err != nil {
			t.Fatalf("ListTicketLabels() error = %v", err)
		}
		if len(ticketLabels) != 2 {
			t.Fatalf("ListTicketLabels() len = %d, want 2", len(ticketLabels))
		}

		if err := svc.RemoveTicketLabel(ticket.ID, label.ID); err != nil {
			t.Fatalf("RemoveTicketLabel() error = %v", err)
		}
		ticketLabels, err = svc.ListTicketLabels(ticket.ID)
		if err != nil {
			t.Fatalf("ListTicketLabels() after remove error = %v", err)
		}
		if len(ticketLabels) != 1 {
			t.Fatalf("ListTicketLabels() after remove len = %d, want 1", len(ticketLabels))
		}

		if err := svc.DeleteLabel(label2.ID); err != nil {
			t.Fatalf("DeleteLabel() error = %v", err)
		}
		labels, err = svc.ListLabels(project.ID)
		if err != nil {
			t.Fatalf("ListLabels() after delete error = %v", err)
		}
		if len(labels) != 1 {
			t.Fatalf("ListLabels() after delete len = %d, want 1", len(labels))
		}
	})

	t.Run("time-tracking", func(t *testing.T) {
		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Time"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}

		ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Timed Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		entry1, err := svc.LogTime(ticket.ID, libticket.TimeEntryRequest{Minutes: 30, Note: "morning"})
		if err != nil {
			t.Fatalf("LogTime() error = %v", err)
		}
		if entry1.Minutes != 30 || entry1.Note != "morning" {
			t.Fatalf("LogTime() = %#v", entry1)
		}

		entry2, err := svc.LogTime(ticket.ID, libticket.TimeEntryRequest{Minutes: 45, Note: "afternoon"})
		if err != nil {
			t.Fatalf("LogTime(2) error = %v", err)
		}

		entries, err := svc.ListTimeEntries(ticket.ID)
		if err != nil {
			t.Fatalf("ListTimeEntries() error = %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("ListTimeEntries() len = %d, want 2", len(entries))
		}

		total, err := svc.TotalTimeForTicket(ticket.ID)
		if err != nil {
			t.Fatalf("TotalTimeForTicket() error = %v", err)
		}
		if total != 75 {
			t.Fatalf("TotalTimeForTicket() = %d, want 75", total)
		}

		if err := svc.DeleteTimeEntry(entry2.ID); err != nil {
			t.Fatalf("DeleteTimeEntry() error = %v", err)
		}

		total, err = svc.TotalTimeForTicket(ticket.ID)
		if err != nil {
			t.Fatalf("TotalTimeForTicket() after delete error = %v", err)
		}
		if total != 30 {
			t.Fatalf("TotalTimeForTicket() after delete = %d, want 30", total)
		}
	})

	t.Run("user-management-and-request-no-work", func(t *testing.T) {
		svc := factory(t)

		user, err := svc.CreateUser("alice", "secret")
		if err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}
		if user.Username != "alice" {
			t.Fatalf("CreateUser() = %#v", user)
		}

		users, err := svc.ListUsers()
		if err != nil {
			t.Fatalf("ListUsers() error = %v", err)
		}
		var found bool
		for _, entry := range users {
			if entry.Username == "alice" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("ListUsers() missing alice: %#v", users)
		}

		if err := svc.SetUserEnabled("alice", false); err != nil {
			t.Fatalf("SetUserEnabled(false) error = %v", err)
		}
		if err := svc.SetUserEnabled("alice", true); err != nil {
			t.Fatalf("SetUserEnabled(true) error = %v", err)
		}
		if err := svc.DeleteUser("alice"); err != nil {
			t.Fatalf("DeleteUser() error = %v", err)
		}

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Empty"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}
		response, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: project.ID})
		if err != nil {
			t.Fatalf("RequestTicket(no work) error = %v", err)
		}
		if response.Status != "NO-WORK" || response.Ticket != nil {
			t.Fatalf("RequestTicket(no work) = %#v", response)
		}
	})
}
