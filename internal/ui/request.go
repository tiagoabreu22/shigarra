package ui

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tiagoabreu22/shigarra/internal/api"
)

// sessionExpiredMsg signals that the SIGARRA session has expired and re-login is needed.
type sessionExpiredMsg struct{}

func friendlyRequestError(action string, err error) (error, tea.Cmd) {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s timed out after %s", action, api.RequestTimeout), nil
	}
	if errors.Is(err, api.ErrSessionExpired) {
		return fmt.Errorf("session expired — please log in again"), func() tea.Msg {
			return sessionExpiredMsg{}
		}
	}
	return err, nil
}
