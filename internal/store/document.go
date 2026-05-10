package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Document struct {
	ID          int64  `json:"document_id"`
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Notes       string `json:"notes"`
	Content     string `json:"content"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type DocumentFile struct {
	ID          int64  `json:"file_id"`
	DocumentID  int64  `json:"document_id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Content     []byte `json:"content,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func CreateDocument(ctx context.Context, db *sql.DB, projectID int64, title, description, notes, content string) (Document, error) {
	title = strings.TrimSpace(title)
	if projectID == 0 {
		return Document{}, errors.New("project is required")
	}
	if title == "" {
		return Document{}, errors.New("document title is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO documents (project_id, title, description, notes, content, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, projectID, title, strings.TrimSpace(description), strings.TrimSpace(notes), content)
	if err != nil {
		return Document{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Document{}, err
	}
	return GetDocument(ctx, db, id)
}

func ListDocumentsByProject(ctx context.Context, db *sql.DB, projectID int64) ([]Document, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT document_id, project_id, title, description, notes, content, created_at, updated_at
		FROM documents
		WHERE project_id = ?
		ORDER BY updated_at DESC, document_id DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs := make([]Document, 0)
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.ProjectID, &doc.Title, &doc.Description, &doc.Notes, &doc.Content, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func GetDocument(ctx context.Context, db *sql.DB, documentID int64) (Document, error) {
	row := db.QueryRowContext(ctx, `
		SELECT document_id, project_id, title, description, notes, content, created_at, updated_at
		FROM documents
		WHERE document_id = ?
	`, documentID)
	var doc Document
	if err := row.Scan(&doc.ID, &doc.ProjectID, &doc.Title, &doc.Description, &doc.Notes, &doc.Content, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func UpdateDocument(ctx context.Context, db *sql.DB, documentID int64, title, description, notes, content string) (Document, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Document{}, errors.New("document title is required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE documents
		SET title = ?, description = ?, notes = ?, content = ?, updated_at = CURRENT_TIMESTAMP
		WHERE document_id = ?
	`, title, strings.TrimSpace(description), strings.TrimSpace(notes), content, documentID)
	if err != nil {
		return Document{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Document{}, err
	}
	if affected == 0 {
		return Document{}, sql.ErrNoRows
	}
	return GetDocument(ctx, db, documentID)
}

func DeleteDocument(ctx context.Context, db *sql.DB, documentID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM documents WHERE document_id = ?`, documentID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func AddDocumentLabel(ctx context.Context, db *sql.DB, documentID, labelID int64) error {
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO document_labels (document_id, label_id) VALUES (?, ?)`, documentID, labelID)
	return err
}

func RemoveDocumentLabel(ctx context.Context, db *sql.DB, documentID, labelID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM document_labels WHERE document_id = ? AND label_id = ?`, documentID, labelID)
	return err
}

func ListDocumentLabels(ctx context.Context, db *sql.DB, documentID int64) ([]Label, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT l.label_id, l.project_id, l.name, l.color, l.created_at
		FROM labels l
		JOIN document_labels dl ON dl.label_id = l.label_id
		WHERE dl.document_id = ?
		ORDER BY l.name
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labels := make([]Label, 0)
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.ProjectID, &label.Name, &label.Color, &label.CreatedAt); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

func AddDocumentFile(ctx context.Context, db *sql.DB, documentID int64, fileName, contentType string, content []byte) (DocumentFile, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return DocumentFile{}, errors.New("file name is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO document_files (document_id, file_name, content_type, size_bytes, content)
		VALUES (?, ?, ?, ?, ?)
	`, documentID, fileName, strings.TrimSpace(contentType), int64(len(content)), content)
	if err != nil {
		return DocumentFile{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return DocumentFile{}, err
	}
	return GetDocumentFile(ctx, db, documentID, id)
}

func ListDocumentFiles(ctx context.Context, db *sql.DB, documentID int64) ([]DocumentFile, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT file_id, document_id, file_name, content_type, size_bytes, created_at
		FROM document_files
		WHERE document_id = ?
		ORDER BY created_at DESC, file_id DESC
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := make([]DocumentFile, 0)
	for rows.Next() {
		var file DocumentFile
		if err := rows.Scan(&file.ID, &file.DocumentID, &file.FileName, &file.ContentType, &file.SizeBytes, &file.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func GetDocumentFile(ctx context.Context, db *sql.DB, documentID, fileID int64) (DocumentFile, error) {
	row := db.QueryRowContext(ctx, `
		SELECT file_id, document_id, file_name, content_type, size_bytes, content, created_at
		FROM document_files
		WHERE document_id = ? AND file_id = ?
	`, documentID, fileID)
	var file DocumentFile
	if err := row.Scan(&file.ID, &file.DocumentID, &file.FileName, &file.ContentType, &file.SizeBytes, &file.Content, &file.CreatedAt); err != nil {
		return DocumentFile{}, err
	}
	return file, nil
}

func DeleteDocumentFile(ctx context.Context, db *sql.DB, documentID, fileID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM document_files WHERE document_id = ? AND file_id = ?`, documentID, fileID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
