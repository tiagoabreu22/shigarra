package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tiagoabreu22/shigarra/internal/api"
)

// examItem wraps api.Exam to implement list.Item.
type examItem struct{ exam api.Exam }

func (e examItem) FilterValue() string { return e.exam.Subject }

type examSectionItem struct{ label string }

func (e examSectionItem) FilterValue() string { return e.label }

// examDelegate is a custom list delegate that renders exam rows with pill badges.
type examDelegate struct{}

func (d examDelegate) Height() int  { return 2 }
func (d examDelegate) Spacing() int { return 1 }

func (d examDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d examDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if section, ok := item.(examSectionItem); ok {
		fmt.Fprint(w, monthHeader(section.label))
		return
	}

	ex, ok := item.(examItem)
	if !ok {
		return
	}
	selected := index == m.Index()

	// Date + time
	date := ex.exam.Start.Format("Mon 02 Jan")
	timeStr := ""
	if ex.exam.Start.Hour() != 0 || ex.exam.Start.Minute() != 0 {
		timeStr = ex.exam.Start.Format("15:04")
		if !ex.exam.End.IsZero() && ex.exam.End != ex.exam.Start {
			timeStr += "–" + ex.exam.End.Format("15:04")
		}
	}
	if timeStr == "" {
		timeStr = "--"
	}

	rooms := strings.Join(ex.exam.Rooms, ", ")
	if rooms == "" {
		rooms = "-"
	}

	cardColor := examAccent(ex.exam.Type)
	if selected {
		cardColor = cAccent
	}

	countdown := daysUntil(ex.exam.Start)

	// Line 1: date  time  type  countdown
	line1 := timePill(date+"  "+timeStr) + "   " + examTypePill(ex.exam.Type) + "   " + countdown

	// Line 2: subject  ·  rooms
	subjectStyle := lipgloss.NewStyle().Foreground(cFg)
	if selected {
		subjectStyle = subjectStyle.Foreground(cFgBright).Bold(true)
	}
	line2 := subjectStyle.Render(ex.exam.Subject) +
		lipgloss.NewStyle().Foreground(cFgMuted).Render("  ·  "+rooms)

	row := timelineCardStyle(cardColor).Width(m.Width() - 4).Render(line1 + "\n" + line2)
	fmt.Fprint(w, row)
}

// daysUntil returns a coloured countdown label.
func daysUntil(t time.Time) string {
	days := int(math.Ceil(time.Until(t).Hours() / 24))
	var label string
	var color lipgloss.Color
	switch {
	case days <= 0:
		label, color = "today", cRed
	case days == 1:
		label, color = "tomorrow", cOrange
	case days <= 7:
		label, color = fmt.Sprintf("in %dd", days), cYellow
	default:
		label, color = fmt.Sprintf("in %dd", days), cFgMuted
	}
	return lipgloss.NewStyle().Foreground(color).Render(label)
}

func examAccent(t string) lipgloss.Color {
	bg, ok := examTypeColor[t]
	if !ok {
		return cAccent
	}
	return bg
}

type examsModel struct {
	list      list.Model
	loading   bool
	errMsg    string
	spinner   spinner.Model
	width     int
	height    int
	apiClient *api.Client
	username  string
	cancel    context.CancelFunc
	requestID int
}

type examsStartFetchMsg struct{}

type examsFetchedMsg struct {
	exams     []api.Exam
	requestID int
}

type examsErrMsg struct {
	err       error
	requestID int
}

func newExamsModel(client *api.Client, username string) examsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cAccent)

	// Create an empty list; items will be set when data arrives.
	l := list.New(nil, examDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(cFgDim).Padding(2, 0)

	return examsModel{
		loading:   true,
		spinner:   sp,
		list:      l,
		apiClient: client,
		username:  username,
	}
}

func (m *examsModel) Cancel() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m examsModel) fetchCmd(ctx context.Context, requestID int) tea.Cmd {
	return func() tea.Msg {
		profile, err := api.FetchProfile(ctx, m.apiClient, m.username)
		if err != nil {
			friendlyErr, extraCmd := friendlyRequestError("profile fetch", err)
			if extraCmd != nil {
				return tea.Batch(
					func() tea.Msg { return examsErrMsg{err: friendlyErr, requestID: requestID} },
					extraCmd,
				)()
			}
			return examsErrMsg{err: friendlyErr, requestID: requestID}
		}

		ids := make([]int, 0, len(profile.Courses))
		for _, c := range profile.Courses {
			ids = append(ids, c.ID)
		}

		exams, err := api.FetchExams(ctx, m.apiClient, ids)
		if err != nil {
			friendlyErr, extraCmd := friendlyRequestError("exams refresh", err)
			if extraCmd != nil {
				return tea.Batch(
					func() tea.Msg { return examsErrMsg{err: friendlyErr, requestID: requestID} },
					extraCmd,
				)()
			}
			return examsErrMsg{err: friendlyErr, requestID: requestID}
		}
		return examsFetchedMsg{exams: exams, requestID: requestID}
	}
}

func (m examsModel) startFetch() (examsModel, tea.Cmd) {
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

func (m examsModel) Init() tea.Cmd {
	return func() tea.Msg { return examsStartFetchMsg{} }
}

func (m examsModel) Update(msg tea.Msg) (examsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-6, msg.Height-6)

	case examsStartFetchMsg:
		return m.startFetch()

	case tea.KeyMsg:
		if m.loading && msg.String() != "r" {
			return m, nil
		}
		if msg.String() == "r" {
			return m.startFetch()
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case examsFetchedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}
		m.Cancel()
		m.loading = false
		future := filterFutureExams(msg.exams)
		sort.Slice(future, func(i, j int) bool {
			return future[i].Start.Before(future[j].Start)
		})
		items := make([]list.Item, 0, len(future)+4)
		month := ""
		for _, ex := range future {
			current := ex.Start.Format("January 2006")
			if current != month {
				items = append(items, examSectionItem{label: current})
				month = current
			}
			items = append(items, examItem{ex})
		}
		m.list.SetItems(items)
		if len(items) > 1 {
			m.list.Select(1)
		}
		w, h := m.width-6, m.height-6
		if w < 40 {
			w = 80
		}
		if h < 5 {
			h = 20
		}
		m.list.SetSize(w, h)

	case examsErrMsg:
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

func filterFutureExams(exams []api.Exam) []api.Exam {
	now := time.Now()
	out := make([]api.Exam, 0, len(exams))
	for _, e := range exams {
		if e.End.After(now) || e.Start.After(now) {
			out = append(out, e)
		}
	}
	return out
}

func (m examsModel) View() string {
	pad := lipgloss.NewStyle().Padding(1, 3)

	if m.loading {
		return pad.Render(m.spinner.View() + "  " +
			lipgloss.NewStyle().Foreground(cFgDim).Render("Loading exams…"))
	}

	if m.errMsg != "" {
		return pad.Render(
			lipgloss.NewStyle().Foreground(cRed).Render("✗  "+m.errMsg) + "\n\n" +
				lipgloss.NewStyle().Foreground(cFgMuted).Render("r  retry"),
		)
	}

	if len(m.list.Items()) == 0 {
		return pad.Render(lipgloss.NewStyle().Foreground(cFgDim).
			Render("✓ No upcoming exams - enjoy the break"))
	}

	return pad.Render(m.list.View())
}
