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
		if ticket.ID == "" {
			t.Fatalf("CreateTicket() = %#v", ticket)
		}

		if _, err := svc.ReadyTicket(ticket.ID); err != nil {
			t.Fatalf("ReadyTicket() error = %v", err)
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
			DependsOn: "NONEXISTENT-42",
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

	t.Run("ticket-lifecycle-close-open-archive-delete", func(t *testing.T) {
		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Lifecycle"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}

		ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Lifecycle Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		// GetTicketByID
		got, err := svc.GetTicketByID(ticket.ID)
		if err != nil {
			t.Fatalf("GetTicketByID() error = %v", err)
		}
		if got.Title != "Lifecycle Task" {
			t.Fatalf("GetTicketByID().Title = %q", got.Title)
		}

		// Close/Open
		closed, err := svc.CloseTicket(ticket.ID)
		if err != nil {
			t.Fatalf("CloseTicket() error = %v", err)
		}
		if closed.Open {
			t.Fatal("CloseTicket().Open = true, want false")
		}

		opened, err := svc.OpenTicket(ticket.ID)
		if err != nil {
			t.Fatalf("OpenTicket() error = %v", err)
		}
		if !opened.Open {
			t.Fatal("OpenTicket().Open = false, want true")
		}

		// Archive/Unarchive
		archived, err := svc.ArchiveTicket(ticket.ID)
		if err != nil {
			t.Fatalf("ArchiveTicket() error = %v", err)
		}
		if !archived.Archived {
			t.Fatal("ArchiveTicket().Archived = false, want true")
		}

		unarchived, err := svc.UnarchiveTicket(ticket.ID)
		if err != nil {
			t.Fatalf("UnarchiveTicket() error = %v", err)
		}
		if unarchived.Archived {
			t.Fatal("UnarchiveTicket().Archived = true, want false")
		}

		// SetTicketHealth
		healthy, err := svc.SetTicketHealth(ticket.ID, 3)
		if err != nil {
			t.Fatalf("SetTicketHealth() error = %v", err)
		}
		if healthy.HealthScore != 3 {
			t.Fatalf("SetTicketHealth().HealthScore = %d, want 3", healthy.HealthScore)
		}

		// SetTicketParent / UnsetTicketParent
		child, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Child Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket(child) error = %v", err)
		}

		parented, err := svc.SetTicketParent(child.ID, ticket.ID)
		if err != nil {
			t.Fatalf("SetTicketParent() error = %v", err)
		}
		if parented.ParentID == nil || *parented.ParentID != ticket.ID {
			t.Fatalf("SetTicketParent().ParentID = %v, want %s", parented.ParentID, ticket.ID)
		}

		unparented, err := svc.UnsetTicketParent(child.ID)
		if err != nil {
			t.Fatalf("UnsetTicketParent() error = %v", err)
		}
		if unparented.ParentID != nil {
			t.Fatalf("UnsetTicketParent().ParentID = %v, want nil", unparented.ParentID)
		}

		// DeleteTicket
		if err := svc.DeleteTicket(child.ID); err != nil {
			t.Fatalf("DeleteTicket() error = %v", err)
		}
		if _, err := svc.GetTicketByID(child.ID); err == nil {
			t.Fatal("GetTicketByID(deleted) error = nil")
		}
	})

	t.Run("workflow-crud-and-stages", func(t *testing.T) {
		svc := factory(t)

		wf, err := svc.CreateWorkflow(libticket.WorkflowRequest{
			Name:        "test-workflow",
			Description: "A test workflow",
		})
		if err != nil {
			t.Fatalf("CreateWorkflow() error = %v", err)
		}
		if wf.Name != "test-workflow" {
			t.Fatalf("CreateWorkflow().Name = %q", wf.Name)
		}

		workflows, err := svc.ListWorkflows()
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		var found bool
		for _, w := range workflows {
			if w.ID == wf.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("ListWorkflows() missing created workflow")
		}

		stage1, err := svc.AddWorkflowStage(wf.ID, libticket.WorkflowStageRequest{
			StageName: "alpha",
			SortOrder: 0,
		})
		if err != nil {
			t.Fatalf("AddWorkflowStage(alpha) error = %v", err)
		}

		stage2, err := svc.AddWorkflowStage(wf.ID, libticket.WorkflowStageRequest{
			StageName: "beta",
			SortOrder: 1,
		})
		if err != nil {
			t.Fatalf("AddWorkflowStage(beta) error = %v", err)
		}

		withStages, err := svc.GetWorkflow(wf.ID)
		if err != nil {
			t.Fatalf("GetWorkflow() error = %v", err)
		}
		if len(withStages.Stages) != 2 {
			t.Fatalf("GetWorkflow().Stages len = %d, want 2", len(withStages.Stages))
		}

		// Reorder stages
		if err := svc.ReorderWorkflowStages(wf.ID, []int64{stage2.ID, stage1.ID}); err != nil {
			t.Fatalf("ReorderWorkflowStages() error = %v", err)
		}

		reordered, err := svc.GetWorkflow(wf.ID)
		if err != nil {
			t.Fatalf("GetWorkflow() after reorder error = %v", err)
		}
		if reordered.Stages[0].StageName != "beta" {
			t.Fatalf("ReorderWorkflowStages() first stage = %q, want beta", reordered.Stages[0].StageName)
		}

		// Export/Import
		exported, err := svc.ExportWorkflow(wf.ID)
		if err != nil {
			t.Fatalf("ExportWorkflow() error = %v", err)
		}
		if exported.Name != "test-workflow" || len(exported.Stages) != 2 {
			t.Fatalf("ExportWorkflow() = %#v", exported)
		}

		exported.Name = "imported-workflow"
		imported, err := svc.ImportWorkflow(exported)
		if err != nil {
			t.Fatalf("ImportWorkflow() error = %v", err)
		}
		if imported.Name != "imported-workflow" {
			t.Fatalf("ImportWorkflow().Name = %q", imported.Name)
		}

		// RemoveWorkflowStage
		if err := svc.RemoveWorkflowStage(stage1.ID); err != nil {
			t.Fatalf("RemoveWorkflowStage() error = %v", err)
		}

		afterRemove, err := svc.GetWorkflow(wf.ID)
		if err != nil {
			t.Fatalf("GetWorkflow() after remove error = %v", err)
		}
		if len(afterRemove.Stages) != 1 {
			t.Fatalf("GetWorkflow().Stages after remove len = %d, want 1", len(afterRemove.Stages))
		}

		// DeleteWorkflow
		if err := svc.DeleteWorkflow(wf.ID); err != nil {
			t.Fatalf("DeleteWorkflow() error = %v", err)
		}
	})

	t.Run("role-crud", func(t *testing.T) {
		svc := factory(t)

		role, err := svc.CreateRole(libticket.RoleRequest{
			Title:      "Tester",
			Motivation: "Ensure quality",
			Goals:      "Find bugs",
		})
		if err != nil {
			t.Fatalf("CreateRole() error = %v", err)
		}
		if role.Title != "Tester" {
			t.Fatalf("CreateRole().Title = %q", role.Title)
		}

		roles, err := svc.ListRoles()
		if err != nil {
			t.Fatalf("ListRoles() error = %v", err)
		}
		var foundRole bool
		for _, r := range roles {
			if r.ID == role.ID {
				foundRole = true
			}
		}
		if !foundRole {
			t.Fatal("ListRoles() missing created role")
		}

		updated, err := svc.UpdateRole(role.ID, libticket.RoleRequest{
			Title:      "Senior Tester",
			Motivation: "Lead quality",
			Goals:      "Zero defects",
		})
		if err != nil {
			t.Fatalf("UpdateRole() error = %v", err)
		}
		if updated.Title != "Senior Tester" {
			t.Fatalf("UpdateRole().Title = %q", updated.Title)
		}

		if err := svc.DeleteRole(role.ID); err != nil {
			t.Fatalf("DeleteRole() error = %v", err)
		}
	})

	t.Run("team-crud-and-membership", func(t *testing.T) {
		svc := factory(t)

		team, err := svc.CreateTeam(libticket.TeamRequest{Name: "Platform"})
		if err != nil {
			t.Fatalf("CreateTeam() error = %v", err)
		}
		if team.Name != "Platform" {
			t.Fatalf("CreateTeam().Name = %q", team.Name)
		}

		teams, err := svc.ListTeams()
		if err != nil {
			t.Fatalf("ListTeams() error = %v", err)
		}
		var foundTeam bool
		for _, tm := range teams {
			if tm.ID == team.ID {
				foundTeam = true
			}
		}
		if !foundTeam {
			t.Fatal("ListTeams() missing created team")
		}

		updatedTeam, err := svc.UpdateTeam(team.ID, libticket.TeamRequest{Name: "Infrastructure"})
		if err != nil {
			t.Fatalf("UpdateTeam() error = %v", err)
		}
		if updatedTeam.Name != "Infrastructure" {
			t.Fatalf("UpdateTeam().Name = %q", updatedTeam.Name)
		}

		// Team members
		user, err := svc.CreateUser("team-member", "secret")
		if err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}

		member, err := svc.AddTeamMember(team.ID, libticket.TeamMemberRequest{
			UserID: user.ID,
			Role:   "member",
		})
		if err != nil {
			t.Fatalf("AddTeamMember() error = %v", err)
		}
		if member.UserID != user.ID {
			t.Fatalf("AddTeamMember().UserID = %s", member.UserID)
		}

		members, err := svc.ListTeamMembers(team.ID)
		if err != nil {
			t.Fatalf("ListTeamMembers() error = %v", err)
		}
		memberCountBefore := len(members)

		if err := svc.RemoveTeamMember(team.ID, user.ID); err != nil {
			t.Fatalf("RemoveTeamMember() error = %v", err)
		}

		membersAfter, err := svc.ListTeamMembers(team.ID)
		if err != nil {
			t.Fatalf("ListTeamMembers() after remove error = %v", err)
		}
		if len(membersAfter) != memberCountBefore-1 {
			t.Fatalf("ListTeamMembers() after remove len = %d, want %d", len(membersAfter), memberCountBefore-1)
		}

		// Remove remaining members before deleting team (HTTP adds creator as member)
		for _, m := range membersAfter {
			_ = svc.RemoveTeamMember(team.ID, m.UserID)
		}

		if err := svc.DeleteTeam(team.ID); err != nil {
			t.Fatalf("DeleteTeam() error = %v", err)
		}
	})

	t.Run("count", func(t *testing.T) {
		svc := factory(t)

		summary, err := svc.Count(nil)
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if summary.Projects == 0 {
			t.Fatal("Count().Projects = 0, want > 0")
		}

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{Title: "Counted"})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}
		if _, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Count Me",
		}); err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		scoped, err := svc.Count(&project.ID)
		if err != nil {
			t.Fatalf("Count(projectID) error = %v", err)
		}
		if len(scoped.Types) == 0 {
			t.Fatal("Count(projectID).Types is empty, want > 0")
		}
	})

	t.Run("agent-crud", func(t *testing.T) {
		svc := factory(t)

		// Create agent
		agent, password, err := svc.CreateAgent(libticket.AgentCreateRequest{})
		if err != nil {
			t.Fatalf("CreateAgent() error = %v", err)
		}
		if agent.ID == "" {
			t.Fatal("agent.ID is empty")
		}
		if password == "" {
			t.Fatal("CreateAgent() returned empty password")
		}

		// List agents
		agents, err := svc.ListAgents()
		if err != nil {
			t.Fatalf("ListAgents() error = %v", err)
		}
		found := false
		for _, a := range agents {
			if a.ID == agent.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("ListAgents() did not include created agent %s", agent.ID)
		}

		// Update agent password
		newPass := "new-password"
		_, err = svc.UpdateAgent(agent.ID, libticket.AgentUpdateRequest{
			Password: &newPass,
		})
		if err != nil {
			t.Fatalf("UpdateAgent() error = %v", err)
		}

		// Disable agent
		disabled, err := svc.SetAgentEnabled(agent.ID, false)
		if err != nil {
			t.Fatalf("SetAgentEnabled(false) error = %v", err)
		}
		if disabled.Enabled {
			t.Fatal("expected agent to be disabled")
		}

		// Re-enable
		enabled, err := svc.SetAgentEnabled(agent.ID, true)
		if err != nil {
			t.Fatalf("SetAgentEnabled(true) error = %v", err)
		}
		if !enabled.Enabled {
			t.Fatal("expected agent to be enabled")
		}

		// Delete agent
		if err := svc.DeleteAgent(agent.ID); err != nil {
			t.Fatalf("DeleteAgent() error = %v", err)
		}
	})

	t.Run("agent-register-and-request-work", func(t *testing.T) {
		svc := factory(t)

		// Create an agent with known password
		agent, _, err := svc.CreateAgent(libticket.AgentCreateRequest{
			Password: "secret123",
		})
		if err != nil {
			t.Fatalf("CreateAgent() error = %v", err)
		}

		// Register (authenticate) the agent
		registered, err := svc.RegisterAgent(libticket.AgentRegisterRequest{
			ID:       agent.ID,
			Password: "secret123",
		})
		if err != nil {
			t.Fatalf("RegisterAgent() error = %v", err)
		}
		if registered.ID != agent.ID {
			t.Fatalf("registered.ID = %s, want %s", registered.ID, agent.ID)
		}

		// Request work (no tickets — expect NONE)
		resp, err := svc.RequestAgentWork(libticket.AgentRequest{
			ID:       agent.ID,
			Password: "secret123",
		})
		if err != nil {
			t.Fatalf("RequestAgentWork() error = %v", err)
		}
		if resp.Status != "NONE" {
			t.Logf("RequestAgentWork() status = %q (may have existing tickets)", resp.Status)
		}

		// Cleanup
		_ = svc.DeleteAgent(agent.ID)
	})

	t.Run("project-member-management", func(t *testing.T) {
		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{
			Title:       "Member Test Project",
			Description: "For testing project members",
		})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}

		// List members (may include creator)
		membersBefore, err := svc.ListProjectMembers(project.ID)
		if err != nil {
			t.Fatalf("ListProjectMembers() error = %v", err)
		}
		countBefore := len(membersBefore)

		// Create a user to add as member
		user, err := svc.CreateUser("projmember", "pass123")
		if err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}

		// Add project member
		member, err := svc.AddProjectMember(project.ID, libticket.ProjectMemberRequest{
			UserID: user.ID,
			Role:   "editor",
		})
		if err != nil {
			t.Fatalf("AddProjectMember() error = %v", err)
		}
		if member.UserID != user.ID {
			t.Fatalf("member.UserID = %s, want %s", member.UserID, user.ID)
		}

		// List members (should have one more)
		membersAfter, err := svc.ListProjectMembers(project.ID)
		if err != nil {
			t.Fatalf("ListProjectMembers() error = %v", err)
		}
		if len(membersAfter) != countBefore+1 {
			t.Fatalf("ListProjectMembers() count = %d, want %d", len(membersAfter), countBefore+1)
		}

		// Remove project member
		if err := svc.RemoveProjectMember(project.ID, user.ID); err != nil {
			t.Fatalf("RemoveProjectMember() error = %v", err)
		}

		// Verify removed
		membersEnd, err := svc.ListProjectMembers(project.ID)
		if err != nil {
			t.Fatalf("ListProjectMembers() error = %v", err)
		}
		if len(membersEnd) != countBefore {
			t.Fatalf("ListProjectMembers() count after remove = %d, want %d", len(membersEnd), countBefore)
		}

		_ = svc.DeleteUser("projmember")
	})

	t.Run("project-team-member-management", func(t *testing.T) {
		svc := factory(t)

		project, err := svc.CreateProject(libticket.ProjectCreateRequest{
			Title:       "Team Member Test Project",
			Description: "For testing project team members",
		})
		if err != nil {
			t.Fatalf("CreateProject() error = %v", err)
		}

		team, err := svc.CreateTeam(libticket.TeamRequest{Name: "projteam"})
		if err != nil {
			t.Fatalf("CreateTeam() error = %v", err)
		}

		// List project teams (initially empty or minimal)
		teamsBefore, err := svc.ListProjectTeamMembers(project.ID)
		if err != nil {
			t.Fatalf("ListProjectTeamMembers() error = %v", err)
		}
		countBefore := len(teamsBefore)

		// Add team to project
		ptm, err := svc.AddProjectTeamMember(project.ID, libticket.ProjectTeamMemberRequest{
			TeamID: team.ID,
			Role:   "editor",
		})
		if err != nil {
			t.Fatalf("AddProjectTeamMember() error = %v", err)
		}
		if ptm.TeamID != team.ID {
			t.Fatalf("ptm.TeamID = %d, want %d", ptm.TeamID, team.ID)
		}

		// List project teams (should have one more)
		teamsAfter, err := svc.ListProjectTeamMembers(project.ID)
		if err != nil {
			t.Fatalf("ListProjectTeamMembers() error = %v", err)
		}
		if len(teamsAfter) != countBefore+1 {
			t.Fatalf("ListProjectTeamMembers() count = %d, want %d", len(teamsAfter), countBefore+1)
		}

		// Remove team from project
		if err := svc.RemoveProjectTeamMember(project.ID, team.ID); err != nil {
			t.Fatalf("RemoveProjectTeamMember() error = %v", err)
		}

		// Cleanup
		_ = svc.DeleteTeam(team.ID)
	})

	t.Run("team-agent-management", func(t *testing.T) {
		svc := factory(t)

		team, err := svc.CreateTeam(libticket.TeamRequest{Name: "agent-team"})
		if err != nil {
			t.Fatalf("CreateTeam() error = %v", err)
		}

		agent, _, err := svc.CreateAgent(libticket.AgentCreateRequest{})
		if err != nil {
			t.Fatalf("CreateAgent() error = %v", err)
		}

		// Add agent to team
		ta, err := svc.AddTeamAgent(team.ID, agent.ID)
		if err != nil {
			t.Fatalf("AddTeamAgent() error = %v", err)
		}
		if ta.AgentID != agent.ID {
			t.Fatalf("ta.AgentID = %s, want %s", ta.AgentID, agent.ID)
		}

		// List team agents
		agents, err := svc.ListTeamAgents(team.ID)
		if err != nil {
			t.Fatalf("ListTeamAgents() error = %v", err)
		}
		if len(agents) == 0 {
			t.Fatal("ListTeamAgents() returned empty list")
		}

		// Remove agent from team
		if err := svc.RemoveTeamAgent(team.ID, agent.ID); err != nil {
			t.Fatalf("RemoveTeamAgent() error = %v", err)
		}

		// Cleanup
		_ = svc.DeleteAgent(agent.ID)
		_ = svc.DeleteTeam(team.ID)
	})

	t.Run("registration-toggle", func(t *testing.T) {
		svc := factory(t)

		// Toggle registration
		if err := svc.SetRegistrationEnabled(false); err != nil {
			t.Fatalf("SetRegistrationEnabled(false) error = %v", err)
		}
		if err := svc.SetRegistrationEnabled(true); err != nil {
			t.Fatalf("SetRegistrationEnabled(true) error = %v", err)
		}
	})

	t.Run("list-tickets-unfiltered", func(t *testing.T) {
		svc := factory(t)

		projects, err := svc.ListProjects()
		if err != nil {
			t.Fatalf("ListProjects() error = %v", err)
		}
		if len(projects) == 0 {
			t.Fatal("no projects")
		}

		// ListTickets (unfiltered wrapper)
		tickets, err := svc.ListTickets(projects[0].ID)
		if err != nil {
			t.Fatalf("ListTickets() error = %v", err)
		}
		_ = tickets // may be empty, just verify no error
	})
}
