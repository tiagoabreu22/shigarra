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
	fieldFaculty  loginField = iota
	fieldUsername            // maps to inputs[0]
	fieldPassword            // maps to inputs[1]
	fieldCount
)

type loginModel struct {
	faculty   facultyPicker
	inputs    [2]textinput.Model // [0]=username, [1]=password
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

	m := loginModel{
		faculty: FacultyPicker(""),
		inputs:  [2]textinput.Model{username, password},
		focused: fieldFaculty,
		spinner: sp,
	}
	m.faculty = m.faculty.Focus()
	return m
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
		if m.focused == fieldFaculty {
			var cmd tea.Cmd
			var nav int
			m.faculty, cmd, nav = m.faculty.handleKey(msg)
			switch nav {
			case 1:
				m = m.nextField()
			case -1:
				m = m.prevField()
			}
			return m, cmd
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

	// Pass all non-key messages through inputs for cursor blink etc.
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.faculty.input, cmd = m.faculty.input.Update(msg)
	cmds = append(cmds, cmd)
	for i := range m.inputs {
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m loginModel) nextField() loginModel {
	if m.focused == fieldFaculty {
		m.faculty = m.faculty.Blur()
	} else {
		m.inputs[m.focused-1].Blur()
	}
	m.focused = (m.focused + 1) % fieldCount
	if m.focused == fieldFaculty {
		m.faculty = m.faculty.Focus()
	} else {
		m.inputs[m.focused-1].Focus()
	}
	return m
}

func (m loginModel) prevField() loginModel {
	if m.focused == fieldFaculty {
		m.faculty = m.faculty.Blur()
	} else {
		m.inputs[m.focused-1].Blur()
	}
	if m.focused == 0 {
		m.focused = fieldCount - 1
	} else {
		m.focused--
	}
	if m.focused == fieldFaculty {
		m.faculty = m.faculty.Focus()
	} else {
		m.inputs[m.focused-1].Focus()
	}
	return m
}

func (m loginModel) submit() (loginModel, tea.Cmd) {
	faculty := m.faculty.Value()
	username := strings.TrimSpace(m.inputs[0].Value())
	password := m.inputs[1].Value()

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
	labelStyle := lipgloss.NewStyle().Foreground(cFgDim).MarginBottom(0)

	dropdownOpen := m.focused == fieldFaculty && m.faculty.open && len(m.faculty.matches) > 0

	inputBox := func(inp textinput.Model, focused bool) string {
		bc := cSurface
		if focused {
			bc = cAccent
		}
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bc).
			Padding(0, 1).
			Width(loginBoxW).
			Render(inp.View())
	}

	var fields strings.Builder

	fields.WriteString(labelStyle.Render("Faculty") + "\n")
	fields.WriteString(m.faculty.inputView(m.focused == fieldFaculty) + "\n")

	if dropdownOpen {
		fields.WriteString(m.faculty.dropdownView() + "\n")
	} else {
		fields.WriteString("\n")
		fields.WriteString(labelStyle.Render("Username") + "\n")
		fields.WriteString(inputBox(m.inputs[0], m.focused == fieldUsername) + "\n")
	}

	fields.WriteString("\n")
	fields.WriteString(labelStyle.Render("Password") + "\n")
	fields.WriteString(inputBox(m.inputs[1], m.focused == fieldPassword) + "\n")

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
