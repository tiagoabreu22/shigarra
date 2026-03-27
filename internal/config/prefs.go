package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Prefs struct {
	CheckUpdates bool `json:"check_updates"`
}

func DefaultPrefs() Prefs {
	return Prefs{CheckUpdates: true}
}

func prefsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shigarra", "prefs.json"), nil
}

func LoadPrefs() (*Prefs, error) {
	path, err := prefsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		p := DefaultPrefs()
		return &p, nil
	}
	if err != nil {
		return nil, err
	}
	var p Prefs
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func SavePrefs(p *Prefs) error {
	path, err := prefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
