package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/passkey"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/testutil"
)

type fakePasskeyService struct {
	beginLoginResult         passkey.Ceremony
	beginRegistrationResult  passkey.Ceremony
	finishLoginResult        passkey.Result
	finishRegistrationResult passkey.Result

	loginUser       passkey.User
	loginSession    string
	loginBody       []byte
	registerUser    passkey.User
	registerSession string
	registerBody    []byte
}

func (f *fakePasskeyService) BeginRegistration(user passkey.User) (passkey.Ceremony, error) {
	f.registerUser = user
	return f.beginRegistrationResult, nil
}

func (f *fakePasskeyService) FinishRegistration(user passkey.User, session string, response []byte) (passkey.Result, error) {
	f.registerUser = user
	f.registerSession = session
	f.registerBody = append([]byte{}, response...)
	return f.finishRegistrationResult, nil
}

func (f *fakePasskeyService) BeginLogin(user passkey.User) (passkey.Ceremony, error) {
	f.loginUser = user
	return f.beginLoginResult, nil
}

func (f *fakePasskeyService) FinishLogin(user passkey.User, session string, response []byte) (passkey.Result, error) {
	f.loginUser = user
	f.loginSession = session
	f.loginBody = append([]byte{}, response...)
	return f.finishLoginResult, nil
}

func testHandlerWithPasskeys(t *testing.T, svc passkey.Service) (http.Handler, *sql.DB) {
	t.Helper()
	dbPath := testutil.SeededDBPath(t, "password")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	handler, err := handlerWithPasskeyFactory(db, "1.2.3", false, nil, "", "", func(*http.Request) (passkey.Service, error) {
		return svc, nil
	})
	if err != nil {
		t.Fatalf("handlerWithPasskeyFactory() error = %v", err)
	}
	return handler, db
}

func TestPasskeyLoginAPIFlow(t *testing.T) {
	t.Parallel()
	fake := &fakePasskeyService{
		beginLoginResult:  passkey.Ceremony{PublicKey: json.RawMessage(`{"challenge":"abc"}`), Session: `{"challenge":"session-1"}`},
		finishLoginResult: passkey.Result{ID: "cred-1", Data: `{"signCount":2}`},
	}
	handler, db := testHandlerWithPasskeys(t, fake)
	defer db.Close()

	user, err := store.CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := store.SavePasskeyCredential(context.Background(), db, user.ID, "laptop", "cred-1", `{"signCount":1}`); err != nil {
		t.Fatalf("SavePasskeyCredential() error = %v", err)
	}

	startResp := doJSONRequest(t, handler, http.MethodPost, "/api/auth/passkey/login/start", map[string]string{"username": "alice"}, "")
	if startResp.Code != http.StatusOK {
		t.Fatalf("passkey login start status = %d body=%s", startResp.Code, startResp.Body.String())
	}
	var startPayload passkeyStartResponse
	decodeResponse(t, startResp, &startPayload)
	if startPayload.Code == "" {
		t.Fatal("passkey login start should return a code")
	}

	pageResp := doJSONRequest(t, handler, http.MethodGet, "/passkey?code="+startPayload.Code, nil, "")
	if pageResp.Code != http.StatusOK {
		t.Fatalf("passkey page status = %d body=%s", pageResp.Code, pageResp.Body.String())
	}
	if !strings.Contains(pageResp.Body.String(), startPayload.Code) {
		t.Fatalf("passkey page missing flow code:\n%s", pageResp.Body.String())
	}

	challengeResp := doJSONRequest(t, handler, http.MethodGet, "/api/auth/passkey/challenge?code="+startPayload.Code, nil, "")
	if challengeResp.Code != http.StatusOK {
		t.Fatalf("passkey challenge status = %d body=%s", challengeResp.Code, challengeResp.Body.String())
	}

	finishResp := doRawRequest(t, handler, http.MethodPost, "/api/auth/passkey/finish?code="+startPayload.Code, []byte(`{"id":"credential"}`), "")
	if finishResp.Code != http.StatusOK {
		t.Fatalf("passkey finish status = %d body=%s", finishResp.Code, finishResp.Body.String())
	}
	if fake.loginSession != `{"challenge":"session-1"}` {
		t.Fatalf("finish login session = %q", fake.loginSession)
	}
	if !bytes.Equal(fake.loginBody, []byte(`{"id":"credential"}`)) {
		t.Fatalf("finish login body = %s", string(fake.loginBody))
	}

	pollResp := doJSONRequest(t, handler, http.MethodPost, "/api/auth/passkey/poll", map[string]string{"code": startPayload.Code}, "")
	if pollResp.Code != http.StatusOK {
		t.Fatalf("passkey poll status = %d body=%s", pollResp.Code, pollResp.Body.String())
	}
	var pollPayload passkeyPollResponse
	decodeResponse(t, pollResp, &pollPayload)
	if pollPayload.User == nil || pollPayload.User.Username != "alice" {
		t.Fatalf("poll user = %#v", pollPayload.User)
	}
	if strings.TrimSpace(pollPayload.Token) == "" {
		t.Fatal("poll should return a token after login completion")
	}

	credentials, err := store.ListPasskeyCredentials(context.Background(), db, user.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials() error = %v", err)
	}
	if credentials[0].CredentialJSON != `{"signCount":2}` {
		t.Fatalf("credential json after login = %q", credentials[0].CredentialJSON)
	}
}

func TestPasskeyRegistrationAPIFlow(t *testing.T) {
	t.Parallel()
	fake := &fakePasskeyService{
		beginRegistrationResult:  passkey.Ceremony{PublicKey: json.RawMessage(`{"challenge":"abc"}`), Session: `{"challenge":"session-2"}`},
		finishRegistrationResult: passkey.Result{ID: "cred-reg", Data: `{"signCount":1}`},
	}
	handler, db := testHandlerWithPasskeys(t, fake)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	startResp := doJSONRequest(t, handler, http.MethodPost, "/api/auth/passkey/register/start", map[string]string{"name": "MacBook"}, adminToken)
	if startResp.Code != http.StatusOK {
		t.Fatalf("passkey register start status = %d body=%s", startResp.Code, startResp.Body.String())
	}
	var startPayload passkeyStartResponse
	decodeResponse(t, startResp, &startPayload)

	finishResp := doRawRequest(t, handler, http.MethodPost, "/api/auth/passkey/finish?code="+startPayload.Code, []byte(`{"id":"credential"}`), "")
	if finishResp.Code != http.StatusOK {
		t.Fatalf("passkey register finish status = %d body=%s", finishResp.Code, finishResp.Body.String())
	}

	pollResp := doJSONRequest(t, handler, http.MethodPost, "/api/auth/passkey/poll", map[string]string{"code": startPayload.Code}, "")
	if pollResp.Code != http.StatusOK {
		t.Fatalf("passkey register poll status = %d body=%s", pollResp.Code, pollResp.Body.String())
	}
	var pollPayload passkeyPollResponse
	decodeResponse(t, pollResp, &pollPayload)
	if pollPayload.Status != store.PasskeyFlowStatusComplete {
		t.Fatalf("poll status = %q, want %q", pollPayload.Status, store.PasskeyFlowStatusComplete)
	}
	if pollPayload.Token != "" {
		t.Fatalf("registration poll should not return a token: %#v", pollPayload)
	}

	adminUser, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	credentials, err := store.ListPasskeyCredentials(context.Background(), db, adminUser.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials(admin) error = %v", err)
	}
	if len(credentials) != 1 {
		t.Fatalf("ListPasskeyCredentials(admin) len = %d, want 1", len(credentials))
	}
	if credentials[0].Name != "MacBook" {
		t.Fatalf("credential name = %q, want MacBook", credentials[0].Name)
	}
}

func TestPasskeyManagementAPIFlow(t *testing.T) {
	t.Parallel()
	handler, db := testHandlerWithPasskeys(t, &fakePasskeyService{})
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	adminUser, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	if err := store.SavePasskeyCredential(context.Background(), db, adminUser.ID, "Laptop", "cred-admin", `{"signCount":1}`); err != nil {
		t.Fatalf("SavePasskeyCredential(admin) error = %v", err)
	}

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/users/me/passkeys", nil, adminToken)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list passkeys status = %d body=%s", listResp.Code, listResp.Body.String())
	}
	var listPayload []map[string]any
	decodeResponse(t, listResp, &listPayload)
	if len(listPayload) != 1 {
		t.Fatalf("list passkeys len = %d, want 1", len(listPayload))
	}
	if _, ok := listPayload[0]["credential_json"]; ok {
		t.Fatalf("list passkeys should not include credential_json: %#v", listPayload[0])
	}
	if got := strings.TrimSpace(listPayload[0]["name"].(string)); got != "Laptop" {
		t.Fatalf("list passkeys name = %q, want Laptop", got)
	}

	renameResp := doJSONRequest(t, handler, http.MethodPut, "/api/users/me/passkeys/cred-admin", map[string]string{"name": "Desk key"}, adminToken)
	if renameResp.Code != http.StatusOK {
		t.Fatalf("rename passkey status = %d body=%s", renameResp.Code, renameResp.Body.String())
	}
	credentials, err := store.ListPasskeyCredentials(context.Background(), db, adminUser.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials(admin) error = %v", err)
	}
	if credentials[0].Name != "Desk key" {
		t.Fatalf("renamed credential name = %q, want Desk key", credentials[0].Name)
	}

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/users/me/passkeys/cred-admin", nil, adminToken)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete passkey status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
	credentials, err = store.ListPasskeyCredentials(context.Background(), db, adminUser.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials(after delete) error = %v", err)
	}
	if len(credentials) != 0 {
		t.Fatalf("ListPasskeyCredentials(after delete) len = %d, want 0", len(credentials))
	}
}

func TestPasskeyFinishCSRFEnforcedWhenSessionCookiePresent(t *testing.T) {
	t.Parallel()
	fake := &fakePasskeyService{
		beginLoginResult:  passkey.Ceremony{PublicKey: json.RawMessage(`{"challenge":"abc"}`), Session: `{"challenge":"session-1"}`},
		finishLoginResult: passkey.Result{ID: "cred-1", Data: `{"signCount":2}`},
	}
	handler, db := testHandlerWithPasskeys(t, fake)
	defer db.Close()

	user, err := store.CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := store.SavePasskeyCredential(context.Background(), db, user.ID, "laptop", "cred-1", `{"signCount":1}`); err != nil {
		t.Fatalf("SavePasskeyCredential() error = %v", err)
	}

	startResp := doJSONRequest(t, handler, http.MethodPost, "/api/auth/passkey/login/start", map[string]string{"username": "alice"}, "")
	if startResp.Code != http.StatusOK {
		t.Fatalf("passkey login start status = %d body=%s", startResp.Code, startResp.Body.String())
	}
	var startPayload passkeyStartResponse
	decodeResponse(t, startResp, &startPayload)

	// Simulate a browser with an existing session: GET challenge to get CSRF cookie.
	challengeReq := httptest.NewRequest(http.MethodGet, "/api/auth/passkey/challenge?code="+startPayload.Code, nil)
	challengeReq.AddCookie(&http.Cookie{Name: legacySessionCookieName, Value: "fake-session"})
	challengeRec := httptest.NewRecorder()
	handler.ServeHTTP(challengeRec, challengeReq)
	if challengeRec.Code != http.StatusOK {
		t.Fatalf("challenge status = %d", challengeRec.Code)
	}

	// Extract CSRF cookie set by the challenge GET.
	var csrfToken string
	for _, c := range challengeRec.Result().Cookies() {
		if c.Name == legacyCSRFCookieName || c.Name == hostCSRFCookieName {
			csrfToken = c.Value
		}
	}
	if csrfToken == "" {
		t.Fatal("challenge GET should set a CSRF cookie")
	}

	// POST finish with session cookie but no CSRF header — must be rejected.
	noCSRFReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/finish?code="+startPayload.Code, bytes.NewReader([]byte(`{"id":"credential"}`)))
	noCSRFReq.Header.Set("Content-Type", "application/json")
	noCSRFReq.AddCookie(&http.Cookie{Name: legacySessionCookieName, Value: "fake-session"})
	noCSRFReq.AddCookie(&http.Cookie{Name: legacyCSRFCookieName, Value: csrfToken})
	noCSRFRec := httptest.NewRecorder()
	handler.ServeHTTP(noCSRFRec, noCSRFReq)
	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("finish without CSRF header status = %d, want 403", noCSRFRec.Code)
	}

	// POST finish with session cookie and matching CSRF header — must succeed.
	withCSRFReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/finish?code="+startPayload.Code, bytes.NewReader([]byte(`{"id":"credential"}`)))
	withCSRFReq.Header.Set("Content-Type", "application/json")
	withCSRFReq.Header.Set("X-CSRF-Token", csrfToken)
	withCSRFReq.AddCookie(&http.Cookie{Name: legacySessionCookieName, Value: "fake-session"})
	withCSRFReq.AddCookie(&http.Cookie{Name: legacyCSRFCookieName, Value: csrfToken})
	withCSRFRec := httptest.NewRecorder()
	handler.ServeHTTP(withCSRFRec, withCSRFReq)
	if withCSRFRec.Code != http.StatusOK {
		t.Fatalf("finish with CSRF header status = %d body=%s", withCSRFRec.Code, withCSRFRec.Body.String())
	}
}

func TestPasskeyVerificationURLAndPageHonorForwardedPrefix(t *testing.T) {
	t.Parallel()
	fake := &fakePasskeyService{
		beginLoginResult: passkey.Ceremony{
			PublicKey: json.RawMessage(`{"challenge":"abc"}`),
			Session:   `{"challenge":"session-1"}`,
		},
	}
	handler, db := testHandlerWithPasskeys(t, fake)
	defer db.Close()

	user, err := store.CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := store.SavePasskeyCredential(context.Background(), db, user.ID, "laptop", "cred-1", `{"signCount":1}`); err != nil {
		t.Fatalf("SavePasskeyCredential() error = %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": "alice"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "tickets.example.com")
	req.Header.Set("X-Forwarded-Prefix", "/ticket")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("passkey login start status = %d body=%s", rec.Code, rec.Body.String())
	}
	var startPayload passkeyStartResponse
	decodeResponse(t, rec, &startPayload)
	if startPayload.VerificationURL != "https://tickets.example.com/ticket/passkey?code="+startPayload.Code {
		t.Fatalf("verification url = %q", startPayload.VerificationURL)
	}

	pageReq := httptest.NewRequest(http.MethodGet, "/passkey?code="+startPayload.Code, nil)
	pageReq.Header.Set("X-Forwarded-Prefix", "/ticket")
	pageRec := httptest.NewRecorder()
	handler.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("passkey page status = %d body=%s", pageRec.Code, pageRec.Body.String())
	}
	pageHTML := pageRec.Body.String()
	if !strings.Contains(pageHTML, `data-code="`+startPayload.Code+`"`) {
		t.Fatalf("passkey page missing flow code dataset:\n%s", pageHTML)
	}
	if !strings.Contains(pageHTML, `data-api-base="/ticket/api/auth/passkey"`) {
		t.Fatalf("passkey page missing prefixed api base:\n%s", pageHTML)
	}
}
