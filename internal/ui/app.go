package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tiagoabreu22/shigarra/internal/api"
	"github.com/tiagoabreu22/shigarra/internal/auth"
	"github.com/tiagoabreu22/shigarra/internal/config"
	"github.com/tiagoabreu22/shigarra/internal/updater"
)

// ── Key bindings ─────────────────────────────────────────────────────────────

var (
	keyLeft = key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/→", "days"),
	)
	keyRight = key.NewBinding(key.WithKeys("right", "l"))

	keyUp = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/↓", "scroll"),
	)
	keyDown = key.NewBinding(key.WithKeys("down", "j"))

	keyTab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch view"),
	)
	keyRefresh = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	)
	keyLogout = key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "logout"),
	)
	keyHelp = key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	)
	keyQuit = key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	)
	keyAbout = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "about"),
	)
)

type updateCheckResultMsg struct {
	result updater.Result
	err    error
}


type schedKeyMap struct{}

func (k schedKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keyLeft, keyTab, keyRefresh, keyLogout, keyAbout, keyHelp, keyQuit}
}
func (k schedKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyLeft, keyRight},
		{keyTab, keyRefresh, keyLogout},
		{keyAbout, keyHelp, keyQuit},
	}
}

type examsKeyMap struct{}

func (k examsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keyUp, keyTab, keyRefresh, keyLogout, keyAbout, keyHelp, keyQuit}
}
func (k examsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyUp, keyDown},
		{keyTab, keyRefresh, keyLogout},
		{keyAbout, keyHelp, keyQuit},
	}
}

type weekKeyMap struct{}

func (k weekKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{keyLeft, keyTab, keyRefresh, keyLogout, keyAbout, keyHelp, keyQuit}
}
func (k weekKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyLeft, keyRight},
		{keyTab, keyRefresh, keyLogout},
		{keyAbout, keyHelp, keyQuit},
	}
}

// ── App model ─────────────────────────────────────────────────────────────────

type screen int

const (
	screenAuthSetup screen = iota
	screenLogin
	screenSavePassword
	screenSchedule
	screenWeek
	screenExams
)

// App is the root Bubble Tea model
type App struct {
	screen         screen
	authSetup      authSetupModel
	login          loginModel
	savePassword   savePasswordModel
	schedule       scheduleModel
	exams          examsModel
	session        *config.Session
	client         *api.Client
	sessionManager *auth.SessionManager
	help           help.Model
	showHelp       bool
	about          aboutModel
	showAbout      bool
	prefs          config.Prefs
	updateAvail    string
	width          int
	height         int

	pendingPassword string
}

const navSidebarW = 16

func NewApp(version, installMethod string) App {
	h := help.New()

	prefs, err := config.LoadPrefs()
	if err != nil || prefs == nil {
		p := config.DefaultPrefs()
		prefs = &p
	}

	app := App{
		screen:    screenLogin,
		authSetup: newAuthSetupModel(),
		login:     newLoginModel(),
		schedule:  newScheduleModel(nil, ""),
		exams:     newExamsModel(nil, ""),
		help:      h,
		prefs:     *prefs,
		about:     newAboutModel(version, installMethod, *prefs),
	}

	sess, err := config.Load()
	if err != nil {
		sess = &config.Session{}
	} else if sess == nil {
		sess = &config.Session{}
	}
	app.session = sess

	// Auto-configure keyring if available and not yet configured
	if !config.AuthIsConfigured(sess) {
		if app.tryAutoConfigureKeyring(sess) {
			if err := app.initSessionManager(sess); err != nil {
				
				app.screen = screenAuthSetup
				return app
			}
			app.tryRestoreLoggedInScreen()
			return app
		}
		// Keyring not available — show the file-storage notice screen
		app.screen = screenAuthSetup
		return app
	}

	if err := app.initSessionManager(sess); err != nil {
		
		app.screen = screenAuthSetup
		return app
	}

	app.tryRestoreLoggedInScreen()
	return app
}

// tryAutoConfigureKeyring silently configures the keyring backend when available
func (a *App) tryAutoConfigureKeyring(sess *config.Session) bool {
	testStore, err := auth.NewStore("shigarra", auth.WithForceBackend("keyring"))
	if err != nil || testStore.Backend() != "keyring" {
		return false
	}
	sess.AuthBackend = "keyring"
	sess.AuthConfigured = true
	if err := config.Save(sess); err != nil {
		return false
	}
	return true
}

func (a *App) initSessionManager(sess *config.Session) error {
	sessionManager, err := auth.NewSessionManager(config.ResolveAuthBackend(sess))
	if err != nil {
		return fmt.Errorf("credential setup failed: %w", err)
	}
	a.sessionManager = sessionManager

	return nil
}

func (a *App) tryRestoreLoggedInScreen() {
	sess := a.session
	if sess == nil || a.sessionManager == nil {
		return
	}
	if strings.TrimSpace(sess.Faculty) == "" || strings.TrimSpace(sess.Username) == "" {
		return
	}
	secrets, err := a.sessionManager.LoadSessionSecrets(sess.Faculty, sess.Username)
	if err != nil || secrets == nil {
		return
	}
	client, err := api.NewClient(sess.Faculty, auth.CookiesToHTTP(secrets.Cookies))
	if err != nil {
		return
	}
	a.client = client
	a.screen = screenSchedule
	a.schedule = newScheduleModel(client, sess.Username)
	a.exams = newExamsModel(client, sess.Username)
}

func Run(version, installMethod string) error {
	p := tea.NewProgram(NewApp(version, installMethod), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (a App) Init() tea.Cmd {
	var cmds []tea.Cmd
	switch a.screen {
	case screenAuthSetup:
		cmds = append(cmds, a.authSetup.Init())
	case screenLogin:
		cmds = append(cmds, a.login.Init())
	case screenSchedule, screenWeek:
		cmds = append(cmds, tea.Batch(a.schedule.Init(), a.exams.Init()))
	}
	if a.prefs.CheckUpdates {
		cmds = append(cmds, checkUpdateCmd(a.about.version))
	}
	return tea.Batch(cmds...)
}

func checkUpdateCmd(version string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result, err := updater.CheckLatest(ctx, version)
		return updateCheckResultMsg{result: result, err: err}
	}
}


func (a *App) cancelOutstanding() {
	a.login.Cancel()
	a.schedule.Cancel()
	a.exams.Cancel()
}

func (a *App) doLogout() tea.Cmd {
	a.cancelOutstanding()
	if a.sessionManager != nil && a.session != nil {
		_ = a.sessionManager.DeleteSessionSecrets(a.session.Faculty, a.session.Username)
		_ = a.sessionManager.DeletePassword(a.session.Faculty, a.session.Username)
	}
	_ = config.Clear()
	a.session = &config.Session{}
	a.client = nil
	a.pendingPassword = ""
	a.screen = screenLogin
	a.login = newLoginModel()
	a.login, _ = a.login.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	return a.login.Init()
}

// autoRefreshResultMsg is the result of a background session auto-refresh
type autoRefreshResultMsg struct {
	cookies []auth.Cookie
	err     error
}

func tryAutoRefreshCmd(faculty, username string, sm *auth.SessionManager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), api.RequestTimeout)
		defer cancel()
		err := sm.TryAutoRefresh(ctx, faculty, username, api.Login)
		if err != nil {
			return autoRefreshResultMsg{err: err}
		}
		secrets, loadErr := sm.LoadSessionSecrets(faculty, username)
		if loadErr != nil || secrets == nil {
			return autoRefreshResultMsg{err: fmt.Errorf("auto-refresh succeeded but could not reload cookies")}
		}
		return autoRefreshResultMsg{cookies: secrets.Cookies}
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width

		a.authSetup, _ = a.authSetup.Update(msg)
		a.login, _ = a.login.Update(msg)
		a.savePassword, _ = a.savePassword.Update(msg)

		subW := msg.Width
		if a.useSidebar(msg.Width, msg.Height) {
			subW -= navSidebarW
		}
		if subW < 40 {
			subW = 40
		}
		subMsg := tea.WindowSizeMsg{Width: subW, Height: msg.Height}
		a.schedule, _ = a.schedule.Update(subMsg)
		a.exams, _ = a.exams.Update(subMsg)
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			a.cancelOutstanding()
			return a, tea.Quit
		}

		if a.showAbout {
			switch msg.String() {
			case "a", "esc":
				a.showAbout = false
			default:
				var cmd tea.Cmd
				a.about, cmd = a.about.Update(msg)
				if a.about.prefs != a.prefs {
					a.prefs = a.about.prefs
					_ = config.SavePrefs(&a.prefs)
				}
				return a, cmd
			}
			return a, nil
		}

		switch msg.String() {
		case "q":
			if a.screen == screenLogin && a.login.focused == fieldFaculty {
				a.cancelOutstanding()
				return a, tea.Quit
			}
			if a.screen != screenLogin && a.screen != screenAuthSetup && a.screen != screenSavePassword {
				a.cancelOutstanding()
				return a, tea.Quit
			}
		case "esc":
			if a.screen == screenLogin && a.login.focused == fieldFaculty && !a.login.faculty.open {
				a.cancelOutstanding()
				return a, tea.Quit
			}
		case "tab":
			switch a.screen {
			case screenSchedule:
				a.screen = screenWeek
				return a, nil
			case screenWeek:
				a.screen = screenExams
				return a, a.exams.Init()
			case screenExams:
				a.screen = screenSchedule
				return a, a.schedule.Init()
			}
		case "L":
			if a.screen != screenLogin && a.screen != screenAuthSetup && a.screen != screenSavePassword {
				return a, a.doLogout()
			}
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		case "a":
			if a.screen != screenLogin && a.screen != screenAuthSetup && a.screen != screenSavePassword {
				a.showAbout = true
				return a, nil
			}
		}

	case authSetupCompleteMsg:
		a.session = msg.session
		a.sessionManager = msg.sessionManager
		a.screen = screenLogin
		return a, a.login.Init()

	case authSetupErrMsg:
		var cmd tea.Cmd
		a.authSetup, cmd = a.authSetup.Update(msg)
		return a, cmd

	case loginSuccessMsg:
		a.login, _ = a.login.Update(msg)
		sess := msg.session
		if a.sessionManager == nil {
			a.login.errMsg = "No secure credential backend is available"
			a.login.loading = false
			return a, nil
		}

		if err := a.sessionManager.SaveSessionSecrets(sess.Faculty, sess.Username, auth.SessionSecrets{
			Cookies: auth.CookiesFromHTTP(msg.cookies),
		}); err != nil {
			a.login.errMsg = fmt.Sprintf("failed to store credentials: %v", err)
			a.login.loading = false
			return a, nil
		}

		sess.AuthBackend = a.session.AuthBackend
		sess.AuthConfigured = true
		if err := config.Save(sess); err != nil {
			a.login.errMsg = "Failed to save session metadata: " + err.Error()
			a.login.loading = false
			return a, nil
		}

		client, err := api.NewClient(sess.Faculty, msg.cookies)
		if err != nil {
			a.login.errMsg = "Failed to create API client: " + err.Error()
			a.login.loading = false
			return a, nil
		}

		a.session = sess
		a.client = client

		// Offer to save password for auto-refresh if not already stored.
		if msg.password != "" && !a.sessionManager.HasStoredPassword(sess.Faculty, sess.Username) {
			a.pendingPassword = msg.password
			a.savePassword = newSavePasswordModel(a.sessionManager.Backend())
			a.savePassword, _ = a.savePassword.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
			a.screen = screenSavePassword
			return a, a.savePassword.Init()
		}

		a.screen = screenSchedule
		a.schedule = newScheduleModel(client, sess.Username)
		a.exams = newExamsModel(client, sess.Username)
		return a, tea.Batch(a.schedule.Init(), a.exams.Init())

	case savePasswordDecisionMsg:
		if msg.save && a.pendingPassword != "" && a.session != nil && a.sessionManager != nil {
			_ = a.sessionManager.SavePassword(a.session.Faculty, a.session.Username, a.pendingPassword)
		}
		a.pendingPassword = "" // clear from memory
		a.screen = screenSchedule
		a.schedule = newScheduleModel(a.client, a.session.Username)
		a.exams = newExamsModel(a.client, a.session.Username)
		return a, tea.Batch(a.schedule.Init(), a.exams.Init())

	case loginErrMsg:
		var cmd tea.Cmd
		a.login, cmd = a.login.Update(msg)
		return a, cmd

	case sessionExpiredMsg:
		a.cancelOutstanding()
		a.client = nil
		// Try silent auto-refresh before showing the login screen
		if a.sessionManager != nil && a.session != nil &&
			a.session.Faculty != "" && a.session.Username != "" {
			return a, tryAutoRefreshCmd(a.session.Faculty, a.session.Username, a.sessionManager)
		}
		a.screen = screenLogin
		a.login = newLoginModel()
		if a.session != nil && a.session.Faculty != "" {
			a.login.faculty.input.SetValue(a.session.Faculty)
			if a.session.Username != "" {
				a.login.inputs[0].SetValue(a.session.Username)
			}
		}
		return a, a.login.Init()

	case autoRefreshResultMsg:
		if msg.err != nil {
			// if auto-refresh failed pre-fill the user
			a.screen = screenLogin
			a.login = newLoginModel()
			if a.session != nil && a.session.Faculty != "" {
				a.login.faculty.input.SetValue(a.session.Faculty)
				if a.session.Username != "" {
					a.login.inputs[0].SetValue(a.session.Username)
				}
			}
			return a, a.login.Init()
		}
		// if auto-refresh succeeded, rebuild the client with the fresh cookies
		if len(msg.cookies) > 0 && a.session != nil {
			if client, err := api.NewClient(a.session.Faculty, auth.CookiesToHTTP(msg.cookies)); err == nil {
				a.client = client
				a.schedule = newScheduleModel(client, a.session.Username)
				a.exams = newExamsModel(client, a.session.Username)
				return a, tea.Batch(a.schedule.Init(), a.exams.Init())
			}
		}
		a.screen = screenLogin
		a.login = newLoginModel()
		return a, a.login.Init()

	case scheduleStartFetchMsg, scheduleFetchedMsg, scheduleErrMsg:
		var cmd tea.Cmd
		a.schedule, cmd = a.schedule.Update(msg)
		return a, cmd

	case examsStartFetchMsg, examsFetchedMsg, examsErrMsg:
		var cmd tea.Cmd
		a.exams, cmd = a.exams.Update(msg)
		return a, cmd

	case updateCheckResultMsg:
		if msg.err == nil && msg.result.UpdateAvailable {
			a.updateAvail = msg.result.LatestVersion
			a.about.latestVersion = msg.result.LatestVersion
			a.about.updateState = updateStateAvailable
		} else if msg.err == nil {
			a.about.updateState = updateStateUpToDate
		} else {
			a.about.updateState = updateStateError
			a.about.errMsg = msg.err.Error()
		}
		return a, nil

	}

	var cmd tea.Cmd
	switch a.screen {
	case screenAuthSetup:
		a.authSetup, cmd = a.authSetup.Update(msg)
	case screenLogin:
		a.login, cmd = a.login.Update(msg)
	case screenSavePassword:
		a.savePassword, cmd = a.savePassword.Update(msg)
	case screenSchedule, screenWeek:
		a.schedule, cmd = a.schedule.Update(msg)
	case screenExams:
		a.exams, cmd = a.exams.Update(msg)
	}
	return a, cmd
}

func (a App) View() string {
	if a.showAbout {
		return a.aboutOverlay()
	}
	if a.showHelp {
		return a.helpOverlay()
	}
	switch a.screen {
	case screenAuthSetup:
		return a.authSetup.View()
	case screenLogin:
		return a.login.View()
	case screenSavePassword:
		return a.savePassword.View()
	}

	header := a.headerView()
	footer := a.footerView()

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	contentH := a.height - headerH - footerH
	if contentH < 5 {
		contentH = 5
	}

	nav := a.navSidebarView(contentH)

	var content string
	switch a.screen {
	case screenSchedule:
		content = a.schedule.View()
	case screenWeek:
		content = a.schedule.WeekView()
	case screenExams:
		content = a.exams.View()
	}

	main := content
	if a.useSidebar(a.width, a.height) {
		main = lipgloss.JoinHorizontal(lipgloss.Top, nav, content)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

func (a App) useSidebar(width, height int) bool {
	return width >= 72 && height >= 22
}

func (a App) navSidebarView(height int) string {
	makeItem := func(label string, active bool) string {
		if active {
			return lipgloss.NewStyle().
				Foreground(cAccent).
				Bold(true).
				Render("▶" + label)
		}
		return lipgloss.NewStyle().
			Foreground(cFgDim).
			Render(" " + label)
	}

	items := []string{
		makeItem("Schedule", a.screen == screenSchedule),
		makeItem("Week", a.screen == screenWeek),
		makeItem("Exams", a.screen == screenExams),
	}

	innerH := height - 4
	for len(items) < innerH {
		items = append(items, "")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(1, 1).
		Render(strings.Join(items, "\n"))
}

func (a App) headerTabsView() string {
	makeTab := func(label string, active bool) string {
		if active {
			return lipgloss.NewStyle().
				Bold(true).
				Foreground(cFgBright).
				Background(cSurface).
				Padding(0, 1).
				Render(label)
		}
		return lipgloss.NewStyle().
			Foreground(cFgDim).
			Padding(0, 1).
			Render(label)
	}
	return makeTab("Schedule", a.screen == screenSchedule) +
		makeTab("Week", a.screen == screenWeek) +
		makeTab("Exams", a.screen == screenExams)
}

func (a App) logoView() string {
	sym := lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render("▲ ")
	shi := lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render("sh")
	garra := lipgloss.NewStyle().Bold(true).Foreground(cFgBright).Render("igarra")
	return sym + shi + garra
}

func (a App) headerView() string {
	logo := a.logoView()

	username := ""
	if a.session != nil {
		username = lipgloss.NewStyle().Foreground(cFgDim).Render(a.session.Username)
	}

	badge := ""
	if a.updateAvail != "" {
		badge = lipgloss.NewStyle().
			Background(cYellow).
			Foreground(cBadgeFg).
			Bold(true).
			Padding(0, 1).
			Render("v" + a.updateAvail + " available")
	}

	tabs := ""
	if !a.useSidebar(a.width, a.height) {
		tabs = a.headerTabsView()
	}

	logoW := lipgloss.Width(logo)
	tabsW := lipgloss.Width(tabs)
	userW := lipgloss.Width(username)
	badgeW := lipgloss.Width(badge)

	var row string
	if a.width > 0 && tabsW > 0 {
		tabsStart := (a.width - tabsW) / 2
		leftPad := tabsStart - logoW - 2
		if leftPad < 1 {
			leftPad = 1
		}
		right := username
		rightW := userW
		if badgeW > 0 && a.width-2-tabsStart-tabsW-userW-badgeW-1 > 0 {
			right = badge + " " + username
			rightW = badgeW + 1 + userW
		}
		rightPad := a.width - 2 - tabsStart - tabsW - rightW
		if rightPad < 1 {
			rightPad = 1
		}
		row = "  " + logo + strings.Repeat(" ", leftPad) + tabs + strings.Repeat(" ", rightPad) + right + "  "
	} else {
		right := username
		rightW := userW
		if badgeW > 0 && a.width-logoW-userW-badgeW-5 > 0 {
			right = badge + " " + username
			rightW = badgeW + 1 + userW
		}
		gap := a.width - logoW - rightW - 4
		if gap < 1 {
			gap = 1
		}
		row = "  " + logo + strings.Repeat(" ", gap) + right + "  "
	}

	divider := lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", a.width))
	return row + "\n" + divider
}

func (a App) footerView() string {
	var keys help.KeyMap
	switch a.screen {
	case screenSchedule:
		keys = schedKeyMap{}
	case screenWeek:
		keys = weekKeyMap{}
	case screenExams:
		keys = examsKeyMap{}
	default:
		return ""
	}

	identity := ""
	if a.session != nil {
		identity = lipgloss.NewStyle().Foreground(cFgDim).
			Render(a.session.Username + " · " + strings.ToUpper(a.session.Faculty))
	}
	hints := lipgloss.NewStyle().Foreground(cFgMuted).
		Render(a.help.ShortHelpView(keys.ShortHelp()))

	divider := lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", a.width))

	// center the hint bar; identity left, hints centered
	hintsW := lipgloss.Width(hints)
	identW := lipgloss.Width(identity)
	if a.width <= 0 {
		return divider + "\n" + identity + "  " + hints
	}
	hintStart := (a.width - hintsW) / 2
	leftPad := hintStart - identW - 2
	if leftPad < 1 {
		leftPad = 1
	}
	rightPad := a.width - 2 - hintStart - hintsW
	if rightPad < 0 {
		rightPad = 0
	}
	return divider + "\n" + "  " + identity + strings.Repeat(" ", leftPad) + hints + strings.Repeat(" ", rightPad)
}

func (a App) helpOverlay() string {
	var keys help.KeyMap
	switch a.screen {
	case screenSchedule:
		keys = schedKeyMap{}
	case screenWeek:
		keys = weekKeyMap{}
	case screenExams:
		keys = examsKeyMap{}
	default:
		keys = schedKeyMap{}
	}

	content := a.help.FullHelpView(keys.FullHelp()) +
		"\n\n" + lipgloss.NewStyle().Foreground(cFgMuted).Render("press ? to close")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cAccent).
		Padding(1, 3).
		Render(
			lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render("Help") +
				"\n\n" + content,
		)

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
}

func (a App) aboutOverlay() string {
	box := a.about.View()
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
}
