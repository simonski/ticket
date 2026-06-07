package server

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
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
	r.registerGoalHandlers()
	r.registerDocumentHandlers()
	r.registerTicketHandlers()
	r.registerSprintHandlers()
	r.registerOrgHandlers()
	r.registerProgrammeHandlers()
}
