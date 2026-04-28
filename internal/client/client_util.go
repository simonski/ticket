package client

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

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
	db, err := store.Open(c.localDBPath)
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

type HTTPStatusError struct {
	StatusCode int
	Status     string
	APIError   string
}

func (e *HTTPStatusError) Error() string {
	if strings.TrimSpace(e.APIError) != "" {
		return e.APIError
	}
	return fmt.Sprintf("request failed with status %s", e.Status)
}

func statusErrorFromResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}
	var apiErr struct {
		Error string `json:"error"`
	}
	if len(body) > 0 && json.Unmarshal(body, &apiErr) == nil && strings.TrimSpace(apiErr.Error) != "" {
		return &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			APIError:   strings.TrimSpace(apiErr.Error),
		}
	}
	return &HTTPStatusError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
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
		return statusErrorFromResponse(resp)
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

	if resp.StatusCode >= 400 {
		return statusErrorFromResponse(resp)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
