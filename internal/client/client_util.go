package client

import (
	"bytes"
	"context"
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
	c.localDBMu.Lock()
	defer c.localDBMu.Unlock()
	if c.localDB != nil {
		return c.localDB, nil
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return nil, err
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		return nil, err
	}
	c.localDB = db
	return c.localDB, nil
}

func (c *Client) localUser(ctx context.Context, db *sql.DB) (store.User, error) {
	return ensureLocalUser(ctx, db, localUsername())
}

func ensureLocalUser(ctx context.Context, db *sql.DB, username string) (store.User, error) {
	if user, err := store.GetUserByUsername(ctx, db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if err := store.SetUserEnabled(ctx, db, username, true); err != nil {
			return store.User{}, err
		}
		return store.GetUserByUsername(ctx, db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	user, err := store.CreateUser(ctx, db, username, "local-mode", "admin")
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

// friendlyConnectionError converts low-level network errors to a clear message.
func friendlyConnectionError(err error, baseURL string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var netErr *net.OpError
		if errors.As(urlErr.Err, &netErr) {
			return fmt.Errorf("cannot connect to %s\nhint: is the server running? check your config location", baseURL)
		}
	}
	return fmt.Errorf("cannot connect to %s", baseURL)
}

// doJSONBasicAuth is like doJSON but uses HTTP Basic Auth instead of Bearer token.
func (c *Client) doJSONBasicAuth(ctx context.Context, method, path, username, password string, body any, out any) error {
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

	httpRequest, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	httpRequest.SetBasicAuth(username, password)

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

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = encoded
	}

	if c.shouldAutoAuthenticate(path) && c.token == "" {
		if err := c.authenticateFromEnvironment(ctx); err != nil {
			return err
		}
	}

	send := func(token string) (*http.Response, error) {
		httpRequest, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		if body != nil {
			httpRequest.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			httpRequest.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := c.http.Do(httpRequest)
		if err != nil {
			return nil, friendlyConnectionError(err, c.baseURL)
		}
		return resp, nil
	}

	resp, err := send(c.token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Env-based remote auth behaves like a stateless client: if token expired,
	// refresh it and retry once.
	if resp.StatusCode == http.StatusUnauthorized && c.shouldAutoAuthenticate(path) {
		c.token = ""
		if err := c.authenticateFromEnvironment(ctx); err == nil && c.token != "" {
			resp.Body.Close()
			resp, err = send(c.token)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
		}
	}

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

func (c *Client) shouldAutoAuthenticate(path string) bool {
	if c.mode != config.ModeRemote || !config.HasRemoteEnvOverride() {
		return false
	}
	switch path {
	case "/api/login", "/api/register":
		return false
	default:
		return true
	}
}

func (c *Client) authenticateFromEnvironment(ctx context.Context) error {
	username := getenvFirst("TICKET_USERNAME")
	password := getenvFirst("TICKET_PASSWORD")
	if username == "" || password == "" {
		return errors.New("TICKET_USERNAME and TICKET_PASSWORD are required when TICKET_URL is set")
	}
	response, err := c.Login(ctx, username, password)
	if err != nil {
		return err
	}
	c.token = response.Token
	return nil
}
