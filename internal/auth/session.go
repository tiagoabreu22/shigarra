package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	defaultAppName = "shigarra"
	sessionService = "session"
)

type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SessionSecrets struct {
	Cookies []Cookie `json:"cookies"`
}

// SessionManager wraps a CredentialStore and handles the session lifecycle:
// storing/loading cookies and password-based auto-refresh.
type SessionManager struct {
	store   CredentialStore
	warning string
}

// NewSessionManager creates a SessionManager using the given backend.
// If forceBackend is empty, the best available backend is auto-detected.
// Priority: system keyring,  file 
func NewSessionManager(forceBackend string) (*SessionManager, error) {
	var opts []StoreOption
	if forceBackend != "" {
		opts = append(opts, WithForceBackend(forceBackend))
	}

	store, err := NewStore(defaultAppName, opts...)
	if err != nil {
		return nil, err
	}

	m := &SessionManager{store: store}

	if store.Backend() == "plaintext" {
		if ps, ok := store.(*plaintextStore); ok {
			m.warning = fmt.Sprintf(
				"credentials stored in %s (0600) — like SSH keys",
				ps.FilePath(),
			)
		} else {
			m.warning = "credentials stored in a file (0600)"
		}
	}

	return m, nil
}

func (m *SessionManager) Backend() string {
	if m == nil || m.store == nil {
		return ""
	}
	return m.store.Backend()
}

// Warning returns a one-line notice when the plaintext backend is active.
func (m *SessionManager) Warning() string {
	if m == nil {
		return ""
	}
	return m.warning
}

func (m *SessionManager) LoadSessionSecrets(faculty, username string) (*SessionSecrets, error) {
	if strings.TrimSpace(faculty) == "" || strings.TrimSpace(username) == "" {
		return nil, nil
	}
	v, err := m.store.Get(sessionService, sessionKey(faculty, username))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var s SessionSecrets
	if err := json.Unmarshal([]byte(v), &s); err != nil {
		return nil, ErrInvalidData
	}
	return &s, nil
}

func (m *SessionManager) SaveSessionSecrets(faculty, username string, s SessionSecrets) error {
	if strings.TrimSpace(faculty) == "" || strings.TrimSpace(username) == "" {
		return fmt.Errorf("faculty and username are required")
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return m.store.Set(sessionService, sessionKey(faculty, username), string(b))
}

func (m *SessionManager) DeleteSessionSecrets(faculty, username string) error {
	if strings.TrimSpace(faculty) == "" || strings.TrimSpace(username) == "" {
		return nil
	}
	if err := m.store.Delete(sessionService, sessionKey(faculty, username)); err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

// SavePassword stores the user's password for auto-refresh.
func (m *SessionManager) SavePassword(faculty, username, password string) error {
	if strings.TrimSpace(faculty) == "" || strings.TrimSpace(username) == "" {
		return fmt.Errorf("faculty and username are required")
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("password cannot be empty")
	}
	return m.store.Set("password", passwordKey(faculty, username), password)
}

// GetPassword retrieves the stored password. Returns ErrNotFound if none is stored.
func (m *SessionManager) GetPassword(faculty, username string) (string, error) {
	if strings.TrimSpace(faculty) == "" || strings.TrimSpace(username) == "" {
		return "", fmt.Errorf("faculty and username are required")
	}
	return m.store.Get("password", passwordKey(faculty, username))
}

func (m *SessionManager) DeletePassword(faculty, username string) error {
	if strings.TrimSpace(faculty) == "" || strings.TrimSpace(username) == "" {
		return nil
	}
	if err := m.store.Delete("password", passwordKey(faculty, username)); err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

func (m *SessionManager) HasStoredPassword(faculty, username string) bool {
	_, err := m.GetPassword(faculty, username)
	return err == nil
}

// TryAutoRefresh silently reauthenticates using the stored password.
// loginFn should be api.Login. Returns an error if no password is stored or
// if login fails — the caller should then show the login screen.
func (m *SessionManager) TryAutoRefresh(
	ctx context.Context,
	faculty, username string,
	loginFn func(ctx context.Context, faculty, username, password string) ([]*http.Cookie, error),
) error {
	password, err := m.GetPassword(faculty, username)
	if err != nil {
		return err
	}
	cookies, err := loginFn(ctx, faculty, username, password)
	if err != nil {
		return err
	}
	return m.SaveSessionSecrets(faculty, username, SessionSecrets{
		Cookies: CookiesFromHTTP(cookies),
	})
}

func sessionKey(faculty, username string) string {
	return strings.ToLower(strings.TrimSpace(faculty)) + ":" + strings.ToLower(strings.TrimSpace(username))
}

func passwordKey(faculty, username string) string {
	return strings.ToLower(strings.TrimSpace(faculty)) + ":" + strings.ToLower(strings.TrimSpace(username))
}

func CookiesFromHTTP(cookies []*http.Cookie) []Cookie {
	out := make([]Cookie, 0, len(cookies))
	for _, c := range cookies {
		if c == nil {
			continue
		}
		out = append(out, Cookie{Name: c.Name, Value: c.Value})
	}
	return out
}

func CookiesToHTTP(cookies []Cookie) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		out = append(out, &http.Cookie{Name: c.Name, Value: c.Value})
	}
	return out
}
