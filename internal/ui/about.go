package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tiagoabreu22/shigarra/internal/config"
	"github.com/tiagoabreu22/shigarra/internal/updater"
)

type updateState int

const (
	updateStateIdle updateState = iota
	updateStateChecking
	updateStateUpToDate
	updateStateAvailable
	updateStateError
)

type aboutModel struct {
	version       string
	prefs         config.Prefs
	updateState   updateState
	latestVersion string
	errMsg        string
	updateCmd     string
}

func newAboutModel(version, installMethod string, prefs config.Prefs) aboutModel {
	return aboutModel{
		version:   version,
		prefs:     prefs,
		updateCmd: updater.UpdateCommand(installMethod),
	}
}

func (m aboutModel) Update(msg tea.Msg) (aboutModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ", "enter":
			m.prefs.CheckUpdates = !m.prefs.CheckUpdates
			if !m.prefs.CheckUpdates {
				m.updateState = updateStateIdle
				m.latestVersion = ""
			}
		}
	}
	return m, nil
}

func (m aboutModel) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render("About")
	b.WriteString(title + "\n\n")

	ver := lipgloss.NewStyle().Foreground(cFgBright).Bold(true).Render("shigarra " + m.version)
	repo := lipgloss.NewStyle().Foreground(cFgDim).Render("github.com/tiagoabreu22/shigarra")
	b.WriteString(ver + "\n" + repo + "\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", 36)) + "\n\n")

	prefix := "▶ "
	labelStyle := lipgloss.NewStyle().Foreground(cFg).Bold(true)
	var badge string
	if m.prefs.CheckUpdates {
		badge = lipgloss.NewStyle().Foreground(cGreen).Render("[on] ")
	} else {
		badge = lipgloss.NewStyle().Foreground(cFgMuted).Render("[off]")
	}
	b.WriteString(labelStyle.Render(prefix+"Check for updates on startup") + "  " + badge + "\n")

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", 36)) + "\n\n")

	b.WriteString(m.updateStatusLine() + "\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(cFgMuted).Render("space · toggle   esc · close"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cAccent).
		Padding(1, 3).
		Render(b.String())
}

func (m aboutModel) updateStatusLine() string {
	switch m.updateState {
	case updateStateIdle:
		return lipgloss.NewStyle().Foreground(cFgMuted).Render("Updates: not checked")
	case updateStateChecking:
		return lipgloss.NewStyle().Foreground(cFgDim).Render("Updates: checking…")
	case updateStateUpToDate:
		return lipgloss.NewStyle().Foreground(cGreen).Render("Updates: up to date")
	case updateStateAvailable:
		cmd := lipgloss.NewStyle().Foreground(cFgBright).Render(m.updateCmd)
		return lipgloss.NewStyle().Foreground(cYellow).Render("v"+m.latestVersion+" available  ") + cmd
	case updateStateError:
		return lipgloss.NewStyle().Foreground(cRed).Render("Updates: " + m.errMsg)
	default:
		return ""
	}
}
