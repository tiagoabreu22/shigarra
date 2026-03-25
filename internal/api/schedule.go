package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// filterSubjectName:
// example: "CC4068 - Tecnologias de Reforço da Privacidade (2S)" → "Tecnologias de Reforço da Privacidade"
var subjectNameRe = regexp.MustCompile(` - ([^()]*)(?: \(|$)`)

func filterSubjectName(name string) string {
	if m := subjectNameRe.FindStringSubmatch(name); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return name
}

var teacherNameRe = regexp.MustCompile(` - (.+)$`)

func filterTeacherName(name string) string {
	if m := teacherNameRe.FindStringSubmatch(name); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return name
}

// lecture represents a single class session.
type Lecture struct {
	Subject    string
	Acronym    string
	TypeClass  string // T, TP, P, OT, ...
	Start      time.Time
	End        time.Time
	Room       string
	Teacher    string
	ClassGroup string
}

// scheduleResponse is the root JSON object from the calendar event source.
type scheduleResponse struct {
	Data []scheduleLecture `json:"data"`
}

type scheduleLecture struct {
	Start    string           `json:"start"`
	End      string           `json:"end"`
	Units    []scheduleUnit   `json:"ucs"`
	Classes  []scheduleClass  `json:"classes"`
	Persons  []schedulePerson `json:"persons"`
	Rooms    []scheduleRoom   `json:"rooms"`
	Typology scheduleTypology `json:"typology"`
}

type scheduleUnit struct {
	Acronym   string `json:"acronym"`
	Name      string `json:"name"`
	SigarraID int    `json:"sigarra_id"`
}

type scheduleClass struct {
	Acronym string `json:"acronym"`
}

type schedulePerson struct {
	Acronym string `json:"acronym"`
	Name    string `json:"name"`
}

type scheduleRoom struct {
	Name string `json:"name"`
}

type scheduleTypology struct {
	Acronym string `json:"acronym"`
}

// lectiveYear returns the lective year integer for a given date,
//
// month < 8 → year - 1   (Feb 2026 → 2025)
// month >= 8 → year       (Sep 2025 → 2025)
func lectiveYear() int {
	now := time.Now()
	if now.Month() < time.August {
		return now.Year() - 1
	}
	return now.Year()
}

// fetchSchedule fetches and returns all lectures for the current academic year.
func FetchSchedule(ctx context.Context, client *Client, username string) ([]Lecture, error) {
	year := fmt.Sprintf("%d", lectiveYear())

	// GET the schedule page to extract the event source URL.
	params := url.Values{
		"pv_num_unico":   {StudentNumber(username)},
		"pv_ano_lectivo": {year},
		"pv_periodos":    {"1"},
	}
	schedURL := fmt.Sprintf("https://sigarra.up.pt/%s/pt/hor_geral.estudantes_view?%s",
		client.Faculty, params.Encode())

	resp, err := client.Get(ctx, schedURL)
	if err != nil {
		return nil, fmt.Errorf("fetch schedule page: %w", err)
	}
	defer resp.Body.Close()

	if err := CheckSessionExpired(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("schedule page returned HTTP %d — URL: %s", resp.StatusCode, schedURL)
	}

	// read body into a buffer so its possible to both parse it and dump it on failure.
	rawHTML, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read schedule HTML: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("parse schedule HTML: %w", err)
	}

	// extract data-evt-source-url from #cal-shadow-container.
	evtURL, exists := doc.Find("#cal-shadow-container").Attr("data-evt-source-url")
	if !exists {
		dumpPath := "/tmp/unitui_schedule_debug.html"
		_ = os.WriteFile(dumpPath, rawHTML, 0600)
		return nil, fmt.Errorf(
			"#cal-shadow-container[data-evt-source-url] not found\n  URL: %s\n  HTML dumped to: %s",
			schedURL, dumpPath,
		)
	}

	// make absolute if relative. Vergonha de dizer quantas horas perdi... 
	if strings.HasPrefix(evtURL, "/") {
		evtURL = "https://sigarra.up.pt" + evtURL
	}

	// fetch the JSON event list.
	evtResp, err := client.Get(ctx, evtURL)
	if err != nil {
		return nil, fmt.Errorf("fetch schedule events: %w", err)
	}
	defer evtResp.Body.Close()

	if err := CheckSessionExpired(evtResp); err != nil {
		return nil, err
	}
	if evtResp.StatusCode != 200 {
		return nil, fmt.Errorf("schedule events returned HTTP %d", evtResp.StatusCode)
	}

	body, err := io.ReadAll(evtResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read events body: %w", err)
	}

	var result scheduleResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse schedule JSON: %w", err)
	}

	var lectures []Lecture
	for _, e := range result.Data {
		start, err := parseScheduleTime(e.Start)
		if err != nil {
			continue
		}
		end, err := parseScheduleTime(e.End)
		if err != nil {
			continue
		}

		// first unit is the subject.
		subject, acronym := "", ""
		if len(e.Units) > 0 {
			subject = filterSubjectName(e.Units[0].Name)
			acronym = e.Units[0].Acronym
		}

		// first class group.
		classGroup := ""
		if len(e.Classes) > 0 {
			classGroup = e.Classes[0].Acronym
		}

		// first person (teacher).
		teacher := ""
		if len(e.Persons) > 0 {
			teacher = filterTeacherName(e.Persons[0].Name)
		}

		// first room.
		room := ""
		if len(e.Rooms) > 0 {
			room = e.Rooms[0].Name
		}

		lectures = append(lectures, Lecture{
			Subject:    subject,
			Acronym:    acronym,
			TypeClass:  e.Typology.Acronym,
			Start:      start,
			End:        end,
			Room:       room,
			Teacher:    teacher,
			ClassGroup: classGroup,
		})
	}

	return lectures, nil
}

// parseScheduleTime tries multiple ISO 8601 layouts.
func parseScheduleTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}
