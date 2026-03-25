package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type savePasswordModel struct {
	width    int
	height   int
	backend  string // "keyring" or "plaintext"
	selected int    // 0 = yes, 1 = no
}

type savePasswordDecisionMsg struct {
	save bool
}

func newSavePasswordModel(backend string) savePasswordModel {
	return savePasswordModel{backend: backend}
}

func (m savePasswordModel) Init() tea.Cmd { return nil }

func (m savePasswordModel) Update(msg tea.Msg) (savePasswordModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < 1 {
				m.selected++
			}
		case "y", "Y":
			return m, func() tea.Msg { return savePasswordDecisionMsg{save: true} }
		case "n", "N", "esc":
			return m, func() tea.Msg { return savePasswordDecisionMsg{save: false} }
		case "enter":
			return m, func() tea.Msg { return savePasswordDecisionMsg{save: m.selected == 0} }
		}
	}
	return m, nil
}

func (m savePasswordModel) View() string {
	var b strings.Builder

	header := lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render("Save password?")
	b.WriteString(header + "\n\n")

	storageDesc := "your system keychain"
	if m.backend == "plaintext" {
		storageDesc = "~/.config/shigarra/credentials.json (0600)"
	}

	desc := lipgloss.NewStyle().Foreground(cFg).Render(
		"Save your password\n" +
			"so shigarra can refresh it automatically?\n\n" +
			"Stored in: " + storageDesc,
	)
	b.WriteString(desc + "\n\n")

	options := []string{"Yes, save it", "No, ask me next time"}
	for i, opt := range options {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(cFg)
		if i == m.selected {
			prefix = "▶ "
			style = style.Foreground(cFgBright).Bold(true)
		}
		b.WriteString(style.Render(prefix+opt) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cFgMuted).Render("↑/↓ · Enter confirm · y yes · n/Esc no"))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cSurface).
		Padding(1, 2).
		Width(56).
		Render(b.String())

	w, h := m.width, m.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, card)
}
