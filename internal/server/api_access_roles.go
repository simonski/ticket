package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

// gatedPanelPrefixes maps API path prefixes to the panel that guards them.
// Only clearly panel-scoped, non-foundational endpoints are server-enforced;
// foundational data (projects, tickets, status) stays ungated so restricting a
// panel never breaks the board. Admin-tier panels keep their own requireAdmin
// checks. The web nav additionally hides panels a user cannot access.
var gatedPanelPrefixes = []struct {
	prefix string
	panel  string
}{
	{"/api/rooms", store.PanelChat},
	{"/api/workflows", store.PanelWorkflows},
	{"/api/roles", store.PanelRoles},
	{"/api/documents", store.PanelDocuments},
	{"/api/teams", store.PanelTeams},
}

func panelForPath(path string) (string, bool) {
	for _, g := range gatedPanelPrefixes {
		if path == g.prefix || strings.HasPrefix(path, g.prefix+"/") {
			return g.panel, true
		}
	}
	return "", false
}

// panelAccessMiddleware enforces per-panel access for authenticated non-admin
// users on gated endpoints. Unauthenticated requests fall through so the handler
// can return its own 401; admins always pass.
func panelAccessMiddleware(next http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panel, gated := panelForPath(r.URL.Path)
		if !gated {
			next.ServeHTTP(w, r)
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if user.Role == "admin" {
			next.ServeHTTP(w, r)
			return
		}
		ok, cerr := store.UserCanAccessPanel(r.Context(), db, user.ID, false, panel)
		if cerr != nil {
			writeError(w, http.StatusInternalServerError, cerr.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "access to this panel is not permitted")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// registerAccessRoleHandlers wires the access-role (per-panel feature flag) API
// (TK-135). All endpoints are admin-only except GET /api/me/panels, which lets
// the current user discover their own effective panel set.
func (r *router) registerAccessRoleHandlers() {
	db := r.db
	mux := r.mux

	// Current user's effective panel set — used by the web nav to decide what to render.
	mux.HandleFunc("/api/me/panels", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		panels, perr := store.EffectivePanels(req.Context(), db, user.ID, user.Role == "admin")
		if perr != nil {
			writeStoreError(w, perr)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"panels": panels})
	})

	// Catalogue of panels an access role may grant (admin, for the editor UI).
	mux.HandleFunc("/api/panels", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, err := requireAdmin(db, req); err != nil {
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"all":       store.AllPanels(),
			"grantable": store.GrantablePanels(),
		})
	})

	// Collection: list + create.
	mux.HandleFunc("/api/access-roles", func(w http.ResponseWriter, req *http.Request) {
		if _, err := requireAdmin(db, req); err != nil {
			writeAuthError(w, err)
			return
		}
		switch req.Method {
		case http.MethodGet:
			roles, lerr := store.ListAccessRoles(req.Context(), db)
			if lerr != nil {
				writeStoreError(w, lerr)
				return
			}
			if roles == nil {
				roles = []store.AccessRole{}
			}
			writeJSON(w, http.StatusOK, roles)
		case http.MethodPost:
			var body accessRoleBody
			if derr := decodeJSONBody(req, &body); derr != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			role, cerr := store.CreateAccessRole(req.Context(), db, body.Name, body.Description, body.Panels)
			if cerr != nil {
				writeStoreError(w, cerr)
				return
			}
			writeJSON(w, http.StatusCreated, role)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Membership: read/replace a user's access-role assignments.
	mux.HandleFunc("/api/access-roles/memberships", func(w http.ResponseWriter, req *http.Request) {
		if _, err := requireAdmin(db, req); err != nil {
			writeAuthError(w, err)
			return
		}
		switch req.Method {
		case http.MethodGet:
			userID := strings.TrimSpace(req.URL.Query().Get("user_id"))
			if userID == "" {
				writeError(w, http.StatusBadRequest, "user_id is required")
				return
			}
			ids, gerr := store.UserAccessRoleIDs(req.Context(), db, userID)
			if gerr != nil {
				writeStoreError(w, gerr)
				return
			}
			if ids == nil {
				ids = []int64{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "role_ids": ids})
		case http.MethodPut:
			var body struct {
				UserID  string  `json:"user_id"`
				RoleIDs []int64 `json:"role_ids"`
			}
			if derr := decodeJSONBody(req, &body); derr != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			if strings.TrimSpace(body.UserID) == "" {
				writeError(w, http.StatusBadRequest, "user_id is required")
				return
			}
			if serr := store.SetUserAccessRoles(req.Context(), db, body.UserID, body.RoleIDs); serr != nil {
				writeStoreError(w, serr)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"user_id": body.UserID, "role_ids": body.RoleIDs})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	// Item: get / update / delete by id.
	mux.HandleFunc("/api/access-roles/", func(w http.ResponseWriter, req *http.Request) {
		if _, err := requireAdmin(db, req); err != nil {
			writeAuthError(w, err)
			return
		}
		idStr := strings.TrimPrefix(req.URL.Path, "/api/access-roles/")
		id, perr := strconv.ParseInt(idStr, 10, 64)
		if perr != nil {
			writeError(w, http.StatusNotFound, "access role not found")
			return
		}
		switch req.Method {
		case http.MethodGet:
			role, gerr := store.GetAccessRole(req.Context(), db, id)
			if gerr != nil {
				writeAccessRoleError(w, gerr)
				return
			}
			writeJSON(w, http.StatusOK, role)
		case http.MethodPut:
			var body accessRoleBody
			if derr := decodeJSONBody(req, &body); derr != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			role, uerr := store.UpdateAccessRole(req.Context(), db, id, body.Name, body.Description, body.Panels)
			if uerr != nil {
				writeAccessRoleError(w, uerr)
				return
			}
			writeJSON(w, http.StatusOK, role)
		case http.MethodDelete:
			if derr := store.DeleteAccessRole(req.Context(), db, id); derr != nil {
				writeAccessRoleError(w, derr)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}

type accessRoleBody struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Panels      []string `json:"panels"`
}

func writeAccessRoleError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrAccessRoleNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeStoreError(w, err)
}
