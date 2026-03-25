package auth

import "errors"

var (
	ErrNotFound    = errors.New("credential not found")
	ErrStoreLocked = errors.New("credential store is locked")
	ErrNoBackend   = errors.New("no suitable credential backend available")
	ErrInvalidData = errors.New("stored credential data is corrupted")
)
