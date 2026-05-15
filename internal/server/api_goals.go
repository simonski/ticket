package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerGoalHandlers() {
	db := r.db
	mux := r.mux

	mux.HandleFunc("/api/goals/", func(w http.ResponseWriter, req *http.Request) {
		user, err := requireUser(db, req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(req.URL.Path, "/api/goals/")
		parts := strings.Split(trimmed, "/")
		if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		goalID, parseErr := strconv.ParseInt(parts[0], 10, 64)
		if parseErr != nil || goalID < 1 {
			writeError(w, http.StatusBadRequest, "invalid goal id")
			return
		}
		goal, err := store.GetGoal(req.Context(), db, goalID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		role, err := projectRoleForUser(req.Context(), db, goal.ProjectID, user)
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
				writeJSON(w, http.StatusOK, goal)
			case http.MethodPut:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if _, updateErr := store.UpdateGoal(req.Context(), db, goalID, payload.Title, payload.Description, payload.Notes, payload.ETA, payload.Priority); updateErr != nil {
					writeStoreError(w, updateErr)
					return
				}
				updated, updateErr := store.SetGoalAgentModelConfig(req.Context(), db, goalID, store.AgentModelConfig{
					Provider: payload.AgentModelProvider,
					Model:    payload.AgentModelName,
					URL:      payload.AgentModelURL,
					APIKey:   payload.AgentModelAPIKey,
				})
				if updateErr != nil {
					writeStoreError(w, updateErr)
					return
				}
				writeJSON(w, http.StatusOK, updated)
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if deleteErr := store.DeleteGoal(req.Context(), db, goalID); deleteErr != nil {
					writeStoreError(w, deleteErr)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}
		if len(parts) == 2 && req.Method == http.MethodPost {
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			switch parts[1] {
			case "refine":
				updated, updateErr := store.SetGoalStatus(req.Context(), db, goalID, "refining")
				if updateErr != nil {
					writeStoreError(w, updateErr)
					return
				}
				writeJSON(w, http.StatusOK, updated)
				return
			case "ready":
				var payload goalReadyRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if !payload.ConfirmRefinement {
					writeError(w, http.StatusBadRequest, "confirm_refinement must be true before setting ready")
					return
				}
				if _, err := store.ConfirmGoalRefinement(req.Context(), db, goalID, true); err != nil {
					writeStoreError(w, err)
					return
				}
				updated, updateErr := store.SetGoalStatus(req.Context(), db, goalID, "ready")
				if updateErr != nil {
					writeStoreError(w, updateErr)
					return
				}
				writeJSON(w, http.StatusOK, updated)
				return
			}
		}
		if len(parts) == 2 && parts[1] == "agent-model" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				writeJSON(w, http.StatusOK, store.AgentModelConfig{
					Provider: goal.AgentModelProvider,
					Model:    goal.AgentModelName,
					URL:      goal.AgentModelURL,
					APIKey:   goal.AgentModelAPIKey,
				})
				return
			case http.MethodPut:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload agentModelConfigRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				updated, err := store.SetGoalAgentModelConfig(req.Context(), db, goalID, store.AgentModelConfig{
					Provider: payload.Provider,
					Model:    payload.Model,
					URL:      payload.URL,
					APIKey:   payload.APIKey,
				})
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, updated)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		if len(parts) == 3 && parts[1] == "agent-model" && parts[2] == "resolved" {
			if req.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if !canReadProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			resolved, err := store.ResolveGoalAgentModelConfig(req.Context(), db, goalID)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, resolved)
			return
		}
		if len(parts) == 3 && parts[1] == "refinement" && parts[2] == "confirm" {
			if req.Method != http.MethodPost {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			var payload goalRefinementConfirmRequest
			if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			updated, err := store.ConfirmGoalRefinement(req.Context(), db, goalID, payload.Confirmed)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
		if len(parts) == 2 && parts[1] == "refinement" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"refined_goal":  goal.RefinedGoal,
					"decomposition": goal.Decompose,
				})
				return
			case http.MethodPut:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalRefinementRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				updated, updateErr := store.UpdateGoalRefinement(req.Context(), db, goalID, payload.RefinedGoal, payload.Decomposition)
				if updateErr != nil {
					writeStoreError(w, updateErr)
					return
				}
				writeJSON(w, http.StatusOK, updated)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		if len(parts) == 2 && parts[1] == "decomposition" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				items, err := store.ListGoalDecompositionItems(req.Context(), db, goalID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, items)
				return
			case http.MethodPost:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalDecompositionItemRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				item, err := store.CreateGoalDecompositionItem(req.Context(), db, goalID, payload.Kind, payload.Text, payload.SortOrder)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, item)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		if len(parts) == 3 && parts[1] == "decomposition" && parts[2] == "reorder" {
			if req.Method != http.MethodPut {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			var payload goalDecompositionReorderRequest
			if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			if err := store.ReorderGoalDecompositionItems(req.Context(), db, goalID, payload.ItemIDs); err != nil {
				writeStoreError(w, err)
				return
			}
			items, err := store.ListGoalDecompositionItems(req.Context(), db, goalID)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, items)
			return
		}
		if len(parts) == 3 && parts[1] == "decomposition" {
			itemID, parseErr := strconv.ParseInt(parts[2], 10, 64)
			if parseErr != nil || itemID < 1 {
				writeError(w, http.StatusBadRequest, "invalid decomposition item id")
				return
			}
			switch req.Method {
			case http.MethodPut:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalDecompositionItemRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				item, err := store.UpdateGoalDecompositionItem(req.Context(), db, goalID, itemID, payload.Kind, payload.Text)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, item)
				return
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if err := store.DeleteGoalDecompositionItem(req.Context(), db, goalID, itemID); err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"ok": true})
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		if len(parts) == 2 && parts[1] == "clarifications" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				clarifications, err := store.ListGoalClarifications(req.Context(), db, goalID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, clarifications)
				return
			case http.MethodPost:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalClarificationRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				clarification, err := store.AddGoalClarification(req.Context(), db, goalID, payload.Question)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, clarification)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		if len(parts) == 3 && parts[1] == "clarifications" {
			clarificationID, parseErr := strconv.ParseInt(parts[2], 10, 64)
			if parseErr != nil || clarificationID < 1 {
				writeError(w, http.StatusBadRequest, "invalid clarification id")
				return
			}
			if req.Method != http.MethodPut {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			var payload goalClarificationResolveRequest
			if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			clarification, err := store.SetGoalClarificationResolved(req.Context(), db, goalID, clarificationID, payload.Resolved)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, clarification)
			return
		}
		if len(parts) == 3 && parts[1] == "chat" && parts[2] == "messages" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				messages, listErr := store.ListGoalChatMessages(req.Context(), db, goalID, 300)
				if listErr != nil {
					writeStoreError(w, listErr)
					return
				}
				writeJSON(w, http.StatusOK, messages)
				return
			case http.MethodPost:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalChatMessageRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				created, createErr := store.AddGoalChatMessage(req.Context(), db, goalID, payload.Author, payload.Text)
				if createErr != nil {
					writeStoreError(w, createErr)
					return
				}
				writeJSON(w, http.StatusCreated, created)
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		if len(parts) == 2 && parts[1] == "stories" {
			switch req.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				stories, listErr := store.ListStoriesForGoal(req.Context(), db, goalID)
				if listErr != nil {
					writeStoreError(w, listErr)
					return
				}
				writeJSON(w, http.StatusOK, stories)
				return
			case http.MethodPost:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalStoryLinkRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if err := store.LinkGoalToStory(req.Context(), db, goalID, payload.StoryID); err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
				return
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload goalStoryLinkRequest
				if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				if err := store.UnlinkGoalFromStory(req.Context(), db, goalID, payload.StoryID); err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"ok": true})
				return
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
		writeError(w, http.StatusNotFound, "not found")
	})
}

func handleProjectGoalInbox(w http.ResponseWriter, req *http.Request, db *sql.DB, projectRef string) bool {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return true
	}
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
	statusFilter := strings.TrimSpace(req.URL.Query().Get("status"))
	sort := strings.TrimSpace(req.URL.Query().Get("sort"))
	items, err := store.ListGoalInbox(req.Context(), db, project.ID, statusFilter, sort)
	if err != nil {
		writeStoreError(w, err)
		return true
	}
	writeJSON(w, http.StatusOK, items)
	return true
}

func handleProjectGoals(w http.ResponseWriter, req *http.Request, db *sql.DB, projectRef string) bool {
	if !strings.HasPrefix(req.URL.Path, "/api/projects/") {
		return false
	}
	user, err := requireUser(db, req)
	if err != nil {
		writeAuthError(w, err)
		return true
	}
	project, role, err := resolveProjectPathForUser(req.Context(), db, user, projectRef, req.Method == http.MethodPost)
	if err != nil {
		writeStoreError(w, err)
		return true
	}
	switch req.Method {
	case http.MethodGet:
		if !canReadProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return true
		}
		goals, listErr := store.ListGoals(req.Context(), db, project.ID)
		if listErr != nil {
			writeStoreError(w, listErr)
			return true
		}
		writeJSON(w, http.StatusOK, goals)
		return true
	case http.MethodPost:
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return true
		}
		var payload goalRequest
		if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return true
		}
		goal, createErr := store.CreateGoal(req.Context(), db, project.ID, payload.Title, payload.Description, payload.Notes, payload.ETA, payload.Priority)
		if createErr != nil {
			writeStoreError(w, createErr)
			return true
		}
		goal, createErr = store.SetGoalAgentModelConfig(req.Context(), db, goal.ID, store.AgentModelConfig{
			Provider: payload.AgentModelProvider,
			Model:    payload.AgentModelName,
			URL:      payload.AgentModelURL,
			APIKey:   payload.AgentModelAPIKey,
		})
		if createErr != nil {
			writeStoreError(w, createErr)
			return true
		}
		writeJSON(w, http.StatusCreated, goal)
		return true
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return true
	}
}
