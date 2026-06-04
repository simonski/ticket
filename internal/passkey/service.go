package passkey

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
)

var ErrNoCredentials = errors.New("no passkeys enrolled for this user")

type Config struct {
	RPID        string
	Origin      string
	DisplayName string
	RequireUV   bool
	RequireRK   bool
}

type StoredCredential struct {
	ID   string
	Data string
}

type User struct {
	ID          string
	Name        string
	DisplayName string
	Credentials []StoredCredential
}

type Ceremony struct {
	PublicKey json.RawMessage
	Session   string
}

type Result struct {
	ID   string
	Data string
}

type Service interface {
	BeginRegistration(user User) (Ceremony, error)
	FinishRegistration(user User, session string, response []byte) (Result, error)
	BeginLogin(user User) (Ceremony, error)
	FinishLogin(user User, session string, response []byte) (Result, error)
}

type service struct {
	wa *webauthnlib.WebAuthn
}

func New(cfg Config) (Service, error) {
	rpID := strings.TrimSpace(cfg.RPID)
	origin := strings.TrimSpace(cfg.Origin)
	displayName := strings.TrimSpace(cfg.DisplayName)
	if rpID == "" {
		return nil, errors.New("passkey relying party id is required")
	}
	if origin == "" {
		return nil, errors.New("passkey origin is required")
	}
	if displayName == "" {
		displayName = "Ticket"
	}
	selection := protocol.AuthenticatorSelection{}
	if cfg.RequireUV {
		selection.UserVerification = protocol.VerificationRequired
	}
	if cfg.RequireRK {
		selection.ResidentKey = protocol.ResidentKeyRequirementRequired
		selection.RequireResidentKey = protocol.ResidentKeyRequired()
	}
	wa, err := webauthnlib.New(&webauthnlib.Config{
		RPID:                   rpID,
		RPDisplayName:          displayName,
		RPOrigins:              []string{origin},
		AuthenticatorSelection: selection,
	})
	if err != nil {
		return nil, err
	}
	return &service{wa: wa}, nil
}

func (s *service) BeginRegistration(user User) (Ceremony, error) {
	wu, err := adaptUser(user)
	if err != nil {
		return Ceremony{}, err
	}
	options, session, err := s.wa.BeginRegistration(wu)
	if err != nil {
		return Ceremony{}, err
	}
	return marshalCeremony(options.Response, session)
}

func (s *service) FinishRegistration(user User, session string, response []byte) (Result, error) {
	wu, err := adaptUser(user)
	if err != nil {
		return Result{}, err
	}
	sessionData, err := unmarshalSession(session)
	if err != nil {
		return Result{}, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return Result{}, err
	}
	credential, err := s.wa.CreateCredential(wu, sessionData, parsed)
	if err != nil {
		return Result{}, err
	}
	return marshalCredential(credential)
}

func (s *service) BeginLogin(user User) (Ceremony, error) {
	wu, err := adaptUser(user)
	if err != nil {
		return Ceremony{}, err
	}
	if len(wu.credentials) == 0 {
		return Ceremony{}, ErrNoCredentials
	}
	options, session, err := s.wa.BeginLogin(wu)
	if err != nil {
		return Ceremony{}, err
	}
	return marshalCeremony(options.Response, session)
}

func (s *service) FinishLogin(user User, session string, response []byte) (Result, error) {
	wu, err := adaptUser(user)
	if err != nil {
		return Result{}, err
	}
	sessionData, err := unmarshalSession(session)
	if err != nil {
		return Result{}, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return Result{}, err
	}
	credential, err := s.wa.ValidateLogin(wu, sessionData, parsed)
	if err != nil {
		return Result{}, err
	}
	return marshalCredential(credential)
}

func marshalCeremony(publicKey any, session *webauthnlib.SessionData) (Ceremony, error) {
	publicKeyJSON, err := json.Marshal(publicKey)
	if err != nil {
		return Ceremony{}, err
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return Ceremony{}, err
	}
	return Ceremony{
		PublicKey: publicKeyJSON,
		Session:   string(sessionJSON),
	}, nil
}

func marshalCredential(credential *webauthnlib.Credential) (Result, error) {
	if credential == nil {
		return Result{}, errors.New("passkey credential is required")
	}
	payload, err := json.Marshal(credential)
	if err != nil {
		return Result{}, err
	}
	return Result{
		ID:   base64.RawURLEncoding.EncodeToString(credential.ID),
		Data: string(payload),
	}, nil
}

func unmarshalSession(raw string) (webauthnlib.SessionData, error) {
	var session webauthnlib.SessionData
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &session); err != nil {
		return webauthnlib.SessionData{}, err
	}
	return session, nil
}

type webauthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthnlib.Credential
}

func adaptUser(user User) (*webauthnUser, error) {
	id := strings.TrimSpace(user.ID)
	name := strings.TrimSpace(user.Name)
	if id == "" {
		return nil, errors.New("passkey user id is required")
	}
	if name == "" {
		return nil, errors.New("passkey username is required")
	}
	displayName := strings.TrimSpace(user.DisplayName)
	if displayName == "" {
		displayName = name
	}
	credentials := make([]webauthnlib.Credential, 0, len(user.Credentials))
	for _, stored := range user.Credentials {
		raw := strings.TrimSpace(stored.Data)
		if raw == "" {
			continue
		}
		var credential webauthnlib.Credential
		if err := json.Unmarshal([]byte(raw), &credential); err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	return &webauthnUser{
		id:          []byte(id),
		name:        name,
		displayName: displayName,
		credentials: credentials,
	}, nil
}

func (u *webauthnUser) WebAuthnID() []byte {
	return u.id
}

func (u *webauthnUser) WebAuthnName() string {
	return u.name
}

func (u *webauthnUser) WebAuthnDisplayName() string {
	return u.displayName
}

func (u *webauthnUser) WebAuthnCredentials() []webauthnlib.Credential {
	return append([]webauthnlib.Credential{}, u.credentials...)
}
