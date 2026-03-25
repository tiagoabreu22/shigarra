package ui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tiagoabreu22/shigarra/internal/api"
)

type scheduleModel struct {
	lectures    []api.Lecture
	byDay       map[string][]api.Lecture
	sortedDays  []string
	currentDate string // selected calendar date (YYYY-MM-DD)
	loading     bool
	errMsg      string
	spinner     spinner.Model
	width       int
	height      int
	apiClient   *api.Client
	username    string
	cancel      context.CancelFunc
	requestID   int
}

type scheduleStartFetchMsg struct{}

type scheduleFetchedMsg struct {
	lectures  []api.Lecture
	requestID int
}

type scheduleErrMsg struct {
	err       error
	requestID int
}

func newScheduleModel(client *api.Client, username string) scheduleModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cAccent)
	return scheduleModel{
		loading:   true,
		spinner:   sp,
		apiClient: client,
		username:  username,
	}
}

func (m *scheduleModel) Cancel() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m scheduleModel) fetchCmd(ctx context.Context, requestID int) tea.Cmd {
	return func() tea.Msg {
		lectures, err := api.FetchSchedule(ctx, m.apiClient, m.username)
		if err != nil {
			friendlyErr, extraCmd := friendlyRequestError("schedule refresh", err)
			if extraCmd != nil {
				// Session expired - trigger both error display and session expiry handling
				return tea.Batch(
					func() tea.Msg { return scheduleErrMsg{err: friendlyErr, requestID: requestID} },
					extraCmd,
				)()
			}
			return scheduleErrMsg{err: friendlyErr, requestID: requestID}
		}
		return scheduleFetchedMsg{lectures: lectures, requestID: requestID}
	}
}

func (m scheduleModel) startFetch() (scheduleModel, tea.Cmd) {
	if m.apiClient == nil || m.username == "" {
		m.loading = false
		m.errMsg = "missing session"
		return m, nil
	}

	m.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), api.RequestTimeout)
	m.cancel = cancel
	m.requestID++
	requestID := m.requestID

	m.loading = true
	m.errMsg = ""
	return m, tea.Batch(m.spinner.Tick, m.fetchCmd(ctx, requestID))
}

func (m scheduleModel) Init() tea.Cmd {
	return func() tea.Msg { return scheduleStartFetchMsg{} }
}

func (m scheduleModel) Update(msg tea.Msg) (scheduleModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case scheduleStartFetchMsg:
		return m.startFetch()

	case tea.KeyMsg:
		if m.loading && msg.String() != "r" {
			return m, nil
		}
		switch msg.String() {
		case "left", "h":
			if len(m.sortedDays) > 0 {
				m.currentDate = addDays(m.currentDate, -1)
				if m.currentDate < m.sortedDays[0] {
					m.currentDate = m.sortedDays[0]
				}
			}
		case "right", "l":
			if len(m.sortedDays) > 0 {
				m.currentDate = addDays(m.currentDate, +1)
				last := m.sortedDays[len(m.sortedDays)-1]
				if m.currentDate > last {
					m.currentDate = last
				}
			}
		case "r":
			return m.startFetch()
		}

	case scheduleFetchedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}
		m.Cancel()
		m.loading = false
		m.lectures = msg.lectures
		m.byDay, m.sortedDays = groupByDay(msg.lectures)
		m.currentDate = todayOrNearest(m.sortedDays)

	case scheduleErrMsg:
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

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func groupByDay(lectures []api.Lecture) (map[string][]api.Lecture, []string) {
	byDay := make(map[string][]api.Lecture)
	for _, l := range lectures {
		key := l.Start.Format("2006-01-02")
		byDay[key] = append(byDay[key], l)
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)
	for d := range byDay {
		sort.Slice(byDay[d], func(i, j int) bool {
			return byDay[d][i].Start.Before(byDay[d][j].Start)
		})
	}
	return byDay, days
}

// addDays returns a YYYY-MM-DD date string offset by n days.
func addDays(date string, n int) string {
	t, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

// weekMonday returns the Monday of the ISO week containing date.
func weekMonday(date string) string {
	t, _ := time.ParseInLocation("2006-01-02", date, time.Local)
	offset := int(t.Weekday()+6) % 7 // shift so Monday=0
	return t.AddDate(0, 0, -offset).Format("2006-01-02")
}

// todayOrNearest returns today if within lecture bounds, else the nearest bound.
func todayOrNearest(days []string) string {
	today := time.Now().Format("2006-01-02")
	if len(days) == 0 {
		return today
	}
	if today < days[0] {
		return days[0]
	}
	last := days[len(days)-1]
	if today > last {
		return last
	}
	return today
}

func centeredCell(text string, width int, style lipgloss.Style) string {
	return style.Width(width).Align(lipgloss.Center).Render(text)
}

func (m scheduleModel) placeContent(block string, panelWidth int) string {
	rendered := lipgloss.NewStyle().
		Width(panelWidth).
		Align(lipgloss.Left).
		Padding(1, 1).
		Render(block)
	if m.width <= 0 {
		return lipgloss.NewStyle().PaddingLeft(2).Render(rendered)
	}
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, rendered)
}

func (m scheduleModel) schedulePanelWidth() int {
	if m.width <= 0 {
		return 84
	}
	panel := m.width - 8
	if panel > 132 {
		panel = 132
	}
	if panel < 56 {
		panel = m.width - 2
	}
	if panel < 40 {
		panel = 40
	}
	return panel
}

func (m scheduleModel) dynamicPanelWidth(block string) int {
	panel := lipgloss.Width(block) + 2
	if panel < 56 {
		panel = 56
	}
	if m.width > 0 {
		maxPanel := m.width - 2
		if maxPanel < 40 {
			maxPanel = 40
		}
		if panel > maxPanel {
			panel = maxPanel
		}
	}
	return panel
}

// weekStripView renders a compact Mon–Sun strip with fixed-width rows.
func (m scheduleModel) weekStripView() string {
	const cellW = 7
	monday := weekMonday(m.currentDate)
	today := time.Now().Format("2006-01-02")

	var dayParts [7]string
	var dotParts [7]string
	var dateParts [7]string

	for i := 0; i < 7; i++ {
		d := addDays(monday, i)
		t, _ := time.ParseInLocation("2006-01-02", d, time.Local)
		name := t.Format("Mon")

		isSelected := d == m.currentDate
		isToday := d == today
		hasLectures := len(m.byDay[d]) > 0

		label := name
		if isSelected {
			label = "[" + name + "]"
		}

		dayStyle := lipgloss.NewStyle().Foreground(cFgDim)
		switch {
		case isSelected && isToday:
			dayStyle = dayStyle.Foreground(cAccent).Bold(true)
		case isSelected:
			dayStyle = dayStyle.Foreground(cFgBright).Bold(true)
		case isToday:
			dayStyle = dayStyle.Foreground(cAccent)
		}
		dayParts[i] = centeredCell(label, cellW, dayStyle)

		dotChar := "·"
		dotColor := cFgMuted
		switch {
		case isSelected && hasLectures:
			dotChar, dotColor = "●", cAccent
		case isSelected:
			dotChar, dotColor = "·", cAccent
		case hasLectures:
			dotChar, dotColor = "○", cFg
		}
		dotParts[i] = centeredCell(dotChar, cellW, lipgloss.NewStyle().Foreground(dotColor))

		dateStyle := lipgloss.NewStyle().Foreground(cFgMuted)
		if isSelected {
			dateParts[i] = lipgloss.NewStyle().
				Width(cellW).
				Align(lipgloss.Center).
				Render(
					lipgloss.NewStyle().
						Background(cAccent).
						Foreground(cFgBright).
						Bold(true).
						Padding(0, 1).
						Render(t.Format("02")),
				)
			continue
		} else if isToday {
			dateStyle = dateStyle.Foreground(cAccent).Bold(true)
		}
		dateParts[i] = centeredCell(t.Format("02"), cellW, dateStyle)
	}

	rule := lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", cellW*7))
	return strings.Join(dayParts[:], "") + "\n" +
		strings.Join(dotParts[:], "") + "\n" +
		strings.Join(dateParts[:], "") + "\n" +
		rule
}

// shortenTeacherName keeps the first two words and the last word of a full name.
func shortenTeacherName(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) <= 3 {
		return strings.Join(parts, " ")
	}
	return parts[0] + " " + parts[1] + " " + parts[len(parts)-1]
}

// shortRoom returns the first comma-separated segment of a room string, or "room ?" if empty.
func shortRoom(room string) string {
	room = strings.TrimSpace(room)
	if room == "" {
		return "room ?"
	}
	if idx := strings.Index(room, ","); idx > 0 {
		room = strings.TrimSpace(room[:idx])
	}
	return room
}

// subjectInitials builds an uppercase acronym from a subject name, skipping Portuguese stopwords.
// Falls back to the uppercase fallback string, or "?" if both are empty.
func subjectInitials(subject, fallback string) string {
	stopwords := map[string]bool{
		"de": true, "da": true, "do": true, "dos": true, "das": true,
		"e": true, "a": true, "o": true,
	}
	parts := strings.Fields(strings.TrimSpace(subject))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		token := strings.Trim(p, ".,;:-_()[]{}")
		if token == "" {
			continue
		}
		if stopwords[strings.ToLower(token)] {
			continue
		}
		r := []rune(token)
		if len(r) == 0 {
			continue
		}
		out = append(out, strings.ToUpper(string(r[0])))
	}
	if len(out) == 0 {
		f := strings.ToUpper(strings.TrimSpace(fallback))
		if f == "" {
			return "?"
		}
		return f
	}
	return strings.Join(out, "")
}

func (m scheduleModel) lectureTimelineView(lectures []api.Lecture) string {
	cards := make([]string, 0, len(lectures))
	panelW := m.schedulePanelWidth()

	for _, l := range lectures {
		accent := lectureAccent(l.TypeClass)
		subject := l.Subject
		if subject == "" {
			subject = l.Acronym
		}
		teacher := shortenTeacherName(l.Teacher)

		timeStr := fmt.Sprintf("%s–%s", l.Start.Format("15:04"), l.End.Format("15:04"))

		// Line 1: time  room  type  (time is king, room second, type is a label)
		line1 := lipgloss.NewStyle().Foreground(cFgBright).Bold(true).Render(timeStr) +
			"     " +
			lipgloss.NewStyle().Foreground(accent).Bold(true).Render(shortRoom(l.Room)) +
			"   " +
			lipgloss.NewStyle().Foreground(accent).Render(l.TypeClass)

		// Line 2: subject name  ·  teacher (dim)
		line2 := lipgloss.NewStyle().Foreground(cFg).Render(subject)
		if teacher != "" {
			line2 += lipgloss.NewStyle().Foreground(cFgMuted).Render(" · " + teacher)
		}

		card := timelineCardStyle(accent).Width(panelW).Render(line1 + "\n" + line2)
		cards = append(cards, card)
	}
	// One blank line between cards so borders don't merge
	return strings.Join(cards, "\n\n")
}

// overlapsHourSlot reports whether lecture l overlaps the one-hour slot starting at hour on day (YYYY-MM-DD).
func overlapsHourSlot(l api.Lecture, day string, hour int) bool {
	slotStart, _ := time.ParseInLocation(
		"2006-01-02 15:04",
		fmt.Sprintf("%s %02d:00", day, hour),
		time.Local,
	)
	slotEnd := slotStart.Add(time.Hour)
	return l.Start.Before(slotEnd) && l.End.After(slotStart)
}

// weekCellLabel formats a compact "room initials" label for the week grid cell.
func weekCellLabel(l api.Lecture) string {
	room := shortRoom(l.Room)
	return room + " " + subjectInitials(l.Subject, l.Acronym)
}

func (m scheduleModel) weekCell(day string, hour int) (string, lipgloss.Color) {
	items := make([]string, 0, 2)
	color := cFg
	for _, l := range m.byDay[day] {
		if overlapsHourSlot(l, day, hour) {
			items = append(items, weekCellLabel(l))
			color = lectureAccent(l.TypeClass)
		}
	}
	if len(items) == 0 {
		return "", cFgMuted
	}
	return strings.Join(items, ", "), color
}

// truncateCell shortens text to width runes, appending "." if truncated.
func truncateCell(text string, width int) string {
	r := []rune(text)
	if len(r) <= width {
		return text
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "."
}

func (m scheduleModel) weekTimetableView() string {
	if m.width > 0 && m.width < 82 {
		return m.weekTimetableCompactView()
	}

	monday := weekMonday(m.currentDate)
	available := 92
	if m.width > 0 {
		available = m.width - 8
	}
	if available < 72 {
		available = 72
	}

	timeColW := 10
	dayColW := (available - timeColW) / 7
	if dayColW < 8 {
		dayColW = 8
	}
	if dayColW > 18 {
		dayColW = 18
	}

	totalW := timeColW + (dayColW * 7)
	today := time.Now().Format("2006-01-02")
	headerCells := []string{
		lipgloss.NewStyle().Width(timeColW).Foreground(cFgMuted).Bold(true).Render("Time"),
	}

	for day := 0; day < 7; day++ {
		date := addDays(monday, day)
		t, _ := time.ParseInLocation("2006-01-02", date, time.Local)
		style := lipgloss.NewStyle().
			Width(dayColW).
			Align(lipgloss.Center).
			Foreground(cAccent).
			Bold(true)
		if date == today {
			style = style.Foreground(cFgBright)
		}
		headerCells = append(headerCells, style.Render(t.Format("Mon 02")))
	}

	weekStart, _ := time.ParseInLocation("2006-01-02", monday, time.Local)
	weekEnd, _ := time.ParseInLocation("2006-01-02", addDays(monday, 6), time.Local)
	weekRange := fmt.Sprintf("%s – %s", weekStart.Format("02 Jan"), weekEnd.Format("02 Jan 2006"))
	title := sectionHeader(
		weekRange,
		totalW,
	)
	rule := lipgloss.NewStyle().Foreground(cBorder).Render(strings.Repeat("─", totalW))
	lines := []string{title, "", strings.Join(headerCells, ""), rule}

	for hour := 8; hour < 18; hour++ {
		rowCells := []string{
			lipgloss.NewStyle().Width(timeColW).Foreground(cFgMuted).
				Render(fmt.Sprintf("%02d:00-%02d", hour, hour+1)),
		}

		for day := 0; day < 7; day++ {
			date := addDays(monday, day)
			text, color := m.weekCell(date, hour)
			style := lipgloss.NewStyle().Width(dayColW)
			if text != "" {
				style = style.Foreground(color)
				text = truncateCell(text, dayColW-1)
			}
			if date == today {
				style = style.Bold(true)
			}
			rowCells = append(rowCells, style.Render(text))
		}
		lines = append(lines, strings.Join(rowCells, ""))
	}

	return strings.Join(lines, "\n")
}

func (m scheduleModel) weekTimetableCompactView() string {
	monday := weekMonday(m.currentDate)
	today := time.Now().Format("2006-01-02")
	weekStart, _ := time.ParseInLocation("2006-01-02", monday, time.Local)
	weekEnd, _ := time.ParseInLocation("2006-01-02", addDays(monday, 6), time.Local)
	weekRange := fmt.Sprintf("%s – %s", weekStart.Format("02 Jan"), weekEnd.Format("02 Jan 2006"))

	lines := []string{
		sectionHeader(weekRange, m.schedulePanelWidth()),
		lipgloss.NewStyle().Foreground(cFgMuted).Render("Compact week view for narrow screens"),
		"",
	}

	for day := 0; day < 7; day++ {
		date := addDays(monday, day)
		t, _ := time.ParseInLocation("2006-01-02", date, time.Local)

		headerStyle := lipgloss.NewStyle().Foreground(cFgDim).Bold(true)
		if date == today {
			headerStyle = headerStyle.Foreground(cAccent)
		}
		lines = append(lines, headerStyle.Render(t.Format("Mon 02 Jan")))

		dayLectures := m.byDay[date]
		if len(dayLectures) == 0 {
			lines = append(lines, lipgloss.NewStyle().Foreground(cFgMuted).Render("  · no classes"), "")
			continue
		}

		for _, l := range dayLectures {
			code := subjectInitials(l.Subject, l.Acronym)
			room := shortRoom(l.Room)
			row := fmt.Sprintf("  %s-%s  %s  %s",
				l.Start.Format("15:04"),
				l.End.Format("15:04"),
				room,
				code,
			)
			lines = append(lines, lipgloss.NewStyle().Foreground(lectureAccent(l.TypeClass)).Render(row))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m scheduleModel) loadingOrErrorView(title string) string {
	if m.loading {
		content := m.spinner.View() + "  " +
			lipgloss.NewStyle().Foreground(cFgDim).Render("Loading "+title+"...")
		return m.placeContent(content, m.dynamicPanelWidth(content))
	}
	if m.errMsg != "" {
		content := lipgloss.NewStyle().Foreground(cRed).Render("x  "+m.errMsg) + "\n\n" +
			lipgloss.NewStyle().Foreground(cFgMuted).Render("r  retry")
		return m.placeContent(content, m.dynamicPanelWidth(content))
	}
	if len(m.sortedDays) == 0 {
		content := lipgloss.NewStyle().Foreground(cFgDim).Render("No schedule data available.")
		return m.placeContent(content, m.dynamicPanelWidth(content))
	}
	return ""
}

func (m scheduleModel) View() string {
	if v := m.loadingOrErrorView("schedule"); v != "" {
		return v
	}

	t, _ := time.ParseInLocation("2006-01-02", m.currentDate, time.Local)
	dayLabel := t.Format("Monday, 02 January 2006")

	strip := m.weekStripView()
	lectures := m.byDay[m.currentDate]
	dayHeader := sectionHeader(dayLabel, m.schedulePanelWidth())
	if len(lectures) == 0 {
		empty := lipgloss.NewStyle().Foreground(cFgDim).Render("No classes today")
		content := strip + "\n\n" + dayHeader + "\n\n" + empty
		return m.placeContent(content, m.schedulePanelWidth())
	}

	timeline := m.lectureTimelineView(lectures)
	content := strip + "\n\n" + dayHeader + "\n\n" + timeline
	return m.placeContent(content, m.schedulePanelWidth())
}

func (m scheduleModel) WeekView() string {
	if v := m.loadingOrErrorView("weekly schedule"); v != "" {
		return v
	}
	content := m.weekTimetableView()
	return m.placeContent(content, m.dynamicPanelWidth(content))
}
