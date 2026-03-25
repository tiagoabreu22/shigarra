package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type Profile struct {
	Name    string   `json:"nome"`
	Email   string   `json:"email"`
	Courses []Course `json:"curso"`
}

type Course struct {
	ID           int    `json:"cur_id"`
	FestID       int    `json:"fest_id"`
	Name         string `json:"nome"`
	Abbreviation string `json:"sigla"`
	Faculty      string `json:"org_sigla"`
}

// profileResponse is the raw JSON structure from the API.
type profileResponse struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Curso []struct {
		CurID    int    `json:"cur_id"`
		FestID   int    `json:"fest_id"`
		Nome     string `json:"nome"`
		Sigla    string `json:"sigla"`
		OrgSigla string `json:"org_sigla"`
	} `json:"curso"`
}

// FetchProfile fetches the student profile including enrolled courses.
func FetchProfile(ctx context.Context, client *Client, username string) (*Profile, error) {
	profileURL := fmt.Sprintf("https://sigarra.up.pt/%s/pt/mob_fest_geral.perfil?pv_codigo=%s",
		client.Faculty, StudentNumber(username))

	resp, err := client.Get(ctx, profileURL)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", err)
	}
	defer resp.Body.Close()

	if err := CheckSessionExpired(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read profile body: %w", err)
	}

	var raw profileResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse profile JSON: %w", err)
	}

	profile := &Profile{
		Name:  raw.Nome,
		Email: raw.Email,
	}
	for _, c := range raw.Curso {
		profile.Courses = append(profile.Courses, Course{
			ID:           c.CurID,
			FestID:       c.FestID,
			Name:         c.Nome,
			Abbreviation: c.Sigla,
			Faculty:      c.OrgSigla,
		})
	}

	return profile, nil
}
