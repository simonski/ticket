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

		task, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Contract Task",
			Stage:     "develop",
			State:     "idle",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}
		if task.ID == 0 {
			t.Fatalf("CreateTicket() = %#v", task)
		}

		response, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: project.ID})
		if err != nil {
			t.Fatalf("RequestTicket() error = %v", err)
		}
		if response.Status != "ASSIGNED" || response.Ticket == nil {
			t.Fatalf("RequestTicket() = %#v", response)
		}

		updated, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
			Title:       task.Title,
			Description: task.Description,
			ParentID:    task.ParentID,
			Assignee:    response.Ticket.Assignee,
			Stage:       "develop",
			State:       "active",
		})
		if err != nil {
			t.Fatalf("UpdateTicket() error = %v", err)
		}
		if updated.Status != "develop/active" {
			t.Fatalf("UpdateTicket().Status = %q, want develop/active", updated.Status)
		}

		comment, err := svc.AddComment(task.ID, "contract comment")
		if err != nil {
			t.Fatalf("AddComment() error = %v", err)
		}
		if comment.Text != "contract comment" || strings.TrimSpace(comment.Author) == "" {
			t.Fatalf("AddComment() = %#v", comment)
		}

		comments, err := svc.ListComments(task.ID)
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
			TicketID:  task.ID,
			DependsOn: other.ID,
		})
		if err != nil {
			t.Fatalf("AddDependency() error = %v", err)
		}
		if dependency.ID == 0 {
			t.Fatalf("AddDependency() = %#v", dependency)
		}

		dependencies, err := svc.ListDependencies(task.ID)
		if err != nil {
			t.Fatalf("ListDependencies() error = %v", err)
		}
		if len(dependencies) != 1 {
			t.Fatalf("ListDependencies() len = %d, want 1", len(dependencies))
		}

		if err := svc.RemoveDependency(libticket.DependencyRequest{
			ProjectID: project.ID,
			TicketID:  task.ID,
			DependsOn: other.ID,
		}); err != nil {
			t.Fatalf("RemoveDependency() error = %v", err)
		}

		cloned, err := svc.CloneTicket(task.ID)
		if err != nil {
			t.Fatalf("CloneTicket() error = %v", err)
		}
		if cloned.CloneOf == nil || *cloned.CloneOf != task.ID {
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

		task, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID:   project.ID,
			Type:        "bug",
			Title:       "Bug task",
			Description: "find me",
			Stage:       "develop",
			State:       "idle",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		requested, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: project.ID, TicketID: &task.ID})
		if err != nil {
			t.Fatalf("RequestTicket() error = %v", err)
		}
		if requested.Status != "ASSIGNED" || requested.Ticket == nil {
			t.Fatalf("RequestTicket() = %#v", requested)
		}

		filtered, err := svc.ListTicketsFiltered(project.ID, "bug", "", "", "develop/active", "find", requested.Ticket.Assignee, 10)
		if err != nil {
			t.Fatalf("ListTicketsFiltered() error = %v", err)
		}
		if len(filtered) != 1 || filtered[0].ID != task.ID {
			t.Fatalf("ListTicketsFiltered() = %#v", filtered)
		}

		completed, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
			Title:       task.Title,
			Description: task.Description,
			ParentID:    task.ParentID,
			Assignee:    requested.Ticket.Assignee,
			Stage:       "done",
			State:       "complete",
		})
		if err != nil {
			t.Fatalf("UpdateTicket(complete) error = %v", err)
		}
		if completed.Status != "done/complete" {
			t.Fatalf("UpdateTicket(complete).Status = %q", completed.Status)
		}

		history, err := svc.ListHistory(task.ID)
		if err != nil {
			t.Fatalf("ListHistory() error = %v", err)
		}
		if len(history) == 0 {
			t.Fatal("ListHistory() returned no history")
		}

		if _, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
			Title:       task.Title,
			Description: task.Description,
			ParentID:    task.ParentID,
			Assignee:    requested.Ticket.Assignee,
			Stage:       "develop",
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
		task, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Negative Task",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}

		if _, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
			Title:       task.Title,
			Description: task.Description,
			ParentID:    task.ParentID,
			Assignee:    task.Assignee,
			Stage:       "bogus",
			State:       "idle",
		}); err == nil {
			t.Fatal("UpdateTicket(invalid lifecycle) error = nil")
		}

		if err := svc.RemoveDependency(libticket.DependencyRequest{
			ProjectID: project.ID,
			TicketID:  task.ID,
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
			Stage:     "develop",
			State:     "idle",
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
		task, err := svc.CreateTicket(libticket.TicketCreateRequest{
			ProjectID: project.ID,
			Type:      "task",
			Title:     "Assigned to bob",
			Assignee:  "bob",
			Stage:     "develop",
			State:     "idle",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}
		if _, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
			Title:       task.Title,
			Description: task.Description,
			ParentID:    task.ParentID,
			Assignee:    task.Assignee,
			Stage:       "develop",
			State:       "active",
		}); err == nil {
			t.Fatal("UpdateTicket(lifecycle by non-assignee) error = nil")
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
