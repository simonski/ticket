package store

import (
	"errors"
	"testing"
)

func TestProjectMemberCRUD(t *testing.T) {
	db := testDB(t)

	admin, err := GetUserByUsername(db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	alice, err := CreateUser(db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	project, err := CreateProjectWithParams(db, ProjectCreateParams{
		Prefix:     "PM",
		Title:      "Members Project",
		Visibility: ProjectVisibilityPrivate,
		CreatedBy:  admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}

	// Add member
	member, err := AddProjectMember(db, project.ID, alice.ID, ProjectRoleEditor)
	if err != nil {
		t.Fatalf("AddProjectMember() error = %v", err)
	}
	if member.Role != ProjectRoleEditor {
		t.Fatalf("AddProjectMember().Role = %q, want editor", member.Role)
	}
	if member.Username != "alice" {
		t.Fatalf("AddProjectMember().Username = %q, want alice", member.Username)
	}

	// List members (admin is auto-added as owner when creating the project)
	members, err := ListProjectMembers(db, project.ID)
	if err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("ListProjectMembers() len = %d, want 2 (admin + alice)", len(members))
	}

	// ProjectRoleForUser
	role, found, err := ProjectRoleForUser(db, project.ID, alice.ID)
	if err != nil {
		t.Fatalf("ProjectRoleForUser() error = %v", err)
	}
	if !found {
		t.Fatal("ProjectRoleForUser() found = false, want true")
	}
	if role != ProjectRoleEditor {
		t.Fatalf("ProjectRoleForUser() = %q, want editor", role)
	}

	// ProjectRoleForUser for non-member
	bob, err := CreateUser(db, "bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	_, found, err = ProjectRoleForUser(db, project.ID, bob.ID)
	if err != nil {
		t.Fatalf("ProjectRoleForUser(bob) error = %v", err)
	}
	if found {
		t.Fatal("ProjectRoleForUser(bob) found = true, want false")
	}

	// Remove member
	if err := RemoveProjectMember(db, project.ID, alice.ID); err != nil {
		t.Fatalf("RemoveProjectMember() error = %v", err)
	}

	// Remove again should fail
	if err := RemoveProjectMember(db, project.ID, alice.ID); !errors.Is(err, ErrProjectMembershipNotFound) {
		t.Fatalf("RemoveProjectMember(again) error = %v, want ErrProjectMembershipNotFound", err)
	}

	// AddProjectMember with invalid role should fail
	if _, err := AddProjectMember(db, project.ID, alice.ID, "invalid"); err == nil {
		t.Fatal("AddProjectMember(invalid role) error = nil, want error")
	}

	// List should only have admin left
	members, err = ListProjectMembers(db, project.ID)
	if err != nil {
		t.Fatalf("ListProjectMembers() after remove error = %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("ListProjectMembers() after remove len = %d, want 1 (admin only)", len(members))
	}
}
