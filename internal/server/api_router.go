package server

import (
	"database/sql"
	"io"
	"net/http"

	"github.com/simonski/ticket/internal/passkey"
)

type passkeyServiceFactory func(*http.Request) (passkey.Service, error)

type router struct {
	mux         *http.ServeMux
	db          *sql.DB
	version     string
	live        *liveHub
	verbose     bool
	output      io.Writer
	notify      func(eventType string, projectID int64, ticketID string)
	authLimiter *rateLimiter
	chatLog     func(string)
	passkeys    passkeyServiceFactory
}
