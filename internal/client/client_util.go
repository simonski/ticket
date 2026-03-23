package client

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func resolveRequestLifecycle(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

func (c *Client) openLocalDB() (*sql.DB, error) {
	resolved, err := config.ResolveURL()
	if err != nil {
		return nil, err
	}
	return store.Open(resolved.DBPath)
}

func (c *Client) localUser(db *sql.DB) (store.User, error) {
	return ensureLocalUser(db, localUsername())
}

func ensureLocalUser(db *sql.DB, username string) (store.User, error) {
	if user, err := store.GetUserByUsername(db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if err := store.SetUserEnabled(db, username, true); err != nil {
			return store.User{}, err
		}
		return store.GetUserByUsername(db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	user, err := store.CreateUser(db, username, "local-mode", "admin")
	if err != nil {
		return store.User{}, err
	}
	return user, nil
}

func localUsername() string {
	return "admin"
}

func getenvFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

// friendlyConnectionError wraps low-level network errors with a clear message.
func friendlyConnectionError(err error, baseURL string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var netErr *net.OpError
		if errors.As(urlErr.Err, &netErr) {
			return fmt.Errorf("cannot connect to %s: %w\nhint: is the server running? check TICKET_URL", baseURL, netErr)
		}
	}
	return fmt.Errorf("cannot connect to %s: %w", baseURL, err)
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	httpRequest, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(httpRequest)
	if err != nil {
		return friendlyConnectionError(err, c.baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("request failed with status %s", resp.Status)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
