package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	sigarraHost    = "sigarra.up.pt"
	RequestTimeout = 30 * time.Second
)

// Client wraps an http.Client with a cookie jar.
type Client struct {
	http    *http.Client
	Faculty string
}

// NewClient creates an API client seeded with the given cookies.
func NewClient(faculty string, cookies []*http.Cookie) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	if len(cookies) > 0 {
		u := &url.URL{Scheme: "https", Host: sigarraHost}
		jar.SetCookies(u, cookies)
	}

	return &Client{
		http: &http.Client{
			Jar:     jar,
			Timeout: RequestTimeout,
		},
		Faculty: faculty,
	}, nil
}

// BaseURL returns the base URL for a given faculty and language.
func BaseURL(faculty, lang string) string {
	return fmt.Sprintf("https://%s/%s/%s/", sigarraHost, faculty, lang)
}

// StudentNumber strips the "up" prefix from a SIGARRA username.
// pv_num_unico and pv_codigo expect the bare number.
func StudentNumber(username string) string {
	lower := strings.ToLower(username)
	if strings.HasPrefix(lower, "up") {
		return username[2:]
	}
	return username
}

func (c *Client) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create GET request: %w", err)
	}
	return c.http.Do(req)
}

func (c *Client) Post(ctx context.Context, rawURL string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.http.Do(req)
}

func (c *Client) Cookies() []*http.Cookie {
	u := &url.URL{Scheme: "https", Host: sigarraHost}
	return c.http.Jar.Cookies(u)
}
