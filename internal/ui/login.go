package ui

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tiagoabreu22/shigarra/internal/api"
	"github.com/tiagoabreu22/shigarra/internal/config"
)

// Login card dimensions.
// card Width() = 48
// input box Width(40).Padding(0,1).Border keeps fields roomy but compact.
const (
	loginCardW  = 48
	loginBoxW   = 40
	loginInputW = 38
)

type loginField int

const (
	fieldFaculty loginField = iota
	fieldUsername
	fieldPassword
	fieldCount
)

type loginModel struct {
	inputs    [fieldCount]textinput.Model
	focused   loginField
	spinner   spinner.Model
	loading   bool
	errMsg    string
	width     int
	height    int
	cancel    context.CancelFunc
	requestID int
}

type loginSuccessMsg struct {
	session   *config.Session
	cookies   []*http.Cookie
	password  string // retained for optional password storage prompt
	requestID int
}

type loginErrMsg struct {
	err       error
	requestID int
}

func newLoginModel() loginModel {
	faculty := textinput.New()
	faculty.Placeholder = "fcup"
	faculty.SetValue("fcup")
	faculty.Width = loginInputW
	faculty.Focus()
	faculty.CharLimit = 20

	username := textinput.New()
	username.Placeholder = "upXXXXXXX"
	username.Width = loginInputW
	username.CharLimit = 20

	password := textinput.New()
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'
	password.Width = loginInputW
	password.CharLimit = 64

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cAccent)

	return loginModel{
		inputs:  [fieldCount]textinput.Model{faculty, username, password},
		focused: fieldFaculty,
		spinner: sp,
	}
}

func (m *loginModel) Cancel() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m loginModel) Init() tea.Cmd { return textinput.Blink }

func (m loginModel) Update(msg tea.Msg) (loginModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "tab", "down":
			m = m.nextField()
		case "shift+tab", "up":
			m = m.prevField()
		case "enter":
			if m.focused == fieldCount-1 {
				return m.submit()
			}
			m = m.nextField()
		}

	case loginSuccessMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}
		m.Cancel()
		m.loading = false
		return m, nil

	case loginErrMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}
		m.Cancel()
		m.loading = false
		if errors.Is(msg.err, context.Canceled) {
			m.errMsg = ""
			return m, nil
		}
		m.errMsg = msg.err.Error()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	var cmds []tea.Cmd
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m loginModel) nextField() loginModel {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused + 1) % fieldCount
	m.inputs[m.focused].Focus()
	return m
}

func (m loginModel) prevField() loginModel {
	m.inputs[m.focused].Blur()
	if m.focused == 0 {
		m.focused = fieldCount - 1
	} else {
		m.focused--
	}
	m.inputs[m.focused].Focus()
	return m
}

func (m loginModel) submit() (loginModel, tea.Cmd) {
	faculty := strings.TrimSpace(m.inputs[fieldFaculty].Value())
	username := strings.TrimSpace(m.inputs[fieldUsername].Value())
	password := m.inputs[fieldPassword].Value()

	if faculty == "" || username == "" || password == "" {
		m.errMsg = "All fields are required"
		return m, nil
	}

	m.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), api.RequestTimeout)
	m.cancel = cancel
	m.requestID++
	requestID := m.requestID

	m.loading = true
	m.errMsg = ""
	return m, tea.Batch(m.spinner.Tick, doLogin(ctx, requestID, faculty, username, password))
}

func doLogin(ctx context.Context, requestID int, faculty, username, password string) tea.Cmd {
	return func() tea.Msg {
		cookies, err := api.Login(ctx, faculty, username, password)
		if err != nil {
			friendlyErr, extraCmd := friendlyRequestError("sign in", err)
			if extraCmd != nil {
				return tea.Batch(
					func() tea.Msg { return loginErrMsg{err: friendlyErr, requestID: requestID} },
					extraCmd,
				)()
			}
			return loginErrMsg{err: friendlyErr, requestID: requestID}
		}

		sess := &config.Session{Faculty: faculty, Username: username}
		return loginSuccessMsg{
			session:   sess,
			cookies:   cookies,
			password:  password,
			requestID: requestID,
		}
	}
}

func (m loginModel) View() string {
	// ── Inputs ────────────────────────────────────────────────────────────
	labels := []string{"Faculty", "Username", "Password"}
	labelStyle := lipgloss.NewStyle().Foreground(cFgDim).MarginBottom(0)

	var fields strings.Builder
	for i, input := range m.inputs {
		fields.WriteString(labelStyle.Render(labels[i]) + "\n")

		borderColor := cSurface
		if loginField(i) == m.focused {
			borderColor = cAccent
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			Width(loginBoxW).
			Render(input.View())

		fields.WriteString(box + "\n")
		if i < int(fieldCount)-1 {
			fields.WriteString("\n")
		}
	}

	// ── Status line ───────────────────────────────────────────────────────
	var status string
	switch {
	case m.loading:
		status = m.spinner.View() + " " +
			lipgloss.NewStyle().Foreground(cFgDim).Render("Signing in…")
	case m.errMsg != "":
		status = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cRed).
			Foreground(cRed).
			Padding(0, 1).
			Render("✗  " + m.errMsg)
	default:
		status = lipgloss.NewStyle().Foreground(cFgMuted).
			Render("↑↓/tab · navigate  |  enter · sign in")
	}
	divider := lipgloss.NewStyle().
		Foreground(cBorder).
		Render(strings.Repeat("─", loginBoxW))

	// ── Card ──────────────────────────────────────────────────────────────
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cSurface).
		Padding(1, 3).
		Width(loginCardW).
		Render(fields.String() + "\n" + divider + "\n" + status)

	art := shigarraArt()
	sub := lipgloss.NewStyle().Foreground(cFgDim).Render("by tiagoabreu22")
	content := lipgloss.JoinVertical(lipgloss.Center, art, sub, "", card)

	w, h := m.width, m.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
