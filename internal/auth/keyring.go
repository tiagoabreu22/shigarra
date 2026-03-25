package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

type keyringStore struct {
	appName string
}

func newKeyringStore(appName string) (*keyringStore, error) {
	s := &keyringStore{appName: appName}
	if err := s.probe(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *keyringStore) probe() error {
	_, err := keyring.Get(s.serviceName("probe"), "probe")
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (s *keyringStore) Get(service, key string) (string, error) {
	v, err := keyring.Get(s.serviceName(service), key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return v, nil
}

func (s *keyringStore) Set(service, key, value string) error {
	return keyring.Set(s.serviceName(service), key, value)
}

func (s *keyringStore) Delete(service, key string) error {
	err := keyring.Delete(s.serviceName(service), key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (s *keyringStore) Backend() string { return "keyring" }

func (s *keyringStore) serviceName(service string) string {
	svc := strings.TrimSpace(service)
	if svc == "" {
		svc = "default"
	}
	return fmt.Sprintf("%s.%s", s.appName, svc)
}
