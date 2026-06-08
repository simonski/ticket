package server

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/simonski/ticket/internal/orchestrator"
	"github.com/simonski/ticket/internal/store"
)

// triggerRefinementPass runs a single orchestrator pass scoped to one ticket,
// off the request path. It is fired when a human posts a message into a
// refine-stage conversation so a refiner agent is assigned immediately, rather
// than waiting for the next periodic orchestrator wake — giving the refinement
// dialogue a near-real-time feel.
func triggerRefinementPass(db *sql.DB, ticketID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := orchestrator.Pass(ctx, db, orchestrator.Options{TicketID: ticketID}); err != nil {
			slog.Error("refinement trigger pass error", "ticket", ticketID, "error", err)
		}
	}()
}

// maybeTriggerRefinement fires an immediate refiner assignment if the ticket is in
// the refine stage. Safe to call after any human comment.
func maybeTriggerRefinement(db *sql.DB, ticket store.Ticket) {
	if ticket.Stage == store.StageRefine {
		triggerRefinementPass(db, ticket.ID)
	}
}
