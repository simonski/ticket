package store

import (
	"context"
	"testing"
)

func TestTeamCRUDAndMemberships(t *testing.T) {
	db := testDB(t)

	parent, err := CreateTeam(context.Background(), db, "Platform", nil)
	if err != nil {
		t.Fatalf("CreateTeam(parent) error = %v", err)
	}
	child, err := CreateTeam(context.Background(), db, "Platform APIs", &parent.ID)
	if err != nil {
		t.Fatalf("CreateTeam(child) error = %v", err)
	}
	if child.ParentTeamID == nil || *child.ParentTeamID != parent.ID {
		t.Fatalf("child parent mismatch: %#v", child)
	}

	alice, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	member, err := AddTeamMember(context.Background(), db, child.ID, alice.ID, TeamRoleOwner, "Lead Engineer")
	if err != nil {
		t.Fatalf("AddTeamMember(owner) error = %v", err)
	}
	if member.Role != TeamRoleOwner || member.JobTitle != "Lead Engineer" {
		t.Fatalf("team member mismatch: %#v", member)
	}

	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	teamAgent, err := AddTeamAgent(context.Background(), db, child.ID, agent.ID)
	if err != nil {
		t.Fatalf("AddTeamAgent() error = %v", err)
	}
	if teamAgent.TeamID != child.ID || teamAgent.AgentID != agent.ID {
		t.Fatalf("team agent mismatch: %#v", teamAgent)
	}

	teams, err := ListTeams(context.Background(), db)
	if err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if len(teams) < 2 {
		t.Fatalf("ListTeams() len = %d, want >=2", len(teams))
	}
}

func TestTeamUpdateAndDelete(t *testing.T) {
	db := testDB(t)

	team, err := CreateTeam(context.Background(), db, "Alpha", nil)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}

	updated, err := UpdateTeam(context.Background(), db, team.ID, "Beta", nil)
	if err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if updated.Name != "Beta" {
		t.Fatalf("UpdateTeam().Name = %q, want Beta", updated.Name)
	}

	if err := DeleteTeam(context.Background(), db, team.ID); err != nil {
		t.Fatalf("DeleteTeam() error = %v", err)
	}
	if err := DeleteTeam(context.Background(), db, team.ID); err == nil {
		t.Fatal("DeleteTeam(deleted) error = nil, want error")
	}
}

func TestUpdateTeamWithParentCycleProtection(t *testing.T) {
	db := testDB(t)

	root, err := CreateTeam(context.Background(), db, "Root", nil)
	if err != nil {
		t.Fatalf("CreateTeam(Root) error = %v", err)
	}
	child, err := CreateTeam(context.Background(), db, "Child", &root.ID)
	if err != nil {
		t.Fatalf("CreateTeam(Child) error = %v", err)
	}

	// Cannot be its own parent
	if _, err := UpdateTeam(context.Background(), db, root.ID, "", &root.ID); err == nil {
		t.Fatal("UpdateTeam(self-parent) error = nil, want error")
	}

	// Cannot create cycle: root -> child -> root
	if _, err := UpdateTeam(context.Background(), db, root.ID, "", &child.ID); err == nil {
		t.Fatal("UpdateTeam(cycle) error = nil, want error")
	}

	// Valid update: set child's parent to nil
	updated, err := UpdateTeam(context.Background(), db, child.ID, "Renamed Child", nil)
	if err != nil {
		t.Fatalf("UpdateTeam(valid) error = %v", err)
	}
	if updated.Name != "Renamed Child" {
		t.Fatalf("UpdateTeam().Name = %q, want Renamed Child", updated.Name)
	}
	if updated.ParentTeamID != nil {
		t.Fatalf("UpdateTeam().ParentTeamID = %v, want nil", updated.ParentTeamID)
	}

	// Update with empty name keeps old name
	kept, err := UpdateTeam(context.Background(), db, child.ID, "", nil)
	if err != nil {
		t.Fatalf("UpdateTeam(empty name) error = %v", err)
	}
	if kept.Name != "Renamed Child" {
		t.Fatalf("UpdateTeam(empty name).Name = %q, want Renamed Child", kept.Name)
	}
}

func TestTeamMemberRemoveAndList(t *testing.T) {
	db := testDB(t)

	team, err := CreateTeam(context.Background(), db, "Gamma", nil)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	alice, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	bob, err := CreateUser(context.Background(), db, "bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}

	if _, err := AddTeamMember(context.Background(), db, team.ID, alice.ID, TeamRoleMember, "Dev"); err != nil {
		t.Fatalf("AddTeamMember(alice) error = %v", err)
	}
	if _, err := AddTeamMember(context.Background(), db, team.ID, bob.ID, TeamRoleOwner, "Lead"); err != nil {
		t.Fatalf("AddTeamMember(bob) error = %v", err)
	}

	members, err := ListTeamMembers(context.Background(), db, team.ID)
	if err != nil {
		t.Fatalf("ListTeamMembers() error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("ListTeamMembers() len = %d, want 2", len(members))
	}

	role, found, err := TeamRoleForUser(context.Background(), db, team.ID, alice.ID)
	if err != nil {
		t.Fatalf("TeamRoleForUser() error = %v", err)
	}
	if !found || role != TeamRoleMember {
		t.Fatalf("TeamRoleForUser() = (%q, %t), want (member, true)", role, found)
	}

	if err := RemoveTeamMember(context.Background(), db, team.ID, alice.ID); err != nil {
		t.Fatalf("RemoveTeamMember() error = %v", err)
	}
	if err := RemoveTeamMember(context.Background(), db, team.ID, alice.ID); err == nil {
		t.Fatal("RemoveTeamMember(again) error = nil, want error")
	}
}

func TestTeamAgentRemoveAndList(t *testing.T) {
	db := testDB(t)

	team, err := CreateTeam(context.Background(), db, "Delta", nil)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if _, err := AddTeamAgent(context.Background(), db, team.ID, agent.ID); err != nil {
		t.Fatalf("AddTeamAgent() error = %v", err)
	}

	agents, err := ListTeamAgents(context.Background(), db, team.ID)
	if err != nil {
		t.Fatalf("ListTeamAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("ListTeamAgents() len = %d, want 1", len(agents))
	}

	if err := RemoveTeamAgent(context.Background(), db, team.ID, agent.ID); err != nil {
		t.Fatalf("RemoveTeamAgent() error = %v", err)
	}
	if err := RemoveTeamAgent(context.Background(), db, team.ID, agent.ID); err == nil {
		t.Fatal("RemoveTeamAgent(again) error = nil, want error")
	}
}

func TestProjectTeamMemberRemoveAndList(t *testing.T) {
	db := testDB(t)

	admin, err := GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	project, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix:    "PTM",
		Title:     "Team Project",
		CreatedBy: admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	team, err := CreateTeam(context.Background(), db, "Epsilon", nil)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}

	if _, err := AddProjectTeamMember(context.Background(), db, project.ID, team.ID, ProjectRoleViewer); err != nil {
		t.Fatalf("AddProjectTeamMember() error = %v", err)
	}

	members, err := ListProjectTeamMembers(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListProjectTeamMembers() error = %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("ListProjectTeamMembers() len = %d, want 1", len(members))
	}

	if err := RemoveProjectTeamMember(context.Background(), db, project.ID, team.ID); err != nil {
		t.Fatalf("RemoveProjectTeamMember() error = %v", err)
	}
	if err := RemoveProjectTeamMember(context.Background(), db, project.ID, team.ID); err == nil {
		t.Fatal("RemoveProjectTeamMember(again) error = nil, want error")
	}
}

func TestTeamDescendantIDs(t *testing.T) {
	db := testDB(t)

	root, err := CreateTeam(context.Background(), db, "Root", nil)
	if err != nil {
		t.Fatalf("CreateTeam(Root) error = %v", err)
	}
	child, err := CreateTeam(context.Background(), db, "Child", &root.ID)
	if err != nil {
		t.Fatalf("CreateTeam(Child) error = %v", err)
	}
	grandchild, err := CreateTeam(context.Background(), db, "Grandchild", &child.ID)
	if err != nil {
		t.Fatalf("CreateTeam(Grandchild) error = %v", err)
	}

	descendants, err := TeamDescendantIDs(context.Background(), db, root.ID)
	if err != nil {
		t.Fatalf("TeamDescendantIDs() error = %v", err)
	}
	if len(descendants) != 2 {
		t.Fatalf("TeamDescendantIDs() len = %d, want 2", len(descendants))
	}
	found := map[int64]bool{}
	for _, id := range descendants {
		found[id] = true
	}
	if !found[child.ID] || !found[grandchild.ID] {
		t.Fatalf("TeamDescendantIDs() = %v, want [%d, %d]", descendants, child.ID, grandchild.ID)
	}
}

func TestProjectRoleViaTeamHierarchy(t *testing.T) {
	db := testDB(t)

	admin, err := GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	user, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	parent, err := CreateTeam(context.Background(), db, "Product", nil)
	if err != nil {
		t.Fatalf("CreateTeam(parent) error = %v", err)
	}
	child, err := CreateTeam(context.Background(), db, "Product Discovery", &parent.ID)
	if err != nil {
		t.Fatalf("CreateTeam(child) error = %v", err)
	}
	if _, err := AddTeamMember(context.Background(), db, child.ID, user.ID, TeamRoleMember, "Researcher"); err != nil {
		t.Fatalf("AddTeamMember(member) error = %v", err)
	}

	project, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix:     "PRD",
		Title:      "Private Program",
		Visibility: ProjectVisibilityPrivate,
		CreatedBy:  admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	if _, err := AddProjectTeamMember(context.Background(), db, project.ID, parent.ID, ProjectRoleEditor); err != nil {
		t.Fatalf("AddProjectTeamMember(editor) error = %v", err)
	}

	visible, err := ListProjectsVisibleToUser(context.Background(), db, user)
	if err != nil {
		t.Fatalf("ListProjectsVisibleToUser() error = %v", err)
	}
	found := false
	for _, p := range visible {
		if p.ID == project.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("private project should be visible via team membership")
	}

	teamIDs, err := TeamIDsForUserWithAncestors(context.Background(), db, user.ID)
	if err != nil {
		t.Fatalf("TeamIDsForUserWithAncestors() error = %v", err)
	}
	role, ok, err := HighestProjectRoleForTeams(context.Background(), db, project.ID, teamIDs)
	if err != nil {
		t.Fatalf("HighestProjectRoleForTeams() error = %v", err)
	}
	if !ok || role != ProjectRoleEditor {
		t.Fatalf("team project role = %q (ok=%t), want editor/true", role, ok)
	}
}
