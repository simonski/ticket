package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerDocumentHandlers() {
	db := r.db
	mux := r.mux

	mux.HandleFunc("/api/documents/", func(w http.ResponseWriter, req *http.Request) {
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(req.URL.Path, "/api/documents/")
		parts := strings.Split(strings.Trim(trimmed, "/"), "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusBadRequest, "invalid document id")
			return
		}
		documentID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || documentID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid document id")
			return
		}
		document, err := store.GetDocument(req.Context(), db, documentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "document not found")
				return
			}
			writeStoreError(w, err)
			return
		}
		role, err := projectRoleForUser(req.Context(), db, document.ProjectID, user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if len(parts) == 1 {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				writeJSON(w, http.StatusOK, document)
				return
			case http.MethodPut:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload documentRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				updated, updateErr := store.UpdateDocument(req.Context(), db, documentID, payload.Title, payload.Description, payload.Notes, payload.Content)
				if updateErr != nil {
					writeStoreError(w, updateErr)
					return
				}
				writeJSON(w, http.StatusOK, updated)
				return
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if err := store.DeleteDocument(req.Context(), db, documentID); err != nil {
					writeStoreError(w, err)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}

		if len(parts) == 2 && parts[1] == "labels" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				labels, err := store.ListDocumentLabels(req.Context(), db, documentID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, labels)
				return
			case http.MethodPost:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload documentLabelRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if payload.LabelID <= 0 {
					writeError(w, http.StatusBadRequest, "label_id is required")
					return
				}
				if err := store.AddDocumentLabel(req.Context(), db, documentID, payload.LabelID); err != nil {
					writeStoreError(w, err)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				labelID, err := strconv.ParseInt(strings.TrimSpace(req.URL.Query().Get("label_id")), 10, 64)
				if err != nil || labelID <= 0 {
					writeError(w, http.StatusBadRequest, "label_id must be numeric")
					return
				}
				if err := store.RemoveDocumentLabel(req.Context(), db, documentID, labelID); err != nil {
					writeStoreError(w, err)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}

		if len(parts) == 2 && parts[1] == "files" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				files, err := store.ListDocumentFiles(req.Context(), db, documentID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, files)
				return
			case http.MethodPost:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload documentFileRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				file, err := store.AddDocumentFile(req.Context(), db, documentID, payload.FileName, payload.ContentType, payload.Content)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				file.Content = nil
				writeJSON(w, http.StatusCreated, file)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}

		if len(parts) == 3 && parts[1] == "files" {
			fileID, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil || fileID <= 0 {
				writeError(w, http.StatusBadRequest, "invalid file id")
				return
			}
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				file, err := store.GetDocumentFile(req.Context(), db, documentID, fileID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "document file not found")
						return
					}
					writeStoreError(w, err)
					return
				}
				contentType := strings.TrimSpace(file.ContentType)
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				w.Header().Set("Content-Type", contentType)
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.FileName))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(file.Content)
				return
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if err := store.DeleteDocumentFile(req.Context(), db, documentID, fileID); err != nil {
					writeStoreError(w, err)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}

		writeError(w, http.StatusNotFound, "not found")
	})
}

func handleProjectDocuments(w http.ResponseWriter, req *http.Request, db *sql.DB, projectRef string) bool {
	if !strings.HasPrefix(req.URL.Path, "/api/projects/") {
		return false
	}
	project, err := store.GetProject(req.Context(), db, projectRef)
	if err != nil {
		writeStoreError(w, err)
		return true
	}
	user, err := requireUser(db, req)
	if err != nil {
		writeAuthError(w, err)
		return true
	}
	role, err := projectRoleForUser(req.Context(), db, project.ID, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	switch req.Method {
	case http.MethodGet:
		if !canReadProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return true
		}
		documents, err := store.ListDocumentsByProject(req.Context(), db, project.ID)
		if err != nil {
			writeStoreError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, documents)
		return true
	case http.MethodPost:
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return true
		}
		var payload documentRequest
		if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return true
		}
		document, err := store.CreateDocument(req.Context(), db, project.ID, payload.Title, payload.Description, payload.Notes, payload.Content)
		if err != nil {
			writeStoreError(w, err)
			return true
		}
		writeJSON(w, http.StatusCreated, document)
		return true
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return true
	}
}
