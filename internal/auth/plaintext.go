package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type plaintextStore struct {
	filePath string
	data     map[string]string
	mu       sync.RWMutex
}

func newPlaintextStore(appName string) (*plaintextStore, error) {
	path, err := plaintextFilePath(appName)
	if err != nil {
		return nil, err
	}
	s := &plaintextStore{filePath: path, data: make(map[string]string)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func plaintextFilePath(appName string) (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, appName, "credentials.json"), nil
}

func (p *plaintextStore) load() error {
	dir := filepath.Dir(p.filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	b, err := os.ReadFile(p.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, &p.data)
}

func (p *plaintextStore) persist() error {
	b, err := json.MarshalIndent(p.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.filePath, b, 0o600)
}

func (p *plaintextStore) Get(service, key string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.data[p.compositeKey(service, key)]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (p *plaintextStore) Set(service, key, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data[p.compositeKey(service, key)] = value
	return p.persist()
}

func (p *plaintextStore) Delete(service, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.data, p.compositeKey(service, key))
	return p.persist()
}

func (p *plaintextStore) Backend() string { return "plaintext" }

func (p *plaintextStore) FilePath() string { return p.filePath }

func (p *plaintextStore) compositeKey(service, key string) string {
	return fmt.Sprintf("%s::%s", service, key)
}
