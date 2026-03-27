package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type facultyEntry struct {
	code string
	name string
}

var upFaculties = []facultyEntry{
	{"faup", "Arquitetura"},
	{"fcup", "Ciências"},
	{"fadeup", "Desporto"},
	{"fdup", "Direito"},
	{"feup", "Engenharia"},
	{"fep", "Economia"},
	{"ffup", "Farmácia"},
	{"flup", "Letras"},
	{"fmup", "Medicina"},
	{"fpceup", "Psicologia e Ciências da Educação"},
	{"icbas", "Ciências Biomédicas Abel Salazar"},
	{"esep", "Enfermagem do Porto"},
}

const dropdownMax = 5

type facultyPicker struct {
	input   textinput.Model
	matches []facultyEntry
	cursor  int
	open    bool
}

func FacultyPicker(defaultCode string) facultyPicker {
	ti := textinput.New()
	ti.Placeholder = "type to search…"
	ti.SetValue(defaultCode)
	ti.Width = loginInputW
	ti.CharLimit = 20
	p := facultyPicker{input: ti, cursor: -1}
	p.matches = filteredFaculties(p.input.Value())
	return p
}

func filteredFaculties(query string) []facultyEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]facultyEntry, 0, len(upFaculties))
	for _, e := range upFaculties {
		if q == "" || strings.Contains(e.code, q) || strings.Contains(strings.ToLower(e.name), q) {
			out = append(out, e)
		}
	}
	return out
}

func (p facultyPicker) Value() string {
	return strings.TrimSpace(p.input.Value())
}

func (p facultyPicker) Focus() facultyPicker {
	p.input.Focus()
	p.matches = filteredFaculties(p.input.Value())
	p.open = len(p.matches) > 0
	if p.cursor < 0 || p.cursor >= len(p.matches) {
		p.cursor = 0
	}
	return p
}

func (p facultyPicker) Blur() facultyPicker {
	p.input.Blur()
	p.open = false
	p.cursor = -1
	return p
}

func (p facultyPicker) handleKey(msg tea.KeyMsg) (facultyPicker, tea.Cmd, int) {
	switch msg.String() {
	case "down":
		if p.open {
			if p.cursor < len(p.matches)-1 {
				p.cursor++
			}
		} else {
			p.matches = filteredFaculties(p.input.Value())
			p.open = len(p.matches) > 0
			p.cursor = 0
		}
		return p, nil, 0

	case "up":
		if p.open && p.cursor > 0 {
			p.cursor--
			return p, nil, 0
		}
		if !p.open {
			return p, nil, -1
		}
		return p, nil, 0

	case "enter":
		if p.open && p.cursor >= 0 && p.cursor < len(p.matches) {
			p.input.SetValue(p.matches[p.cursor].code)
			p.open = false
			p.cursor = -1
		}
		return p, nil, 1

	case "tab":
		if p.open && p.cursor >= 0 && p.cursor < len(p.matches) {
			p.input.SetValue(p.matches[p.cursor].code)
		}
		p.open = false
		return p, nil, 1

	case "shift+tab":
		p.open = false
		return p, nil, -1

	case "esc":
		p.open = !p.open
		if p.open {
			p.matches = filteredFaculties(p.input.Value())
			p.cursor = 0
		}
		return p, nil, 0

	default:
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		p.matches = filteredFaculties(p.input.Value())
		if len(p.matches) > 0 {
			p.open = true
			p.cursor = 0
		} else {
			p.open = false
			p.cursor = -1
		}
		return p, cmd, 0
	}
}

func (p facultyPicker) visibleRange() (int, int) {
	total := len(p.matches)
	if total == 0 {
		return 0, 0
	}
	start := p.cursor - dropdownMax/2
	if start < 0 {
		start = 0
	}
	end := start + dropdownMax
	if end > total {
		end = total
		start = end - dropdownMax
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func (p facultyPicker) inputView(focused bool) string {
	borderColor := cSurface
	if focused {
		borderColor = cAccent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(loginBoxW).
		Render(p.input.View())
}

func (p facultyPicker) dropdownView() string {
	start, end := p.visibleRange()
	rows := make([]string, 0, dropdownMax)
	for i := start; i < end; i++ {
		e := p.matches[i]
		line := fmt.Sprintf("%-8s %s", e.code, e.name)
		runes := []rune(line)
		maxW := loginBoxW - 2
		if len(runes) > maxW {
			runes = runes[:maxW-1]
			line = string(runes) + "…"
		}
		rowStyle := lipgloss.NewStyle().Padding(0, 1).Width(loginBoxW)
		if i == p.cursor {
			rowStyle = rowStyle.Background(cAccent).Foreground(cBadgeFg)
		} else {
			rowStyle = rowStyle.Foreground(cFgDim)
		}
		rows = append(rows, rowStyle.Render(line))
	}
	blank := lipgloss.NewStyle().Width(loginBoxW).Render("")
	for len(rows) < dropdownMax {
		rows = append(rows, blank)
	}
	return strings.Join(rows, "\n")
}
