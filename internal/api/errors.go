package api

import (
	"errors"
	"net/http"
	"strings"
)

// common API errors
var (
	// ErrSessionExpired indicates the SIGARRA session has expired (HTTP 403 or redirect to login).
	ErrSessionExpired = errors.New("session expired")

	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
)

// IsSessionExpired checks if an HTTP response indicates a session expiry.
// Returns true for:
// - HTTP 403 (Forbidden)
// - HTTP 401 (Unauthorized)
// - Redirects to login pages
// - SIGARRA-specific error indicators
func IsSessionExpired(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	// check status code
	if resp.StatusCode == http.StatusForbidden {
		return true
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return true
	}

	// check for redirects to login page
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if strings.Contains(location, "/login") || 
		   strings.Contains(location, "/authenticate") ||
		   strings.Contains(location, "mob_val_geral") {
			return true
		}
	}

	// check URL for login redirects (even with 200 OK)
	if resp.Request != nil && resp.Request.URL != nil {
		url := resp.Request.URL.String()
		if strings.Contains(url, "/login") || 
		   strings.Contains(url, "/authenticate") ||
		   strings.Contains(url, "mob_val_geral") {
			return true
		}
	}

	return false
}

// CheckSessionExpired checks the response and returns ErrSessionExpired if needed.
func CheckSessionExpired(resp *http.Response) error {
	if IsSessionExpired(resp) {
		return ErrSessionExpired
	}
	return nil
}
