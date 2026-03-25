package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tiagoabreu22/shigarra/internal/auth"
	"github.com/tiagoabreu22/shigarra/internal/config"
)

type authSetupModel struct {
	width    int
	height   int
	filePath string
	errMsg   string
}

type authSetupCompleteMsg struct {
	session        *config.Session
	sessionManager *auth.SessionManager
}

type authSetupErrMsg struct{ err error }

func newAuthSetupModel() authSetupModel {
	// Compute the file path for display purposes.
	d, _ := os.UserConfigDir()
	fp := filepath.Join(d, "shigarra", "credentials.json")
	return authSetupModel{filePath: fp}
}

func (m authSetupModel) Init() tea.Cmd { return nil }

func (m authSetupModel) Update(msg tea.Msg) (authSetupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case authSetupErrMsg:
		m.errMsg = msg.err.Error()

	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			return m, m.configure()
		}
	}
	return m, nil
}

func (m authSetupModel) configure() tea.Cmd {
	return func() tea.Msg {
		sess, err := config.Load()
		if err != nil {
			return authSetupErrMsg{err}
		}
		if sess == nil {
			sess = &config.Session{}
		}
		sess.AuthBackend = "plaintext"
		sess.AuthConfigured = true
		if err := config.Save(sess); err != nil {
			return authSetupErrMsg{err}
		}
		sm, err := auth.NewSessionManager("plaintext")
		if err != nil {
			return authSetupErrMsg{err}
		}
		return authSetupCompleteMsg{session: sess, sessionManager: sm}
	}
}

func (m authSetupModel) View() string {
	var b strings.Builder

	header := lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render("Credential Storage")
	b.WriteString(header + "\n\n")

	notice := lipgloss.NewStyle().Foreground(cFg).Render(
		"System keychain is not available on this machine.\n\n" +
			"Your session cookies and password will be stored in:\n",
	)
	b.WriteString(notice)

	pathStyle := lipgloss.NewStyle().Foreground(cFgBright).Bold(true)
	b.WriteString(pathStyle.Render("  "+m.filePath) + "\n\n")

	desc := lipgloss.NewStyle().Foreground(cFgDim).Render(
		"The file will have mode 0600 (owner-read only),\n" +
			"the same protection used by SSH private keys and\n" +
			"git's credential store. Full-disk encryption provides\n" +
			"additional protection if you need it.",
	)
	b.WriteString(desc + "\n\n")

	if m.errMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(cRed).Render("✗ "+m.errMsg) + "\n\n")
	}

	b.WriteString(lipgloss.NewStyle().Foreground(cFgMuted).Render("Press Enter to continue"))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cSurface).
		Padding(1, 2).
		Width(64).
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
