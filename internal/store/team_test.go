package store

import "testing"

func TestTeamCRUDAndMemberships(t *testing.T) {
	db := testDB(t)

	parent, err := CreateTeam(db, "Platform", nil)
	if err != nil {
		t.Fatalf("CreateTeam(parent) error = %v", err)
	}
	child, err := CreateTeam(db, "Platform APIs", &parent.ID)
	if err != nil {
		t.Fatalf("CreateTeam(child) error = %v", err)
	}
	if child.ParentTeamID == nil || *child.ParentTeamID != parent.ID {
		t.Fatalf("child parent mismatch: %#v", child)
	}

	alice, err := CreateUser(db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	member, err := AddTeamMember(db, child.ID, alice.ID, TeamRoleOwner, "Lead Engineer")
	if err != nil {
		t.Fatalf("AddTeamMember(owner) error = %v", err)
	}
	if member.Role != TeamRoleOwner || member.JobTitle != "Lead Engineer" {
		t.Fatalf("team member mismatch: %#v", member)
	}

	agent, _, err := CreateAgent(db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	teamAgent, err := AddTeamAgent(db, child.ID, agent.ID)
	if err != nil {
		t.Fatalf("AddTeamAgent() error = %v", err)
	}
	if teamAgent.TeamID != child.ID || teamAgent.AgentID != agent.ID {
		t.Fatalf("team agent mismatch: %#v", teamAgent)
	}

	teams, err := ListTeams(db)
	if err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if len(teams) < 2 {
		t.Fatalf("ListTeams() len = %d, want >=2", len(teams))
	}
}

func TestProjectRoleViaTeamHierarchy(t *testing.T) {
	db := testDB(t)

	admin, err := GetUserByUsername(db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	user, err := CreateUser(db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	parent, err := CreateTeam(db, "Product", nil)
	if err != nil {
		t.Fatalf("CreateTeam(parent) error = %v", err)
	}
	child, err := CreateTeam(db, "Product Discovery", &parent.ID)
	if err != nil {
		t.Fatalf("CreateTeam(child) error = %v", err)
	}
	if _, err := AddTeamMember(db, child.ID, user.ID, TeamRoleMember, "Researcher"); err != nil {
		t.Fatalf("AddTeamMember(member) error = %v", err)
	}

	project, err := CreateProjectWithParams(db, ProjectCreateParams{
		Prefix:     "PRD",
		Title:      "Private Program",
		Visibility: ProjectVisibilityPrivate,
		CreatedBy:  admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	if _, err := AddProjectTeamMember(db, project.ID, parent.ID, ProjectRoleEditor); err != nil {
		t.Fatalf("AddProjectTeamMember(editor) error = %v", err)
	}

	visible, err := ListProjectsVisibleToUser(db, user)
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

	teamIDs, err := TeamIDsForUserWithAncestors(db, user.ID)
	if err != nil {
		t.Fatalf("TeamIDsForUserWithAncestors() error = %v", err)
	}
	role, ok, err := HighestProjectRoleForTeams(db, project.ID, teamIDs)
	if err != nil {
		t.Fatalf("HighestProjectRoleForTeams() error = %v", err)
	}
	if !ok || role != ProjectRoleEditor {
		t.Fatalf("team project role = %q (ok=%t), want editor/true", role, ok)
	}
}
