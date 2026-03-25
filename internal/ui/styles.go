package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette — shared across all screens.
const (
	cAccent   = lipgloss.Color("75")  // sky blue — primary accent
	cFgBright = lipgloss.Color("255") // near-white
	cFg       = lipgloss.Color("252") // main text
	cFgDim    = lipgloss.Color("244") // secondary text
	cFgMuted  = lipgloss.Color("240") // hints / disabled
	cSurface  = lipgloss.Color("236") // active tab / card bg
	cBorder   = lipgloss.Color("238") // borders, dividers
	cBadgeFg  = lipgloss.Color("232") // dark text on coloured badge

	// Semantic
	cGreen  = lipgloss.Color("78")
	cYellow = lipgloss.Color("220")
	cOrange = lipgloss.Color("214")
	cRed    = lipgloss.Color("203")
	cCyan   = lipgloss.Color("87")
	cPurple = lipgloss.Color("141")
	cBlue   = lipgloss.Color("69")
)

// lectureTypeColor maps class-type codes to pill background colours.
var lectureTypeColor = map[string]lipgloss.Color{
	"T":  cBlue,
	"TP": cYellow,
	"P":  cGreen,
	"OT": cCyan,
	"PL": cGreen,
	"S":  cPurple,
}

// examTypeColor maps exam-type codes to pill background colours.
var examTypeColor = map[string]lipgloss.Color{
	"EN":  cRed,
	"ER":  cOrange,
	"MT":  cPurple,
	"EC":  cYellow,
	"EE":  cCyan,
	"EAE": cBlue,
}

func lectureTypePill(t string) string { return typeTag(t, lectureTypeColor) }
func examTypePill(t string) string    { return typeTag(t, examTypeColor) }

func lectureAccent(t string) lipgloss.Color {
	bg, ok := lectureTypeColor[t]
	if !ok {
		return cAccent
	}
	return bg
}

func timelineCardStyle(accent lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(accent).
		PaddingLeft(1).
		MarginBottom(0)
}

// typeTag renders a colored text label (no bg) for the lecture/exam type.
func typeTag(t string, colorMap map[string]lipgloss.Color) string {
	c, ok := colorMap[t]
	if !ok {
		c = cFgDim
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(t)
}

func timePill(text string) string {
	return lipgloss.NewStyle().
		Foreground(cFgBright).
		Bold(true).
		Render(text)
}


func sectionHeader(text string, width int) string {
	label := lipgloss.NewStyle().
		Bold(true).
		Foreground(cAccent).
		Render(text)
	if width <= 0 {
		return label
	}
	used := lipgloss.Width(text) + 2
	fill := width - used
	if fill < 0 {
		fill = 0
	}
	return label + " " + lipgloss.NewStyle().
		Foreground(cBorder).
		Render(strings.Repeat("─", fill))
}

// shigarraArt returns the 4-line SHIGARRA block-character wordmark.
// The first two letters (SH) are rendered in accent blue; the rest in bright white.
func shigarraArt() string {
	lines := []string{
		" ▄█▀▀▀█▄ ██   ██   ▀▀██▀    ▄█▀▀█▄    ▄██▄   ██▀▀▀█▄  ██▀▀▀█▄    ▄██▄   ",
		" ▀█▄▄▄   ██▄▄▄██     ██    ██  ▄▄▄  ▄█▀  ▀█▄ ██▄▄▄█▀  ██▄▄▄█▀  ▄█▀  ▀█▄ ",
		" ▄▄  ██  ██   ██     ██    ▀█▄  ██  ██▀▀▀▀██ ██  ▀█▄  ██  ▀█▄  ██▀▀▀▀██ ",
		"  ▀▀▀▀   ▀▀   ▀▀   ▀▀▀▀▀     ▀▀▀▀   ▀▀    ▀▀ ▀▀   ▀▀  ▀▀   ▀▀  ▀▀    ▀▀  ",
	}
	sh := lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	ig := lipgloss.NewStyle().Bold(true).Foreground(cFgBright)
	const shRunes = 18 // covers "SH" (9 runes each)
	rendered := make([]string, len(lines))
	for i, line := range lines {
		r := []rune(line)
		split := shRunes
		if split > len(r) {
			split = len(r)
		}
		rendered[i] = sh.Render(string(r[:split])) + ig.Render(string(r[split:]))
	}
	return strings.Join(rendered, "\n")
}

func monthHeader(text string) string {
	return lipgloss.NewStyle().
		Foreground(cFgDim).
		Bold(true).
		Render("── " + text + " ──")
}
