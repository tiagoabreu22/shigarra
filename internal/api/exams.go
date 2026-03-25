package api

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Exam struct {
	Subject string
	Acronym string
	Type    string // EN, ER, MT, EC, EE, EAE
	Start   time.Time
	End     time.Time
	Rooms   []string
}

var timeRangeRe = regexp.MustCompile(`(\d{2}:\d{2})-(\d{2}:\d{2})`)

// session substring matching on the h3 text
var examTypeMap = []struct {
	key  string
	code string
}{
	{"Mini-testes", "MT"},
	{"Normal", "EN"},
	{"Recurso", "ER"},
	{"Especial de Conclusão", "EC"},
	{"Port.Est.Especiais", "EE"},
	{"Exames ao abrigo de estatutos especiais", "EAE"},
}

func getExamSeasonAbbr(seasonStr string) string {
	for _, entry := range examTypeMap {
		if strings.Contains(seasonStr, entry.key) {
			return entry.code
		}
	}
	return "?"
}

// FetchExams fetches exams for all given course IDs.
func FetchExams(ctx context.Context, client *Client, courseIDs []int) ([]Exam, error) {
	var allExams []Exam
	seen := map[string]bool{}

	for _, curID := range courseIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		exams, err := fetchExamsForCourse(ctx, client, curID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		for _, ex := range exams {
			key := fmt.Sprintf("%s|%s|%s", ex.Acronym, ex.Type, ex.Start.Format(time.RFC3339))
			if !seen[key] {
				seen[key] = true
				allExams = append(allExams, ex)
			}
		}
	}

	return allExams, nil
}

func fetchExamsForCourse(ctx context.Context, client *Client, curID int) ([]Exam, error) {
	examURL := fmt.Sprintf("https://sigarra.up.pt/%s/pt/exa_geral.mapa_de_exames?p_curso_id=%d",
		client.Faculty, curID)

	resp, err := client.Get(ctx, examURL)
	if err != nil {
		return nil, fmt.Errorf("fetch exams: %w", err)
	}
	defer resp.Body.Close()

	if err := CheckSessionExpired(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse exams HTML: %w", err)
	}

	return parseExamsDoc(doc), nil
}

func parseExamsDoc(doc *goquery.Document) []Exam {
	var examTypes []string
	doc.Find("h3").Each(func(_ int, s *goquery.Selection) {
		examTypes = append(examTypes, getExamSeasonAbbr(s.Text()))
	})

	var (
		dates    []string
		exams    []Exam
		days     int
		tableNum int
	)

	doc.Find("div > table > tbody > tr > td").Each(func(_ int, td *goquery.Selection) {
		td.Find("table").Not(".mapa").Each(func(_ int, table *goquery.Selection) {
			table.Find("span.exame-data").Each(func(_ int, span *goquery.Selection) {
				dates = append(dates, strings.TrimSpace(span.Text()))
			})

			table.Find("td.l.k").Each(func(_ int, examTd *goquery.Selection) {
				if examTd.Find("td.exame").Length() > 0 {
					examTd.Find("td.exame").Each(func(_ int, examsDay *goquery.Selection) {
						acronym := ""
						subject := ""
						if link := examsDay.Find("a").First(); link.Length() > 0 {
							acronym = strings.TrimSpace(link.Text())
							subject, _ = link.Attr("title")
						}

						var rooms []string
						if sala := examsDay.Find("span.exame-sala").First(); sala.Length() > 0 {
							for _, r := range strings.Split(sala.Text(), ",") {
								r = strings.TrimSpace(r)
								if r != "" {
									rooms = append(rooms, r)
								}
							}
						}

						dayText := examsDay.Text()
						var begin, end time.Time

						if days < len(dates) {
							dateStr := dates[days]
							if !strings.HasSuffix(dayText, "-") {
								if m := timeRangeRe.FindStringSubmatch(dayText); len(m) == 3 {
									begin = parseDateTimeLocal(dateStr, m[1])
									end = parseDateTimeLocal(dateStr, m[2])
								}
							}
							if begin.IsZero() {
								begin = parseDateTimeLocal(dateStr, "00:00")
								end = begin
							}
						}

						examType := "?"
						if tableNum < len(examTypes) {
							examType = examTypes[tableNum]
						}

						exams = append(exams, Exam{
							Subject: subject,
							Acronym: acronym,
							Type:    examType,
							Start:   begin,
							End:     end,
							Rooms:   rooms,
						})
					})
				}
				days++
			})
		})
		tableNum++
	})

	return exams
}

// parseDateTimeLocal parses "YYYY-MM-DD" + "HH:MM" in local timezone.
func parseDateTimeLocal(dateStr, timeStr string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+timeStr, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}
