package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

//login authenticates with SIGARRA using credentials and returns the seeded cookie jar.
func Login(ctx context.Context, faculty, username, password string) ([]*http.Cookie, error) {
	client, err := NewClient(faculty, nil)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	base := BaseURL(faculty, "pt")

	loginURL := base + "vld_validacao.validacao"
	resp, err := client.Post(ctx, loginURL, url.Values{
		"p_user": {username},
		"p_pass": {password},
	})
	if err != nil {
		return nil, fmt.Errorf("login POST: %w", err)
	}
	resp.Body.Close()

	homeURL := base + "web_page.inicial"
	resp, err = client.Get(ctx, homeURL)
	if err != nil {
		return nil, fmt.Errorf("home GET: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse home HTML: %w", err)
	}

	if doc.Find("div.autenticado").Length() == 0 {
		return nil, fmt.Errorf("authentication failed: invalid credentials")
	}

	return client.Cookies(), nil
}

// healthCheck verifies the session is still valid by checking the course list endpoint.
func HealthCheck(ctx context.Context, client *Client, username string) bool {
	checkURL := fmt.Sprintf("https://sigarra.up.pt/%s/pt/fest_geral.cursos_list?pv_num_unico=%s", client.Faculty, StudentNumber(username))
	resp, err := client.Get(ctx, checkURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
