package server

import (
	"database/sql"
	"io"
	"net/http"
)

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
}
