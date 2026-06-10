package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/ticketmarkdown"
)

func (r *router) registerTicketHandlers() {
	db := r.db
	mux := r.mux
	notify := r.notify

	mux.HandleFunc("/api/tickets/import-markdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		var payload ticketMarkdownImportRequest
		if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		doc, parseErr := ticketmarkdown.Parse(payload.Content)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, parseErr.Error())
			return
		}
		currentTicket, err := store.GetTicket(r.Context(), db, doc.ID)
		if err != nil {
			if errors.Is(err, store.ErrTicketNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		role, err := projectRoleForUser(r.Context(), db, currentTicket.ProjectID, user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !canWriteProject(role) {
			writeAuthError(w, store.ErrForbidden)
			return
		}
		ticket, err := store.UpdateTicket(r.Context(), db, currentTicket.ID, store.TicketUpdateParams{
			Title:              doc.Title,
			Description:        doc.Description,
			AcceptanceCriteria: doc.AcceptanceCriteria,
			DORMap:             currentTicket.DORMap,
			DODMap:             currentTicket.DODMap,
			ACMap:              currentTicket.ACMap,
			GitRepository:      currentTicket.GitRepository,
			GitBranch:          currentTicket.GitBranch,
			ParentID:           currentTicket.ParentID,
			Assignee:           currentTicket.Assignee,
			Priority:           currentTicket.Priority,
			Order:              currentTicket.Order,
			EstimateEffort:     currentTicket.EstimateEffort,
			EstimateComplete:   currentTicket.EstimateComplete,
			Type:               doc.Type,
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
			writeStoreError(w, err)
			return
		}
		notify("ticket_updated", ticket.ProjectID, ticket.ID)
		writeJSON(w, http.StatusOK, ticket)
	})

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
		err = json.NewDecoder(r.Body).Decode(&ticketPayload)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		state, err := resolveCreateLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		project, _, err := resolveProjectForWriteRequest(r.Context(), db, r, user, ticketPayload.ProjectID)
		if err != nil {
			if errors.Is(err, store.ErrForbidden) || errors.Is(err, store.ErrUnauthorized) {
				writeAuthError(w, err)
			} else {
				writeStoreError(w, err)
			}
			return
		}
		ticket, err := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
			ProjectID:          project.ID,
			ParentID:           ticketPayload.ParentID,
			Type:               ticketPayload.Type,
			Title:              ticketPayload.Title,
			Description:        ticketPayload.Description,
			AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
			DORMap:             ticketPayload.DORMap,
			DODMap:             ticketPayload.DODMap,
			ACMap:              ticketPayload.ACMap,
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
			writeStoreError(w, err)
			return
		}
		if ticketPayload.Message != "" {
			if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, ticketPayload.Message); commentErr != nil {
				log.Printf("warning: add comment on ticket create %s: %v", ticket.ID, commentErr)
			}
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
			err = json.NewDecoder(r.Body).Decode(&payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			project, _, err := resolveProjectForWriteRequest(r.Context(), db, r, user, payload.ProjectID)
			if err != nil {
				if errors.Is(err, store.ErrForbidden) || errors.Is(err, store.ErrUnauthorized) {
					writeAuthError(w, err)
				} else {
					writeStoreError(w, err)
				}
				return
			}
			story, err := store.CreateStoryWithParams(r.Context(), db, store.StoryCreateParams{
				ID:          payload.ID,
				ProjectID:   project.ID,
				Title:       payload.Title,
				Description: payload.Description,
				CreatedBy:   user.ID,
			})
			if err != nil {
				writeStoreError(w, err)
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
		if _, scanErr := fmt.Sscan(parts[0], &storyID); scanErr != nil {
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
			err = store.DeleteStory(r.Context(), db, story.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if len(parts) == 1 && r.Method == http.MethodPut {
			var payload storyRequest
			err = json.NewDecoder(r.Body).Decode(&payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			updated, updateErr := store.UpdateStory(r.Context(), db, story.ID, payload.Title, payload.Description)
			if updateErr != nil {
				if errors.Is(updateErr, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "story not found")
					return
				}
				writeStoreError(w, updateErr)
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

		if err = runStoryBreakdownViaTicketCLI(db, project, story); err != nil {
			var analysis storyAnalysisResult
			prompt := fmt.Sprintf(
				"Story title: %s\nStory description: %s\nGenerate JSON shape {\"epics\":[{\"title\":\"...\",\"description\":\"...\",\"tasks\":[{\"title\":\"...\",\"description\":\"...\"}]}]} with 1-4 epics and 2-5 tasks per epic.",
				story.Title,
				story.Description,
			)
			if err = runRoleJSONAnalysis(db, "StoryReview", prompt, &analysis); err != nil || len(analysis.Epics) == 0 {
				analysis = fallbackStoryAnalysis(story)
			}
			for _, epicSpec := range analysis.Epics {
				epicTitle := strings.TrimSpace(epicSpec.Title)
				if epicTitle == "" {
					continue
				}
				epic, createErr := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
					ProjectID:   story.ProjectID,
					Type:        "epic",
					Title:       epicTitle,
					Description: strings.TrimSpace(epicSpec.Description),
					Author:      user.Username,
					CreatedBy:   user.ID,
					State:       store.StateIdle,
				})
				if createErr != nil {
					writeStoreError(w, createErr)
					return
				}
				for _, taskSpec := range epicSpec.Tasks {
					taskTitle := strings.TrimSpace(taskSpec.Title)
					if taskTitle == "" {
						continue
					}
					parentID := epic.ID
					_, createErr := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
						ProjectID:   story.ProjectID,
						ParentID:    &parentID,
						Type:        "task",
						Title:       taskTitle,
						Description: strings.TrimSpace(taskSpec.Description),
						Author:      user.Username,
						CreatedBy:   user.ID,
						State:       store.StateIdle,
					})
					if createErr != nil {
						writeStoreError(w, createErr)
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
		createdTicketIDs := make([]string, 0)
		createdTickets := make([]store.Ticket, 0)
		for _, ticket := range afterTickets {
			if _, existed := beforeIDs[ticket.ID]; existed {
				continue
			}
			createdTicketIDs = append(createdTicketIDs, ticket.ID)
			createdTickets = append(createdTickets, ticket)
			switch strings.ToLower(strings.TrimSpace(ticket.Type)) {
			case "epic":
				createdEpics++
			case "task":
				createdTasks++
			}
		}
		if linkErr := store.LinkStoryToTickets(r.Context(), db, story.ID, createdTicketIDs); linkErr != nil {
			writeError(w, http.StatusInternalServerError, linkErr.Error())
			return
		}
		for _, ticket := range createdTickets {
			notify("ticket_created", ticket.ProjectID, ticket.ID)
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
		err = json.NewDecoder(r.Body).Decode(&claimRequest)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		ticketID := claimRequest.TicketID
		ticketRef := strings.TrimSpace(claimRequest.TicketRef)
		if claimRequest.ProjectID != 0 {
			role, roleErr := projectRoleForUser(r.Context(), db, claimRequest.ProjectID, user)
			if roleErr != nil {
				writeError(w, http.StatusInternalServerError, roleErr.Error())
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
			writeStoreError(w, err)
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
				limit, err := queryInt(r, "limit", 0)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				offset, err := queryInt(r, "offset", 0)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				events, err := store.ListHistoryEvents(r.Context(), db, id, limit, offset)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, events)
				return
			}
			if len(parts) == 2 && parts[1] == "phase-signoffs" && r.Method == http.MethodGet {
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				signoffs, err := store.ListTicketPhaseSignoffs(r.Context(), db, id)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, signoffs)
				return
			}
			if len(parts) == 3 && parts[1] == "phase-signoffs" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				phase := strings.TrimSpace(parts[2])
				var payload phaseSignoffRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				signoff, err := store.SetTicketPhaseSignoff(r.Context(), db, id, phase, payload.Approved, user.ID, payload.Note)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if err := store.AddHistoryEvent(r.Context(), db, ticketRef.ProjectID, ticketRef.ID, "ticket_phase_signoff_updated", map[string]any{
					"phase":    signoff.Phase,
					"approved": signoff.Approved,
					"note":     signoff.Note,
					"who":      user.Username,
				}, user.ID); err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticketRef.ProjectID, ticketRef.ID)
				writeJSON(w, http.StatusOK, signoff)
				return
			}
			if len(parts) == 3 && parts[1] == "refinement" && parts[2] == "approve" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				approved, err := store.ApproveRefinement(r.Context(), db, id, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", approved.ProjectID, approved.ID)
				writeJSON(w, http.StatusOK, approved)
				return
			}
			if len(parts) == 3 && parts[1] == "children" && parts[2] == "reorder" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload ticketChildrenReorderRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				children, err := store.ReorderChildTickets(r.Context(), db, id, payload.Order, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticketRef.ProjectID, ticketRef.ID)
				writeJSON(w, http.StatusOK, children)
				return
			}
			if len(parts) == 2 && parts[1] == "context" {
				switch r.Method {
				case http.MethodGet:
					if !canReadProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					edges, err := store.ListContextEdgesForNode(r.Context(), db, store.ContextNodeTicket, id)
					if err != nil {
						writeStoreError(w, err)
						return
					}
					writeJSON(w, http.StatusOK, edges)
					return
				case http.MethodPost:
					if !canWriteProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var payload ticketContextRequest
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						writeError(w, http.StatusBadRequest, "invalid json body")
						return
					}
					edge, err := store.AddContextEdge(r.Context(), db, ticketRef.ProjectID, store.ContextNodeTicket, id, payload.TargetType, payload.TargetID, payload.Relation, payload.Title, user.ID)
					if err != nil {
						writeStoreError(w, err)
						return
					}
					if err := store.AddHistoryEvent(r.Context(), db, ticketRef.ProjectID, ticketRef.ID, "ticket_context_added", map[string]any{
						"edge_id":     edge.ID,
						"target_type": edge.TargetType,
						"target_id":   edge.TargetID,
						"relation":    edge.Relation,
						"actor":       user.Username,
					}, user.ID); err != nil {
						writeStoreError(w, err)
						return
					}
					notify("ticket_updated", ticketRef.ProjectID, ticketRef.ID)
					writeJSON(w, http.StatusCreated, edge)
					return
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
			}
			if len(parts) == 3 && parts[1] == "context" && r.Method == http.MethodDelete {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var edgeID int64
				if _, err := fmt.Sscan(parts[2], &edgeID); err != nil {
					writeError(w, http.StatusBadRequest, "invalid context edge id")
					return
				}
				if err := store.RemoveContextEdge(r.Context(), db, ticketRef.ProjectID, edgeID); err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticketRef.ProjectID, ticketRef.ID)
				writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
				return
			}
			if len(parts) == 2 && parts[1] == "inbox" && r.Method == http.MethodGet {
				if !canViewInterventions(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				status := strings.TrimSpace(r.URL.Query().Get("status"))
				entries, err := store.ListInboxEntriesByTicket(r.Context(), db, id, status)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, entries)
				return
			}
			if len(parts) == 3 && parts[1] == "inbox" && parts[2] == "escalate" && r.Method == http.MethodPost {
				if !canManageInterventions(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload inboxEscalateRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				ticket, err := store.GetTicket(r.Context(), db, id)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				entry, err := store.CreateFailureEscalationInboxEntry(r.Context(), db, ticket, payload.Message, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if err := store.AddHistoryEvent(r.Context(), db, ticket.ProjectID, ticket.ID, "ticket_escalated_to_inbox", map[string]any{
					"inbox_id":         entry.ID,
					"recommendations":  entry.Recommendations,
					"message":          entry.Message,
					"escalated_by":     user.Username,
					"escalation_state": entry.Status,
				}, user.ID); err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusCreated, entry)
				return
			}
			if len(parts) == 4 && parts[1] == "inbox" && parts[3] == "decide" && r.Method == http.MethodPost {
				if !canManageInterventions(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var inboxID int64
				if _, err := fmt.Sscan(parts[2], &inboxID); err != nil {
					writeError(w, http.StatusBadRequest, "invalid inbox id")
					return
				}
				var payload inboxDecisionRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				entry, err := store.DecideInboxEntry(r.Context(), db, inboxID, payload.Decision, payload.Message, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if entry.TicketID != id {
					writeError(w, http.StatusBadRequest, "inbox entry does not belong to this ticket")
					return
				}
				if err := store.AddHistoryEvent(r.Context(), db, ticketRef.ProjectID, ticketRef.ID, "ticket_inbox_decision_recorded", map[string]any{
					"inbox_id":   entry.ID,
					"decision":   entry.Decision,
					"message":    entry.Message,
					"decided_by": user.Username,
				}, user.ID); err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticketRef.ProjectID, ticketRef.ID)
				writeJSON(w, http.StatusOK, entry)
				return
			}
			if len(parts) == 2 && parts[1] == "execution-packet" && r.Method == http.MethodGet {
				if !canReadProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				packet, err := store.BuildExecutionPacket(r.Context(), db, id)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, packet)
				return
			}
			if len(parts) == 2 && parts[1] == "work-items" && r.Method == http.MethodGet {
				if !canViewWorkItems(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				limit, err := queryInt(r, "limit", 0)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				offset, err := queryInt(r, "offset", 0)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				items, err := store.ListWorkItemsByTicketWithParams(r.Context(), db, id, store.WorkItemListParams{
					Status:       r.URL.Query().Get("status"),
					AssigneeType: r.URL.Query().Get("assignee_type"),
					Limit:        limit,
					Offset:       offset,
				})
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, items)
				return
			}
			if len(parts) == 4 && parts[1] == "work-items" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				workItemID := strings.TrimSpace(parts[2])
				if workItemID == "" {
					writeError(w, http.StatusBadRequest, "work_item_id is required")
					return
				}
				action := strings.TrimSpace(strings.ToLower(parts[3]))
				var payload struct {
					Assignee  string `json:"assignee"`
					Message   string `json:"message"`
					CommitRef string `json:"commit_ref"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				var (
					item      store.WorkItem
					actionErr error
				)
				switch action {
				case "reassign":
					item, actionErr = store.ReassignWorkItem(r.Context(), db, id, workItemID, payload.Assignee, user.Username, user.ID)
				case "cancel":
					item, actionErr = store.CancelWorkItem(r.Context(), db, id, workItemID, payload.Message, user.Username, user.ID)
				case "retry":
					item, actionErr = store.RetryWorkItem(r.Context(), db, id, workItemID, payload.Assignee, user.Username, user.ID)
				case "feedback":
					item, actionErr = store.AddWorkItemFeedback(r.Context(), db, id, workItemID, payload.Message, payload.CommitRef, user.Username, user.ID)
				default:
					writeError(w, http.StatusBadRequest, "invalid work-item action")
					return
				}
				if actionErr != nil {
					writeStoreError(w, actionErr)
					return
				}
				if err := store.AddHistoryEvent(r.Context(), db, ticketRef.ProjectID, ticketRef.ID, "ticket_work_item_"+action, map[string]any{
					"work_item_id": item.ID,
					"actor":        user.Username,
					"message":      payload.Message,
					"commit_ref":   payload.CommitRef,
				}, user.ID); err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticketRef.ProjectID, ticketRef.ID)
				writeJSON(w, http.StatusOK, item)
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
					writeStoreError(w, err)
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
							writeStoreError(w, err)
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
					if !canCommentProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var req struct {
						LabelID int64 `json:"label_id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeStoreError(w, err)
						return
					}
					if err := store.AddTicketLabel(r.Context(), db, id, req.LabelID); err != nil {
						writeStoreError(w, err)
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
					if !canCommentProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var req struct {
						Minutes int    `json:"minutes"`
						Note    string `json:"note"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeStoreError(w, err)
						return
					}
					entry, err := store.LogTime(r.Context(), db, id, user.ID, req.Minutes, req.Note)
					if err != nil {
						writeStoreError(w, err)
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
					if !canCommentProject(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var commentPayload commentRequest
					if err := json.NewDecoder(r.Body).Decode(&commentPayload); err != nil {
						writeError(w, http.StatusBadRequest, "invalid json body")
						return
					}
					comment, err := store.AddComment(r.Context(), db, id, user.ID, commentPayload.Comment) // #nosec G104 -- best-effort comment; main operation already succeeded
					if err != nil {
						writeStoreError(w, err)
						return
					}
					ticket, err := store.GetTicket(r.Context(), db, id)
					if err == nil {
						if err := store.AddHistoryEvent(r.Context(), db, ticket.ProjectID, id, "comment_added", map[string]any{
							"key":        ticket.ID,
							"comment_id": comment.ID,
						}, user.ID); err != nil {
							log.Printf("warning: add history event for ticket %s comment %d: %v", ticket.ID, comment.ID, err)
						}
						notify("ticket_updated", ticket.ProjectID, ticket.ID)
						// A human reply into a refine-stage conversation wakes a refiner
						// immediately, so the dialogue feels near-real-time.
						if user.UserType != "agent" {
							maybeTriggerRefinement(db, ticket)
						}
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
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode clone message for ticket %s: %v", id, decodeErr)
				}
				cloned, err := store.CloneTicket(r.Context(), db, id, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
					return
				}
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, cloned.ID, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment after clone ticket %s: %v", cloned.ID, commentErr)
					}
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
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode close message for ticket %s: %v", id, decodeErr)
				}
				// Add comment before close — AddComment rejects closed tickets.
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, id, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment before close ticket %s: %v", id, commentErr)
					}
				}
				ticket, err := store.SetTicketComplete(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
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
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode open message for ticket %s: %v", id, decodeErr)
				}
				ticket, err := store.SetTicketComplete(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
					return
				}
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment after open ticket %s: %v", ticket.ID, commentErr)
					}
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
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode archive message for ticket %s: %v", id, decodeErr)
				}
				// Add comment before archive — AddComment rejects archived tickets.
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, id, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment before archive ticket %s: %v", id, commentErr)
					}
				}
				ticket, err := store.SetTicketArchived(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
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
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode unarchive message for ticket %s: %v", id, decodeErr)
				}
				ticket, err := store.SetTicketArchived(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
					return
				}
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment after unarchive ticket %s: %v", ticket.ID, commentErr)
					}
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
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode ready message for ticket %s: %v", id, decodeErr)
				}
				ticket, err := store.SetTicketDraft(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
					return
				}
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment after ready ticket %s: %v", ticket.ID, commentErr)
					}
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
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode notready message for ticket %s: %v", id, decodeErr)
				}
				ticket, err := store.SetTicketDraft(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					if errors.Is(err, store.ErrTicketNotFound) {
						writeError(w, http.StatusNotFound, err.Error())
						return
					}
					writeStoreError(w, err)
					return
				}
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment after notready ticket %s: %v", ticket.ID, commentErr)
					}
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "complete" && r.Method == http.MethodPost {
				var msg messageRequest
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode complete message for ticket %s: %v", id, decodeErr)
				}
				ticket, err := store.SetTicketComplete(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment after complete ticket %s: %v", ticket.ID, commentErr)
					}
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "reopen" && r.Method == http.MethodPost {
				var msg messageRequest
				if decodeErr := json.NewDecoder(r.Body).Decode(&msg); decodeErr != nil {
					log.Printf("warning: decode reopen message for ticket %s: %v", id, decodeErr)
				}
				ticket, err := store.SetTicketComplete(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if msg.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, msg.Message); commentErr != nil {
						log.Printf("warning: add comment after reopen ticket %s: %v", ticket.ID, commentErr)
					}
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "draft" && r.Method == http.MethodPost {
				ticket, err := store.SetTicketDraft(r.Context(), db, id, true, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "undraft" && r.Method == http.MethodPost {
				ticket, err := store.SetTicketDraft(r.Context(), db, id, false, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "next" && r.Method == http.MethodPost {
				ticket, err := store.NextTicket(r.Context(), db, id, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "previous" && r.Method == http.MethodPost {
				ticket, err := store.PreviousTicket(r.Context(), db, id, user.Username, user.ID)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			if len(parts) == 2 && parts[1] == "intervention-state" {
				if r.Method == http.MethodGet {
					if !canViewInterventions(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					state, err := store.GetInterventionState(r.Context(), db, id)
					if err != nil {
						writeStoreError(w, err)
						return
					}
					writeJSON(w, http.StatusOK, state)
					return
				}
				if r.Method == http.MethodPost {
					if !canManageInterventions(role) {
						writeAuthError(w, store.ErrForbidden)
						return
					}
					var payload interventionStateRequest
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						writeError(w, http.StatusBadRequest, "invalid json body")
						return
					}
					state, err := store.SetInterventionState(r.Context(), db, id, payload.State, user.ID, user.ID)
					if err != nil {
						writeStoreError(w, err)
						return
					}
					if err := store.AddHistoryEvent(r.Context(), db, ticketRef.ProjectID, ticketRef.ID, "ticket_intervention_state_updated", map[string]any{
						"state": state.State,
						"owner": state.OwnerName,
						"who":   user.Username,
					}, user.ID); err != nil {
						writeStoreError(w, err)
						return
					}
					notify("ticket_updated", ticketRef.ProjectID, ticketRef.ID)
					writeJSON(w, http.StatusOK, state)
					return
				}
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if len(parts) == 2 && parts[1] == "intervene" && r.Method == http.MethodPost {
				if !canManageInterventions(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				var payload interventionRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				outcome := strings.TrimSpace(strings.ToLower(payload.Outcome))
				if outcome == "" {
					writeError(w, http.StatusBadRequest, "outcome is required")
					return
				}
				if strings.ToLower(strings.TrimSpace(ticketRef.State)) != store.StateFail {
					writeError(w, http.StatusConflict, "ticket must be in fail state to intervene")
					return
				}

				var (
					ticket   store.Ticket
					followUp *store.Ticket
					err      error
				)
				switch outcome {
				case "retry-role":
					ticket, err = store.UpdateTicket(r.Context(), db, id, store.TicketUpdateParams{
						Title:              ticketRef.Title,
						Description:        ticketRef.Description,
						AcceptanceCriteria: ticketRef.AcceptanceCriteria,
						DORMap:             ticketRef.DORMap,
						DODMap:             ticketRef.DODMap,
						ACMap:              ticketRef.ACMap,
						GitRepository:      ticketRef.GitRepository,
						GitBranch:          ticketRef.GitBranch,
						ParentID:           ticketRef.ParentID,
						Assignee:           ticketRef.Assignee,
						Stage:              ticketRef.Stage,
						State:              store.StateIdle,
						Priority:           ticketRef.Priority,
						Order:              ticketRef.Order,
						EstimateEffort:     ticketRef.EstimateEffort,
						EstimateComplete:   ticketRef.EstimateComplete,
						Type:               ticketRef.Type,
						UpdatedBy:          user.ID,
						ActorUsername:      user.Username,
						ActorRole:          user.Role,
					})
				case "retry-stage":
					ticket, err = store.PreviousTicket(r.Context(), db, id, user.Username, user.ID)
				case "split-work":
					followUpTitle := strings.TrimSpace("Follow-up: " + ticketRef.Title)
					if followUpTitle == "Follow-up:" {
						followUpTitle = "Follow-up work"
					}
					created, createErr := store.CreateTicket(r.Context(), db, store.TicketCreateParams{
						ProjectID:          ticketRef.ProjectID,
						Type:               "task",
						Title:              followUpTitle,
						Description:        strings.TrimSpace("Created from intervention on " + ticketRef.ID + ".\n\n" + payload.Message),
						AcceptanceCriteria: ticketRef.AcceptanceCriteria,
						DORMap:             ticketRef.DORMap,
						DODMap:             ticketRef.DODMap,
						ACMap:              ticketRef.ACMap,
						GitRepository:      ticketRef.GitRepository,
						GitBranch:          ticketRef.GitBranch,
						Priority:           ticketRef.Priority,
						EstimateEffort:     ticketRef.EstimateEffort,
						EstimateComplete:   "",
						Author:             user.Username,
						CreatedBy:          user.ID,
					})
					if createErr != nil {
						writeStoreError(w, createErr)
						return
					}
					followUp = &created
					ticket, err = store.UpdateTicket(r.Context(), db, id, store.TicketUpdateParams{
						Title:              ticketRef.Title,
						Description:        ticketRef.Description,
						AcceptanceCriteria: ticketRef.AcceptanceCriteria,
						DORMap:             ticketRef.DORMap,
						DODMap:             ticketRef.DODMap,
						ACMap:              ticketRef.ACMap,
						GitRepository:      ticketRef.GitRepository,
						GitBranch:          ticketRef.GitBranch,
						ParentID:           ticketRef.ParentID,
						Assignee:           ticketRef.Assignee,
						Stage:              ticketRef.Stage,
						State:              store.StateIdle,
						Priority:           ticketRef.Priority,
						Order:              ticketRef.Order,
						EstimateEffort:     ticketRef.EstimateEffort,
						EstimateComplete:   ticketRef.EstimateComplete,
						Type:               ticketRef.Type,
						UpdatedBy:          user.ID,
						ActorUsername:      user.Username,
						ActorRole:          user.Role,
					})
				case "cancel":
					ticket, err = store.SetTicketArchived(r.Context(), db, id, true, user.Username, user.ID)
				default:
					writeError(w, http.StatusBadRequest, "invalid outcome")
					return
				}
				if err != nil {
					writeStoreError(w, err)
					return
				}
				if payload.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, payload.Message); commentErr != nil {
						log.Printf("warning: add comment after intervene ticket %s: %v", ticket.ID, commentErr)
					}
				}
				historyPayload := map[string]any{
					"outcome": outcome,
					"who":     user.Username,
					"message": payload.Message,
				}
				if followUp != nil {
					historyPayload["follow_up_ticket_id"] = followUp.ID
					historyPayload["follow_up_ticket_key"] = followUp.ID
				}
				nextInterventionState := store.InterventionStateTriaged
				if outcome == "cancel" {
					nextInterventionState = store.InterventionStateWontFix
				}
				if _, setStateErr := store.SetInterventionState(r.Context(), db, ticket.ID, nextInterventionState, user.ID, user.ID); setStateErr != nil {
					writeStoreError(w, setStateErr)
					return
				}
				if err := store.AddHistoryEvent(r.Context(), db, ticket.ProjectID, ticket.ID, "ticket_intervention_decided", historyPayload, user.ID); err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				if followUp != nil {
					notify("ticket_created", followUp.ProjectID, followUp.ID)
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"ticket":         ticket,
					"follow_up":      followUp,
					"decision":       outcome,
					"intervention":   true,
					"decision_actor": user.Username,
				})
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
						writeStoreError(w, err)
						return
					}
					notify("ticket_updated", ticket.ProjectID, ticket.ID)
					writeJSON(w, http.StatusOK, ticket)
				case http.MethodDelete:
					ticket, err := store.UnsetTicketWorkflow(r.Context(), db, id)
					if err != nil {
						writeStoreError(w, err)
						return
					}
					notify("ticket_updated", ticket.ProjectID, ticket.ID)
					writeJSON(w, http.StatusOK, ticket)
				default:
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			}
			// PUT /api/tickets/{id}/release — add the feature (subtree) to a release
			// (release_id set) or remove it (release_id null).
			if len(parts) == 2 && parts[1] == "release" {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				if r.Method != http.MethodPut {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				var payload struct {
					ReleaseID *int `json:"release_id"`
				}
				if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				var actErr error
				if payload.ReleaseID == nil {
					actErr = store.RemoveFeatureFromRelease(r.Context(), db, id)
				} else {
					actErr = store.AssignFeatureToRelease(r.Context(), db, id, *payload.ReleaseID)
				}
				if actErr != nil {
					writeStoreError(w, actErr)
					return
				}
				ticket, err := store.GetTicket(r.Context(), db, id)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				notify("ticket_updated", ticket.ProjectID, ticket.ID)
				writeJSON(w, http.StatusOK, ticket)
				return
			}
			// POST /api/tickets/{id}/clone — deep-clone a feature subtree.
			if len(parts) == 2 && parts[1] == "clone" && r.Method == http.MethodPost {
				if !canWriteProject(role) {
					writeAuthError(w, store.ErrForbidden)
					return
				}
				cloned, cloneErr := store.CloneFeature(r.Context(), db, id, user.Username, user.ID)
				if cloneErr != nil {
					writeStoreError(w, cloneErr)
					return
				}
				notify("ticket_updated", cloned.ProjectID, cloned.ID)
				writeJSON(w, http.StatusCreated, cloned)
				return
			}
			if len(parts) == 2 && parts[1] == "prompt" && r.Method == http.MethodGet {
				ticket, err := store.GetTicket(r.Context(), db, id)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{
					"prompt": buildTicketWorkPrompt(r.Context(), db, ticket),
				})
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
					writeStoreError(w, err)
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
				createdTasks := make([]store.Ticket, 0, len(analysis.Tickets))
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
						writeStoreError(w, err)
						return
					}
					createdTasks = append(createdTasks, task)
					created++
				}
				createdIDs := make([]string, 0, len(createdTasks))
				for _, ticket := range createdTasks {
					createdIDs = append(createdIDs, ticket.ID)
				}
				if linkErr := store.LinkStoryToTickets(r.Context(), db, story.ID, createdIDs); linkErr != nil {
					writeError(w, http.StatusInternalServerError, linkErr.Error())
					return
				}
				for _, task := range createdTasks {
					notify("ticket_created", task.ProjectID, task.ID)
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
				hasChildren, err := ticketHasChildrenForAPI(r.Context(), db, id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				ticketPayload = autoProgressTicketLifecycle(ticketPayload, currentTicket, user.Username, hasChildren)
				stage, state, err := resolveLifecycleRequest(ticketPayload.Status, ticketPayload.Stage, ticketPayload.State)
				if err != nil {
					writeStoreError(w, err)
					return
				}
				ticket, err := store.UpdateTicket(r.Context(), db, id, store.TicketUpdateParams{
					Title:              ticketPayload.Title,
					Description:        ticketPayload.Description,
					AcceptanceCriteria: ticketPayload.AcceptanceCriteria,
					DORMap:             ticketPayload.DORMap,
					DODMap:             ticketPayload.DODMap,
					ACMap:              ticketPayload.ACMap,
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
					Type:               ticketPayload.Type,
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
					writeStoreError(w, err)
					return
				}
				if ticketPayload.Message != "" {
					if _, commentErr := store.AddComment(r.Context(), db, ticket.ID, user.ID, ticketPayload.Message); commentErr != nil {
						log.Printf("warning: add comment after ticket update %s: %v", ticket.ID, commentErr)
					}
				}
				if strings.EqualFold(strings.TrimSpace(ticket.State), store.StateFail) {
					message := strings.TrimSpace(ticketPayload.Message)
					if message == "" {
						message = "Outcome failed and requires human decision."
					}
					entry, entryErr := store.EnsureFailureEscalationInboxEntry(r.Context(), db, ticket, message, user.ID)
					if entryErr != nil {
						writeStoreError(w, entryErr)
						return
					}
					if err := store.AddHistoryEvent(r.Context(), db, ticket.ProjectID, ticket.ID, "ticket_escalated_to_inbox", map[string]any{
						"inbox_id":         entry.ID,
						"recommendations":  entry.Recommendations,
						"message":          entry.Message,
						"escalated_by":     user.Username,
						"escalation_state": entry.Status,
					}, user.ID); err != nil {
						writeStoreError(w, err)
						return
					}
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
						writeStoreError(w, err)
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

	// POST /api/tickets/{id}/mark-ready — human approves refiner recommendation.
	mux.HandleFunc("/api/tickets-action/mark-ready/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := requireUser(db, r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/tickets-action/mark-ready/")
		id = strings.Trim(id, "/")
		if id == "" {
			writeError(w, http.StatusBadRequest, "ticket id required")
			return
		}
		updated, err := store.MarkTicketReady(r.Context(), db, id, user.Username, user.ID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		notify("ticket_updated", updated.ProjectID, updated.ID)
		writeJSON(w, http.StatusOK, updated)
	})

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
			writeStoreError(w, err)
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
			writeStoreError(w, err)
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
			project, _, err := resolveProjectForDependencyRequest(r.Context(), db, r, user, dependencyPayload.ProjectID, dependencyPayload.TicketID, dependencyPayload.DependsOn)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			dependency, err := store.AddDependency(r.Context(), db, project.ID, dependencyPayload.TicketID, dependencyPayload.DependsOn, user.ID)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			notify("ticket_updated", project.ID, dependencyPayload.TicketID)
			writeJSON(w, http.StatusCreated, dependency)
		case http.MethodDelete:
			projectIDRaw := strings.TrimSpace(r.URL.Query().Get("project_id"))
			var projectID int64
			if projectIDRaw != "" {
				if _, err := fmt.Sscan(projectIDRaw, &projectID); err != nil {
					writeError(w, http.StatusBadRequest, "project_id must be numeric")
					return
				}
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
			project, _, err := resolveProjectForDependencyRequest(r.Context(), db, r, user, projectID, ticketID, dependsOn)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			if err := store.DeleteDependency(r.Context(), db, project.ID, ticketID, dependsOn); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "dependency not found")
					return
				}
				writeStoreError(w, err)
				return
			}
			notify("ticket_updated", project.ID, ticketID)
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
