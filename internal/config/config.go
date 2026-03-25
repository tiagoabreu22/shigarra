package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Session holds persisted login metadata. Secrets (cookies, password) are
// stored separately by the SessionManager in the credential backend.
type Session struct {
	Faculty        string `json:"faculty"`
	Username       string `json:"username"`
	AuthBackend    string `json:"auth_backend,omitempty"`
	AuthConfigured bool   `json:"auth_configured,omitempty"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shigarra", "session.json"), nil
}

// Load reads the session from ~/.config/shigarra/session.json.
// Returns nil, nil when no file exists.
func Load() (*Session, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save writes the session metadata to disk with 0600 permissions.
func Save(s *Session) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func Clear() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func ResolveAuthBackend(s *Session) string {
	if s == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(s.AuthBackend))
}

func AuthIsConfigured(s *Session) bool {
	if s == nil {
		return false
	}
	return s.AuthConfigured
}
