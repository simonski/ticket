package server

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

// handleProjectContext serves the project-scoped context graph (FACTORY.md §5.7):
//
//	GET /api/projects/{ref}/context        — full context graph (nodes + edges)
//	GET /api/projects/{ref}/context/search — search nodes by text (?q=)
func handleProjectContext(w http.ResponseWriter, req *http.Request, db *sql.DB, projectRef string, parts []string) bool {
	user, err := requireUser(db, req)
	if err != nil {
		writeAuthError(w, err)
		return true
	}
	project, role, err := resolveProjectPathForUser(req.Context(), db, user, projectRef, false)
	if err != nil {
		writeStoreError(w, err)
		return true
	}
	if !canReadProject(role) {
		writeAuthError(w, store.ErrForbidden)
		return true
	}
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return true
	}

	if len(parts) == 2 {
		graph, err := store.BuildContextGraph(req.Context(), db, project.ID)
		if err != nil {
			writeStoreError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, graph)
		return true
	}
	if len(parts) == 3 && parts[2] == "search" {
		query := strings.TrimSpace(req.URL.Query().Get("q"))
		nodes, err := store.SearchContext(req.Context(), db, project.ID, query)
		if err != nil {
			writeStoreError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, nodes)
		return true
	}
	writeError(w, http.StatusNotFound, "not found")
	return true
}
