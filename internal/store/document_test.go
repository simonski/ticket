package store

import (
	"bytes"
	"context"
	"testing"
)

func TestDocumentCRUDLabelsAndFiles(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Docs Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	document, err := CreateDocument(ctx, db, project.ID, "Architecture", "high level", "review soon", "some text")
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}
	if document.ID == 0 {
		t.Fatal("CreateDocument() document.ID = 0")
	}

	documents, err := ListDocumentsByProject(ctx, db, project.ID)
	if err != nil {
		t.Fatalf("ListDocumentsByProject() error = %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("ListDocumentsByProject() len = %d, want 1", len(documents))
	}

	updated, err := UpdateDocument(ctx, db, document.ID, "Architecture v2", "updated", "updated notes", "updated text")
	if err != nil {
		t.Fatalf("UpdateDocument() error = %v", err)
	}
	if updated.Title != "Architecture v2" {
		t.Fatalf("UpdateDocument().Title = %q, want Architecture v2", updated.Title)
	}

	label, err := CreateLabel(ctx, db, project.ID, "docs", "#0055ff")
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if err := AddDocumentLabel(ctx, db, document.ID, label.ID); err != nil {
		t.Fatalf("AddDocumentLabel() error = %v", err)
	}
	labels, err := ListDocumentLabels(ctx, db, document.ID)
	if err != nil {
		t.Fatalf("ListDocumentLabels() error = %v", err)
	}
	if len(labels) != 1 || labels[0].ID != label.ID {
		t.Fatalf("ListDocumentLabels() = %#v, want label %d", labels, label.ID)
	}
	if err := RemoveDocumentLabel(ctx, db, document.ID, label.ID); err != nil {
		t.Fatalf("RemoveDocumentLabel() error = %v", err)
	}
	labels, err = ListDocumentLabels(ctx, db, document.ID)
	if err != nil {
		t.Fatalf("ListDocumentLabels() after remove error = %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("ListDocumentLabels() after remove len = %d, want 0", len(labels))
	}

	content := []byte("hello document file")
	file, err := AddDocumentFile(ctx, db, document.ID, "note.txt", "text/plain", content)
	if err != nil {
		t.Fatalf("AddDocumentFile() error = %v", err)
	}
	if file.ID == 0 {
		t.Fatal("AddDocumentFile() file.ID = 0")
	}

	files, err := ListDocumentFiles(ctx, db, document.ID)
	if err != nil {
		t.Fatalf("ListDocumentFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].ID != file.ID {
		t.Fatalf("ListDocumentFiles() = %#v, want file %d", files, file.ID)
	}

	fetchedFile, err := GetDocumentFile(ctx, db, document.ID, file.ID)
	if err != nil {
		t.Fatalf("GetDocumentFile() error = %v", err)
	}
	if !bytes.Equal(fetchedFile.Content, content) {
		t.Fatalf("GetDocumentFile().Content = %q, want %q", string(fetchedFile.Content), string(content))
	}

	if err := DeleteDocumentFile(ctx, db, document.ID, file.ID); err != nil {
		t.Fatalf("DeleteDocumentFile() error = %v", err)
	}
	files, err = ListDocumentFiles(ctx, db, document.ID)
	if err != nil {
		t.Fatalf("ListDocumentFiles() after delete error = %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("ListDocumentFiles() after delete len = %d, want 0", len(files))
	}

	if err := DeleteDocument(ctx, db, document.ID); err != nil {
		t.Fatalf("DeleteDocument() error = %v", err)
	}
	documents, err = ListDocumentsByProject(ctx, db, project.ID)
	if err != nil {
		t.Fatalf("ListDocumentsByProject() after delete error = %v", err)
	}
	if len(documents) != 0 {
		t.Fatalf("ListDocumentsByProject() after delete len = %d, want 0", len(documents))
	}
}

func TestDeleteLabelRemovesDocumentLabelLinks(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Label Cascade Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	document, err := CreateDocument(ctx, db, project.ID, "Doc", "", "", "")
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}
	label, err := CreateLabel(ctx, db, project.ID, "linked", "#123456")
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if err := AddDocumentLabel(ctx, db, document.ID, label.ID); err != nil {
		t.Fatalf("AddDocumentLabel() error = %v", err)
	}

	if err := DeleteLabel(ctx, db, label.ID); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}

	labels, err := ListDocumentLabels(ctx, db, document.ID)
	if err != nil {
		t.Fatalf("ListDocumentLabels() error = %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("ListDocumentLabels() after label delete len = %d, want 0", len(labels))
	}
}
