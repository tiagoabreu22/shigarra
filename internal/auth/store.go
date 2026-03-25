package auth

import (
	"fmt"
	"strings"
)

// CredentialStore is the interface for backend-agnostic credential storage.
type CredentialStore interface {
	Get(service, key string) (string, error)
	Set(service, key, value string) error
	Delete(service, key string) error
	Backend() string // "keyring" or "plaintext"
}

type StoreOption func(*storeConfig)

type storeConfig struct {
	appName      string
	forceBackend string
}

func defaultConfig(appName string) storeConfig {
	return storeConfig{appName: appName}
}

// WithForceBackend forces a specific backend ("keyring" or "plaintext").
// Primarily useful for testing.
func WithForceBackend(name string) StoreOption {
	return func(cfg *storeConfig) {
		cfg.forceBackend = strings.TrimSpace(strings.ToLower(name))
	}
}

// NewStore auto-detects the best available backend.
// Priority: system keyring → plaintext file (0600).
func NewStore(appName string, opts ...StoreOption) (CredentialStore, error) {
	cfg := defaultConfig(appName)
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.forceBackend != "" {
		return buildForcedStore(cfg)
	}

	if ks, err := newKeyringStore(cfg.appName); err == nil {
		return ks, nil
	}

	return newPlaintextStore(cfg.appName)
}

func buildForcedStore(cfg storeConfig) (CredentialStore, error) {
	switch cfg.forceBackend {
	case "keyring":
		return newKeyringStore(cfg.appName)
	case "plaintext":
		return newPlaintextStore(cfg.appName)
	default:
		return nil, fmt.Errorf("unsupported backend %q", cfg.forceBackend)
	}
}
