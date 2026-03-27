package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const releaseAPI = "https://api.github.com/repos/tiagoabreu22/shigarra/releases/latest"

type Result struct {
	LatestVersion   string
	UpdateAvailable bool
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func CheckLatest(ctx context.Context, current string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "shigarra/"+current)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Result{}, err
	}

	latest := stripV(release.TagName)
	if !isNewer(stripV(current), latest) {
		return Result{LatestVersion: latest}, nil
	}

	return Result{LatestVersion: latest, UpdateAvailable: true}, nil
}

func UpdateCommand(installMethod string) string {
	switch installMethod {
	case "homebrew":
		return "brew upgrade shigarra"
	case "chocolatey":
		return "choco upgrade shigarra"
	default:
		return "go install github.com/tiagoabreu22/shigarra@latest"
	}
}

func stripV(v string) string {
	return strings.TrimPrefix(v, "v")
}

func isNewer(current, candidate string) bool {
	if current == "dev" || candidate == "dev" || current == "" || candidate == "" {
		return false
	}
	cv, err1 := parseSemver(current)
	av, err2 := parseSemver(candidate)
	if err1 != nil || err2 != nil {
		return current != candidate
	}
	for i := 0; i < 3; i++ {
		if av[i] > cv[i] {
			return true
		}
		if av[i] < cv[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) ([3]int, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("not semver: %q", v)
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, err
		}
		out[i] = n
	}
	return out, nil
}
