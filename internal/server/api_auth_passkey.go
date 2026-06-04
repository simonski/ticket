package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/passkey"
	"github.com/simonski/ticket/internal/store"
)

type passkeyLoginStartRequest struct {
	Username string `json:"username"`
}

type passkeyRegistrationStartRequest struct {
	Name string `json:"name,omitempty"`
}

type passkeyStartResponse struct {
	VerificationURL string `json:"verification_url"`
	Code            string `json:"code"`
	ExpiresAt       string `json:"expires_at"`
}

type passkeyChallengeResponse struct {
	Kind      string          `json:"kind"`
	PublicKey json.RawMessage `json:"public_key"`
}

type passkeyPollRequest struct {
	Code string `json:"code"`
}

type passkeyPollResponse struct {
	Status string      `json:"status"`
	Token  string      `json:"token,omitempty"`
	User   *store.User `json:"user,omitempty"`
}

type passkeyCredentialResponse struct {
	CredentialID string `json:"credential_id"`
	Name         string `json:"name"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	LastUsedAt   string `json:"last_used_at,omitempty"`
}

func newPasskeyCredentialResponse(credential store.PasskeyCredential) passkeyCredentialResponse {
	return passkeyCredentialResponse{
		CredentialID: credential.CredentialID,
		Name:         credential.Name,
		CreatedAt:    credential.CreatedAt,
		UpdatedAt:    credential.UpdatedAt,
		LastUsedAt:   credential.LastUsedAt,
	}
}

var passkeyPageTemplate = template.Must(template.New("passkey-page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Ticket Passkey</title>
  <style nonce="{{.Nonce}}">
    :root { color-scheme: light dark; }
    body { font-family: system-ui, sans-serif; margin: 0; padding: 2rem; line-height: 1.5; background: #0b1020; color: #f5f7fb; }
    main { max-width: 42rem; margin: 0 auto; background: rgba(16, 23, 42, 0.95); padding: 2rem; border-radius: 16px; box-shadow: 0 24px 60px rgba(0,0,0,.35); }
    h1 { margin-top: 0; }
    code { background: rgba(148,163,184,.2); padding: .15rem .35rem; border-radius: .35rem; }
    .muted { color: #cbd5e1; }
    .error { color: #fca5a5; }
    .success { color: #86efac; }
  </style>
</head>
<body>
  <main data-code="{{.Code}}" data-api-base="{{.APIBase}}">
    <h1>Complete passkey verification</h1>
    <p class="muted">Code: <code>{{.Code}}</code></p>
    <p id="status">Waiting for your browser passkey prompt…</p>
  </main>
  <script nonce="{{.Nonce}}">
    const passkeyRoot = document.querySelector("main");
    const code = passkeyRoot ? String(passkeyRoot.dataset.code || "") : "";
    const apiBase = passkeyRoot ? String(passkeyRoot.dataset.apiBase || "") : "";
    const statusEl = document.getElementById("status");

    function setStatus(text, className) {
      statusEl.textContent = text;
      statusEl.className = className || "";
    }

    function fromBase64URL(value) {
      const base64 = String(value || "").replace(/-/g, "+").replace(/_/g, "/");
      const padded = base64 + "=".repeat((4 - base64.length % 4) % 4);
      const raw = atob(padded);
      const out = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
      return out;
    }

    function toBase64URL(buffer) {
      const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer || []);
      let raw = "";
      for (let i = 0; i < bytes.length; i++) raw += String.fromCharCode(bytes[i]);
      return btoa(raw).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
    }

    function normalizeCreateOptions(value) {
      if (window.PublicKeyCredential && PublicKeyCredential.parseCreationOptionsFromJSON) {
        return PublicKeyCredential.parseCreationOptionsFromJSON(value);
      }
      const copy = structuredClone(value);
      copy.challenge = fromBase64URL(copy.challenge);
      if (copy.user && copy.user.id) copy.user.id = fromBase64URL(copy.user.id);
      if (Array.isArray(copy.excludeCredentials)) {
        copy.excludeCredentials = copy.excludeCredentials.map((item) => ({ ...item, id: fromBase64URL(item.id) }));
      }
      return copy;
    }

    function normalizeGetOptions(value) {
      if (window.PublicKeyCredential && PublicKeyCredential.parseRequestOptionsFromJSON) {
        return PublicKeyCredential.parseRequestOptionsFromJSON(value);
      }
      const copy = structuredClone(value);
      copy.challenge = fromBase64URL(copy.challenge);
      if (Array.isArray(copy.allowCredentials)) {
        copy.allowCredentials = copy.allowCredentials.map((item) => ({ ...item, id: fromBase64URL(item.id) }));
      }
      return copy;
    }

    function serializeCredential(credential) {
      const response = credential.response || {};
      const payload = {
        id: credential.id,
        rawId: toBase64URL(credential.rawId),
        type: credential.type,
        response: {
          clientDataJSON: toBase64URL(response.clientDataJSON),
        },
      };
      if (response.attestationObject) payload.response.attestationObject = toBase64URL(response.attestationObject);
      if (response.authenticatorData) payload.response.authenticatorData = toBase64URL(response.authenticatorData);
      if (response.signature) payload.response.signature = toBase64URL(response.signature);
      if (response.userHandle) payload.response.userHandle = toBase64URL(response.userHandle);
      if (response.transports && typeof response.getTransports === "function") payload.response.transports = response.getTransports();
      if (credential.authenticatorAttachment) payload.authenticatorAttachment = credential.authenticatorAttachment;
      if (typeof credential.getClientExtensionResults === "function") payload.clientExtensionResults = credential.getClientExtensionResults();
      return payload;
    }

    function getCsrfToken() {
      const match = document.cookie.match(/(?:^|;\s*)(?:__Host-_csrf|_csrf)=([^;]*)/);
      return match ? decodeURIComponent(match[1]) : "";
    }

    async function finish(payload) {
      const headers = { "Content-Type": "application/json" };
      const csrfToken = getCsrfToken();
      if (csrfToken) headers["X-CSRF-Token"] = csrfToken;
      const response = await fetch(apiBase + "/finish?code=" + encodeURIComponent(code), {
        method: "POST",
        headers,
        credentials: "same-origin",
        body: JSON.stringify(payload),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(body.error || response.statusText || "passkey verification failed");
      }
      return body;
    }

    async function run() {
      try {
        const challengeResponse = await fetch(apiBase + "/challenge?code=" + encodeURIComponent(code), { credentials: "same-origin" });
        const challenge = await challengeResponse.json().catch(() => ({}));
        if (!challengeResponse.ok) throw new Error(challenge.error || challengeResponse.statusText || "unable to load passkey challenge");
        if (challenge.kind === "registration") {
          const credential = await navigator.credentials.create({ publicKey: normalizeCreateOptions(challenge.public_key) });
          await finish(serializeCredential(credential));
          setStatus("Passkey enrolled. You can return to the CLI.", "success");
          return;
        }
        if (challenge.kind === "login") {
          const credential = await navigator.credentials.get({ publicKey: normalizeGetOptions(challenge.public_key) });
          await finish(serializeCredential(credential));
          setStatus("Passkey accepted. Return to the CLI to finish signing in.", "success");
          return;
        }
        throw new Error("unknown passkey flow");
      } catch (error) {
        setStatus(error && error.message ? error.message : "passkey verification failed", "error");
      }
    }

    run();
  </script>
</body>
</html>`))

func defaultPasskeyServiceFactory(r *http.Request) (passkey.Service, error) {
	return passkey.New(passkey.Config{
		RPID:        passkeyRelyingPartyID(r),
		Origin:      passkeyOrigin(r),
		DisplayName: passkeyDisplayName(),
		RequireUV:   true,
		RequireRK:   true,
	})
}

func passkeyRelyingPartyID(r *http.Request) string {
	if override := strings.TrimSpace(os.Getenv("TICKET_PASSKEY_RP_ID")); override != "" {
		return override
	}
	return passkeyHost(r)
}

func passkeyOrigin(r *http.Request) string {
	if override := strings.TrimSpace(os.Getenv("TICKET_PASSKEY_ORIGIN")); override != "" {
		return override
	}
	scheme := "http"
	if requestIsSecure(r) {
		scheme = "https"
	}
	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	return scheme + "://" + host
}

func passkeyDisplayName() string {
	if displayName := strings.TrimSpace(os.Getenv("TICKET_PASSKEY_RP_DISPLAY_NAME")); displayName != "" {
		return displayName
	}
	return "Ticket"
}

func passkeyHost(r *http.Request) string {
	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return parsedHost
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return strings.Trim(host, "[]")
	}
	return host
}

func firstHeaderValue(raw string) string {
	value := strings.TrimSpace(raw)
	if comma := strings.Index(value, ","); comma >= 0 {
		value = strings.TrimSpace(value[:comma])
	}
	return value
}

func normalizeExternalPathPrefix(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "/" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return strings.TrimRight(value, "/")
}

func externalPathPrefix(r *http.Request, routePath string) string {
	if forwarded := normalizeExternalPathPrefix(firstHeaderValue(r.Header.Get("X-Forwarded-Prefix"))); forwarded != "" {
		return forwarded
	}
	pathValue := strings.TrimSpace(r.URL.Path)
	routePath = strings.TrimSpace(routePath)
	if routePath == "" || pathValue == "" {
		return ""
	}
	if strings.HasSuffix(pathValue, routePath) {
		return normalizeExternalPathPrefix(strings.TrimSuffix(pathValue, routePath))
	}
	return ""
}

func absolutePasskeyURL(r *http.Request, routePath, code string) string {
	return passkeyOrigin(r) + externalPathPrefix(r, routePath) + "/passkey?code=" + url.QueryEscape(code)
}

func passkeyUser(user store.User, credentials []store.PasskeyCredential) passkey.User {
	out := passkey.User{
		ID:          user.ID,
		Name:        user.Username,
		DisplayName: strings.TrimSpace(user.DisplayName),
		Credentials: make([]passkey.StoredCredential, 0, len(credentials)),
	}
	for _, credential := range credentials {
		out.Credentials = append(out.Credentials, passkey.StoredCredential{
			ID:   credential.CredentialID,
			Data: credential.CredentialJSON,
		})
	}
	return out
}

func (r *router) registerPasskeyHandlers() {
	r.mux.HandleFunc("/passkey", r.handlePasskeyPage)
	r.mux.HandleFunc("/api/auth/passkey/login/start", r.handlePasskeyLoginStart)
	r.mux.HandleFunc("/api/auth/passkey/register/start", r.handlePasskeyRegistrationStart)
	r.mux.HandleFunc("/api/auth/passkey/challenge", r.handlePasskeyChallenge)
	r.mux.HandleFunc("/api/auth/passkey/finish", r.handlePasskeyFinish)
	r.mux.HandleFunc("/api/auth/passkey/poll", r.handlePasskeyPoll)
}

func (r *router) handlePasskeyPage(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	code := strings.TrimSpace(req.URL.Query().Get("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "passkey code is required")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := passkeyPageTemplate.Execute(w, map[string]any{
		"Code":    code,
		"APIBase": externalPathPrefix(req, "/passkey") + "/api/auth/passkey",
		"Nonce":   cspNonceFromContext(req.Context()),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (r *router) handlePasskeyLoginStart(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !r.authLimiter.allow(clientIP(req)) {
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	var payload passkeyLoginStartRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	username := strings.TrimSpace(payload.Username)
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	user, err := store.GetUserByUsername(req.Context(), r.db, username)
	if err != nil || !user.Enabled {
		writeError(w, http.StatusNotFound, store.ErrPasskeyUnavailable.Error())
		return
	}
	credentials, err := store.ListPasskeyCredentials(req.Context(), r.db, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(credentials) == 0 {
		writeError(w, http.StatusNotFound, store.ErrPasskeyUnavailable.Error())
		return
	}
	service, err := r.passkeys(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ceremony, err := service.BeginLogin(passkeyUser(user, credentials))
	if err != nil {
		if errors.Is(err, passkey.ErrNoCredentials) {
			writeError(w, http.StatusNotFound, store.ErrPasskeyUnavailable.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	flow, err := store.CreatePasskeyFlow(req.Context(), r.db, store.PasskeyFlowPurposeLogin, user.ID, "", ceremony.Session, string(ceremony.PublicKey))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, passkeyStartResponse{
		VerificationURL: absolutePasskeyURL(req, "/api/auth/passkey/login/start", flow.Code),
		Code:            flow.Code,
		ExpiresAt:       flow.ExpiresAt,
	})
}

func (r *router) handlePasskeyRegistrationStart(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, err := userFromRequest(r.db, req)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var payload passkeyRegistrationStartRequest
	if decodeErr := json.NewDecoder(req.Body).Decode(&payload); decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	credentials, err := store.ListPasskeyCredentials(req.Context(), r.db, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	service, err := r.passkeys(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ceremony, err := service.BeginRegistration(passkeyUser(user, credentials))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	flow, err := store.CreatePasskeyFlow(req.Context(), r.db, store.PasskeyFlowPurposeRegistration, user.ID, strings.TrimSpace(payload.Name), ceremony.Session, string(ceremony.PublicKey))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, passkeyStartResponse{
		VerificationURL: absolutePasskeyURL(req, "/api/auth/passkey/register/start", flow.Code),
		Code:            flow.Code,
		ExpiresAt:       flow.ExpiresAt,
	})
}

func (r *router) handlePasskeyChallenge(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	flow, err := store.GetPasskeyFlow(req.Context(), r.db, req.URL.Query().Get("code"))
	if err != nil {
		r.writePasskeyFlowError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, passkeyChallengeResponse{
		Kind:      flow.Purpose,
		PublicKey: json.RawMessage(flow.OptionsJSON),
	})
}

func (r *router) handlePasskeyFinish(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !r.authLimiter.allow(clientIP(req)) {
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	flow, err := store.GetPasskeyFlow(req.Context(), r.db, req.URL.Query().Get("code"))
	if err != nil {
		r.writePasskeyFlowError(w, err)
		return
	}
	user, err := store.GetUserByID(req.Context(), r.db, flow.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	credentials, err := store.ListPasskeyCredentials(req.Context(), r.db, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	service, err := r.passkeys(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch flow.Purpose {
	case store.PasskeyFlowPurposeLogin:
		result, err := service.FinishLogin(passkeyUser(user, credentials), flow.SessionJSON, body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if updateErr := store.UpdatePasskeyCredential(req.Context(), r.db, result.ID, result.Data); updateErr != nil {
			writeStoreError(w, updateErr)
			return
		}
		token, err := store.CreateSession(req.Context(), r.db, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := store.CompletePasskeyFlow(req.Context(), r.db, flow.Code, token); err != nil {
			r.writePasskeyFlowError(w, err)
			return
		}
	case store.PasskeyFlowPurposeRegistration:
		result, err := service.FinishRegistration(passkeyUser(user, credentials), flow.SessionJSON, body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if saveErr := store.SavePasskeyCredential(req.Context(), r.db, user.ID, flow.CredentialName, result.ID, result.Data); saveErr != nil {
			writeStoreError(w, saveErr)
			return
		}
		if err := store.CompletePasskeyFlow(req.Context(), r.db, flow.Code, ""); err != nil {
			r.writePasskeyFlowError(w, err)
			return
		}
	default:
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("unsupported passkey flow %q", flow.Purpose))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handlePasskeyPoll(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload passkeyPollRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	flow, err := store.ConsumePasskeyFlow(req.Context(), r.db, payload.Code)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrPasskeyFlowPending):
			writeJSON(w, http.StatusAccepted, passkeyPollResponse{Status: store.PasskeyFlowStatusPending})
		default:
			r.writePasskeyFlowError(w, err)
		}
		return
	}
	response := passkeyPollResponse{Status: store.PasskeyFlowStatusComplete}
	if flow.Purpose == store.PasskeyFlowPurposeLogin && strings.TrimSpace(flow.Token) != "" {
		user, err := store.GetUserByToken(req.Context(), r.db, flow.Token)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		response.Token = flow.Token
		response.User = &user
	}
	writeJSON(w, http.StatusOK, response)
}

func (r *router) writePasskeyFlowError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrPasskeyNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrPasskeyFlowExpired), errors.Is(err, store.ErrPasskeyFlowConsumed):
		writeError(w, http.StatusGone, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
