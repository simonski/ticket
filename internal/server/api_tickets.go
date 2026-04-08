package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func (r *router) registerTicketHandlers() {
	db := r.db
	mux := r.mux
	notify := r.notify

	handleTicketsCollection := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		var ticketPayload ticketRequest
		if err := json.NewDecoder(r.Body).Decode(&ticketPayload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		_, state, _ := resolveLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
		role, err := projectRoleForUser(r.Context(), db, ticketPayload.ProjectID, user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return
		}
		ticket, err := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
			ProjectID:          ticketPayload.ProjectID,
			ParentID:           ticketPayload.ParentID,
			Type:               ticketPayload.Type,
			Title:              ticketPayload.Title,
			Description:        ticketPayload.Description,
			AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
			GitRepository:      ticketPayload.GitRepository,
			GitBranch:          ticketPayload.GitBranch,
			Priority:           ticketPayload.Priority,
			EstimateEffort:     ticketPayload.EstimateEffort,
			EstimateComplete:   ticketPayload.EstimateComplete,
			Assignee:           ticketPayload.Assignee,
			State:              state,
			Author:             user.Username,
			CreatedBy:          user.ID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if ticketPayload.Message != "" {
			store.AddComment(r.Context(), db, ticket.ID, user.ID, ticketPayload.Message)
		}
		notify("ticket_created", ticket.ProjectID, ticket.ID)
		writeJSON(w, http.StatusCreated, ticket)
	}
	mux.HandleFunc("/api/tickets", handleTicketsCollection)

	mux.HandleFunc("/api/stories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			var payload storyRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, payload.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			story, err := store.CreateStory(r.Context(), db, payload.ProjectID, payload.Title, payload.Description, user.ID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, story)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/stories/", func(w http.ResponseWriter, r *http.Request) {
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/stories/")
		parts := strings.Split(trimmed, "/")
		var storyID int64
		if _, err := fmt.Sscan(parts[0], &storyID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid story id")
			return
		}
		story, err := store.GetStory(r.Context(), db, storyID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "story not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		role, err := projectRoleForUser(r.Context(), db, story.ProjectID, user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return
		}
		if len(parts) == 1 && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, story)
			return
		}
		if len(parts) == 1 && r.Method == http.MethodDelete {
			if err := store.DeleteStory(r.Context(), db, story.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if len(parts) == 1 && r.Method == http.MethodPut {
			var payload storyRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			updated, err := store.UpdateStory(r.Context(), db, story.ID, payload.Title, payload.Description)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "story not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
		if len(parts) != 2 || parts[1] != "analyse" || r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		project, err := store.GetProjectByID(r.Context(), db, story.ProjectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		beforeTickets, err := store.ListTicketsByProject(r.Context(), db, story.ProjectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		beforeIDs := make(map[string]struct{}, len(beforeTickets))
		for _, ticket := range beforeTickets {
			beforeIDs[ticket.ID] = struct{}{}
		}

		if err := runStoryBreakdownViaTicketCLI(db, project, story); err != nil {
			var analysis storyAnalysisResult
			prompt := fmt.Sprintf(
				"Story title: %s\nStory description: %s\nGenerate JSON shape {\"epics\":[{\"title\":\"...\",\"description\":\"...\",\"tasks\":[{\"title\":\"...\",\"description\":\"...\"}]}]} with 1-4 epics and 2-5 tasks per epic.",
				story.Title,
				story.Description,
			)
			if err := runRoleJSONAnalysis(db, "StoryReview", prompt, &analysis); err != nil || len(analysis.Epics) == 0 {
				analysis = fallbackStoryAnalysis(story)
			}
			for _, epicSpec := range analysis.Epics {
				epicTitle := strings.TrimSpace(epicSpec.Title)
				if epicTitle == "" {
					continue
				}
				epic, err := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
					ProjectID:   story.ProjectID,
					Type:        "epic",
					Title:       epicTitle,
					Description: strings.TrimSpace(epicSpec.Description),
					Author:      user.Username,
					CreatedBy:   user.ID,
					State:       store.StateIdle,
				})
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				for _, taskSpec := range epicSpec.Tasks {
					taskTitle := strings.TrimSpace(taskSpec.Title)
					if taskTitle == "" {
						continue
					}
					parentID := epic.ID
					_, err := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
						ProjectID:   story.ProjectID,
						ParentID:    &parentID,
						Type:        "task",
						Title:       taskTitle,
						Description: strings.TrimSpace(taskSpec.Description),
						Author:      user.Username,
						CreatedBy:   user.ID,
						State:       store.StateIdle,
					})
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
				}
			}
		}

		afterTickets, err := store.ListTicketsByProject(r.Context(), db, story.ProjectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		createdEpics := 0
		createdTasks := 0
		for _, ticket := range afterTickets {
			if _, existed := beforeIDs[ticket.ID]; existed {
				continue
			}
			_ = store.LinkStoryToTicket(r.Context(), db, story.ID, ticket.ID)
			notify("ticket_created", ticket.ProjectID, ticket.ID)
			switch strings.ToLower(strings.TrimSpace(ticket.Type)) {
			case "epic":
				createdEpics++
			case "task":
				createdTasks++
			}
		}
		updatedStory, err := store.UpdateStoryStatus(r.Context(), db, story.ID, "ready_for_review")
		if err == nil {
			story = updatedStory
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"story":         story,
			"created_epics": createdEpics,
			"created_tasks": createdTasks,
		})
	})

	handleTicketClaim := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		var claimRequest ticketClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&claimRequest); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		ticketID := claimRequest.TicketID
		ticketRef := strings.TrimSpace(claimRequest.TicketRef)
		if claimRequest.ProjectID != 0 {
			role, err := projectRoleForUser(r.Context(), db, claimRequest.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
		}
		ticket, status, err := store.RequestTicket(r.Context(), db, store.TicketRequestParams{
			ProjectID: claimRequest.ProjectID,
			TicketID:  ticketID,
			TicketRef: ticketRef,
			Username:  user.Username,
			UserID:    user.ID,
			DryRun:    claimRequest.DryRun,
		})
		if err != nil {
			if errors.Is(err, store.ErrTicketNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		payload := map[string]any{"status": status}
		if status == "ASSIGNED" || status == "AVAILABLE" {
			payload["ticket"] = ticket
			ctx := store.EnrichTicketContext(r.Context(), db, ticket)
			payload["project"] = ctx.Project
			payload["parents"] = ctx.Parents
			payload["workflow"] = ctx.Workflow
			payload["role"] = ctx.Role
			if status == "ASSIGNED" {
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
			}
		}
		writeJSON(w, http.StatusOK, payload)
	}
	mux.HandleFunc("/api/tickets/claim", handleTicketClaim)

	handleTicketByRef := func(pathPrefix string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, err := requireUser(db, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			trimmed := strings.TrimPrefix(r.URL.Path, pathPrefix)
			parts := strings.Split(trimmed, "/")
			ticketRef, err := store.GetTicketByRef(r.Context(), db, parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, "ticket not found")
				return
			}
			id := ticketRef.ID
			role, err := projectRoleForUser(r.Context(), db, ticketRef.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				events, err := store.ListHistoryEvents(r.Context(), db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, events)
				return
			}

			if len(parts) == 2 && parts[1] == "health" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var healthPayload ticketHealthRequest
				if err := json.NewDecoder(r.Body).Decode(&healthPayload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				ticket, err := store.SetTicketHealth(r.Context(), db, id, healthPayload.Score)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, "ticket not found")
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}

			if (len(parts) == 2 && parts[1] == "labels") || (len(parts) == 3 && parts[1] == "labels") {
				if len(parts) == 3 {
					// /api/tickets/<ref>/labels/<label_id>
					var labelID int64
					if _, err := fmt.Sscan(parts[2], &labelID); err != nil {
						writeError(w, http.StatusBadRequest, "invalid label id")
						return
					}
					if r.Method == http.MethodDelete {
						if !canWriteProject(role) {
							writeAuthError(w, store.ErrForbidden)
							return
						}
						if err := store.RemoveTicketLabel(r.Context(), db, id, labelID); err != nil {
							writeError(w, http.StatusBadRequest, err.Error())
							return
						}
						writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
						return
					}
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				switch r.Method {
				case http.MethodGet:
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					labels, err := store.ListTicketLabels(r.Context(), db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, labels)
				case http.MethodPost:
					if !canWriteProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var req struct {
						LabelID int64 `json:"label_id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					if err := store.AddTicketLabel(r.Context(), db, id, req.LabelID); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}

			if (len(parts) == 2 && parts[1] == "time") || (len(parts) == 3 && parts[1] == "time") {
				if len(parts) == 3 && parts[2] == "total" {
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					total, err := store.TotalTimeForTicket(r.Context(), db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]int{"total": total})
					return
				}
				switch r.Method {
				case http.MethodGet:
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					entries, err := store.ListTimeEntries(r.Context(), db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, entries)
				case http.MethodPost:
					if !canWriteProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var req struct {
						Minutes int    `json:"minutes"`
						Note    string `json:"note"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					entry, err := store.LogTime(r.Context(), db, id, user.ID, req.Minutes, req.Note)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeJSON(w, http.StatusCreated, entry)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}

			if len(parts) == 2 && parts[1] == "comments" {
				switch r.Method {
				case http.MethodGet:
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					comments, err := store.ListComments(r.Context(), db, id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, err.Error())
						return
					}
					writeJSON(w, http.StatusOK, comments)
				case http.MethodPost:
					if !canWriteProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var commentPayload commentRequest
					if err := json.NewDecoder(r.Body).Decode(&commentPayload); err != nil {
						writeError(w, http.StatusBadRequest, "invalid json body")
						return
					}
					comment, err := store.AddComment(r.Context(), db, id, user.ID, commentPayload.Comment)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					ticket, err := store.GetTicket(r.Context(), db, id)
					if err == nil {
						_ = store.AddHistoryEvent(r.Context(), db, ticket.ProjectID, id, "comment_added", map[string]any{
							"key":        ticket.ID,
							"comment_id": comment.ID,
						}, user.ID)
						notify("ticket_updated", ticket.ProjectID, ticket.ID)
					}
					writeJSON(w, http.StatusCreated, comment)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}

			if len(parts) == 2 && parts[1] == "dependencies" && r.Method == http.MethodGet {
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				dependencies, err := store.ListDependencies(r.Context(), db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, dependencies)
				return
			}

			if len(parts) == 2 && parts[1] == "clone" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var msg messageRequest
				json.NewDecoder(r.Body).Decode(&msg)
				cloned, err := store.CloneTicket(r.Context(), db, id, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if msg.Message != "" {
					store.AddComment(r.Context(), db, cloned.ID, user.ID, msg.Message)
				}
				notify("ticket_created", cloned.ProjectID, cloned.ID)
				writeJSON(w, http.StatusCreated, cloned)
				return
			}
			if len(parts) == 2 && parts[1] == "close" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var msg messageRequest
				json.NewDecoder(r.Body).Decode(&msg)
				// Add comment before close — AddComment rejects closed tickets.
				if msg.Message != "" {
					store.AddComment(r.Context(), db, id, user.ID, msg.Message)
				}
				ticket, err := store.SetTicketOpen(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "open" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var msg messageRequest
				json.NewDecoder(r.Body).Decode(&msg)
				ticket, err := store.SetTicketOpen(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if msg.Message != "" {
					store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message)
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "archive" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var msg messageRequest
				json.NewDecoder(r.Body).Decode(&msg)
				// Add comment before archive — AddComment rejects archived tickets.
				if msg.Message != "" {
					store.AddComment(r.Context(), db, id, user.ID, msg.Message)
				}
				ticket, err := store.SetTicketArchived(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "unarchive" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var msg messageRequest
				json.NewDecoder(r.Body).Decode(&msg)
				ticket, err := store.SetTicketArchived(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if msg.Message != "" {
					store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message)
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "ready" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var msg messageRequest
				json.NewDecoder(r.Body).Decode(&msg)
				ticket, err := store.SetTicketReady(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if msg.Message != "" {
					store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message)
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "notready" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var msg messageRequest
				json.NewDecoder(r.Body).Decode(&msg)
				ticket, err := store.SetTicketReady(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if msg.Message != "" {
					store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message)
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "workflow" {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				switch r.Method {
				case http.MethodPost:
					var payload struct {
						WorkflowID int64 `json:"workflow_id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.WorkflowID == 0 {
						writeError(w, http.StatusBadRequest, "workflow_id is required")
						return
					}
					ticket, err := store.SetTicketWorkflow(r.Context(), db, id, payload.WorkflowID)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					notify("ticket_updated", ticket.ProjectID, ticket.ID)
					writeJSON(w, http.StatusOK, ticket)
				case http.MethodDelete:
					ticket, err := store.UnsetTicketWorkflow(r.Context(), db, id)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					notify("ticket_updated", ticket.ProjectID, ticket.ID)
					writeJSON(w, http.StatusOK, ticket)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}
			if len(parts) == 2 && parts[1] == "analyse" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				epic, err := store.GetTicket(r.Context(), db, id)
				if err != nil {
					writeError(w, http.StatusNotFound, "ticket not found")
					return
				}
				if epic.Type != "epic" {
					writeError(w, http.StatusBadRequest, "analyse is only supported for epics")
					return
				}
				storyID, ok, err := store.StoryIDForTicket(r.Context(), db, epic.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				if !ok {
					writeError(w, http.StatusBadRequest, "epic is not linked to a story")
					return
				}
				story, err := store.GetStory(r.Context(), db, storyID)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}

				var analysis epicAnalysisResult
				prompt := fmt.Sprintf(
					"Story title: %s\nStory description: %s\nEpic title: %s\nEpic description: %s\nGenerate JSON shape {\"tickets\":[{\"title\":\"...\",\"description\":\"...\"}]} with 2-6 implementation tickets.",
					story.Title, story.Description, epic.Title, epic.Description,
				)
				if err := runRoleJSONAnalysis(db, "EpicReview", prompt, &analysis); err != nil || len(analysis.Tickets) == 0 {
					analysis = fallbackEpicAnalysis(epic)
				}

				created := 0
				for _, taskSpec := range analysis.Tickets {
					taskTitle := strings.TrimSpace(taskSpec.Title)
					if taskTitle == "" {
						continue
					}
					parentID := epic.ID
					task, err := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
						ProjectID:   epic.ProjectID,
						ParentID:    &parentID,
						Type:        "task",
						Title:       taskTitle,
						Description: strings.TrimSpace(taskSpec.Description),
						Author:      user.Username,
						CreatedBy:   user.ID,
						State:       store.StateIdle,
					})
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					_ = store.LinkStoryToTicket(r.Context(), db, story.ID, task.ID)
					notify("ticket_created", task.ProjectID, task.ID)
					created++
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"epic_id":         epic.ID,
					"story_id":        story.ID,
					"created_tickets": created,
				})
				return
			}

			switch r.Method {
			case http.MethodGet:
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				ticket, err := store.GetTicket(r.Context(), db, id)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, ticket)
			case http.MethodPut:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var ticketPayload ticketRequest
				if err := json.NewDecoder(r.Body).Decode(&ticketPayload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				currentTicket, err := store.GetTicket(r.Context(), db, id)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				ticketPayload = autoProgressTicketLifecycle(ticketPayload, currentTicket, user.Username)
				stage, state, _ := resolveLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
				ticket, err := store.UpdateTicket(r.Context(), db, id, store.TicketUpdateParams{
					Title:              ticketPayload.Title,
					Description:        ticketPayload.Description,
					AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
					GitRepository:      ticketPayload.GitRepository,
					GitBranch:          ticketPayload.GitBranch,
					ParentID:           ticketPayload.ParentID,
					Assignee:           ticketPayload.Assignee,
					Stage:              stage,
					State:              state,
					Priority:           ticketPayload.Priority,
					Order:              ticketPayload.Order,
					EstimateEffort:     ticketPayload.EstimateEffort,
					EstimateComplete:   ticketPayload.EstimateComplete,
					UpdatedBy:          user.ID,
					ActorUsername:      user.Username,
					ActorRole:          user.Role,
				})
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					if errors.Is(err, store.ErrAdminRequired) || errors.Is(err, store.ErrForbidden) {
						writeAuthError(w, err)
						return
					}
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if ticketPayload.Message != "" {
					store.AddComment(r.Context(), db, ticket.ID, user.ID, ticketPayload.Message)
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
			case http.MethodDelete:
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if err := store.DeleteTicket(r.Context(), db, id); err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					if errors.Is(err, store.ErrTicketHasChildren) {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				notify("ticket_deleted", ticketRef.ProjectID, id)
				writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		}
	}
	mux.HandleFunc("/api/tickets/", handleTicketByRef("/api/tickets/"))

	mux.HandleFunc("/api/labels/", func(w http.ResponseWriter, r *http.Request) {
		_, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/api/labels/")
		var id int64
		if _, err := fmt.Sscan(idStr, &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid label id")
			return
		}
		if err := store.DeleteLabel(r.Context(), db, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("/api/time/", func(w http.ResponseWriter, r *http.Request) {
		_, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/api/time/")
		var id int64
		if _, err := fmt.Sscan(idStr, &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid time entry id")
			return
		}
		if err := store.DeleteTimeEntry(r.Context(), db, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("/api/dependencies", func(w http.ResponseWriter, r *http.Request) {
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var dependencyPayload dependencyRequest
			if err := json.NewDecoder(r.Body).Decode(&dependencyPayload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, dependencyPayload.ProjectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			dependency, err := store.AddDependency(r.Context(), db, dependencyPayload.ProjectID, dependencyPayload.TicketID, dependencyPayload.DependsOn, user.ID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("ticket_updated", dependencyPayload.ProjectID, dependencyPayload.TicketID)
			writeJSON(w, http.StatusCreated, dependency)
		case http.MethodDelete:
			var projectID int64
			if _, err := fmt.Sscan(strings.TrimSpace(r.URL.Query().Get("project_id")), &projectID); err != nil {
				writeError(w, http.StatusBadRequest, "project_id must be numeric")
				return
			}
			ticketID := strings.TrimSpace(r.URL.Query().Get("ticket_id"))
			if ticketID == "" {
				writeError(w, http.StatusBadRequest, "ticket_id is required")
				return
			}
			dependsOn := strings.TrimSpace(r.URL.Query().Get("depends_on"))
			if dependsOn == "" {
				writeError(w, http.StatusBadRequest, "depends_on is required")
				return
			}
			role, err := projectRoleForUser(r.Context(), db, projectID, user)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !canWriteProject(role) {
				writeAuthError(w, store.ErrForbidden)
				return
			}
			if err := store.DeleteDependency(r.Context(), db, projectID, ticketID, dependsOn); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "dependency not found")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			notify("ticket_updated", projectID, ticketID)
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
