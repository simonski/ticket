package server

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/simonski/ticket/internal/store"
)

func registerAPI(mux *http.ServeMux, db *sql.DB, version string, live *liveHub, verbose bool, output io.Writer, passkeys passkeyServiceFactory) {
	authLimiter := newRateLimiter(10, 1*time.Minute)
	var chatLog func(string)
	if verbose {
		if output == nil {
			output = io.Discard
		}
		var chatLogMu sync.Mutex
		chatLog = func(line string) {
			chatLogMu.Lock()
			defer chatLogMu.Unlock()
			fmt.Fprintf(output, "CHAT %s\n", strings.TrimRight(line, "\n"))
		}
	}

	// refineLog mirrors chatLog for the refinement WebSocket: when the server is
	// started with -v it prints the refiner command, prompt, and its stdout/stderr
	// so failures (e.g. "exit status 1") can be diagnosed.
	var refineLog func(string)
	if verbose {
		if output == nil {
			output = io.Discard
		}
		var refineLogMu sync.Mutex
		refineLog = func(line string) {
			refineLogMu.Lock()
			defer refineLogMu.Unlock()
			fmt.Fprintf(output, "REFINE %s\n", strings.TrimRight(line, "\n"))
		}
	}

	notify := func(eventType string, projectID int64, ticketID string) {
		if live == nil {
			return
		}
		live.broadcast(buildLiveChangeEvent(eventType, projectID, ticketID))
	}

	r := &router{
		mux:         mux,
		db:          db,
		version:     version,
		live:        live,
		verbose:     verbose,
		output:      output,
		notify:      notify,
		authLimiter: authLimiter,
		chatLog:     chatLog,
		passkeys:    passkeys,
	}
	r.registerAuthHandlers()
	r.registerSystemHandlers()
	r.registerPlanHandlers()
	r.registerUserHandlers()
	r.registerAgentHandlers()
	r.registerRoleHandlers()
	r.registerWorkflowHandlers()
	r.registerTeamHandlers()
	r.registerProjectHandlers()
	r.registerDocumentHandlers()
	r.registerRoomHandlers()
	r.registerAccessRoleHandlers()
	r.registerTicketHandlers()
	r.registerReleaseHandlers()
	r.registerOrgHandlers()
	r.registerProgrammeHandlers()

	// Streaming refinement chat: GET /api/refinement/ws?ticket=ID upgrades to a
	// WebSocket that streams a refiner LLM's reply token by token (Phase 6).
	mux.HandleFunc("/api/refinement/ws", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := bearerToken(req)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, err := store.GetUserByToken(req.Context(), db, token)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		ticketID := strings.TrimSpace(req.URL.Query().Get("ticket"))
		if ticketID == "" {
			writeError(w, http.StatusBadRequest, "ticket query parameter is required")
			return
		}
		if _, err := store.GetTicket(req.Context(), db, ticketID); err != nil {
			writeStoreError(w, err)
			return
		}
		if err := websocketServeRefinement(w, req, db, ticketID, user.ID, notify, refineLog); err != nil {
			if !strings.Contains(err.Error(), "upgrade") {
				writeError(w, http.StatusInternalServerError, err.Error())
			}
		}
	})
}
