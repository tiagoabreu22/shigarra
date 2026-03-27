package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/tiagoabreu22/shigarra/internal/api"
	"github.com/tiagoabreu22/shigarra/internal/auth"
	"github.com/tiagoabreu22/shigarra/internal/config"
	"github.com/tiagoabreu22/shigarra/internal/ui"
)

// version and installMethod are set at build time via ldflags.
var (
	version       = "dev"
	installMethod = "other"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("shigarra %s\n", version)
			return
		case "dump":
			if len(os.Args) < 3 {
				log.Fatal("usage: shigarra dump <schedule|exams>")
			}
			if err := runDump(os.Args[2]); err != nil {
				log.Fatal(err)
			}
			return
		}
	}
	if err := ui.Run(version, installMethod); err != nil {
		log.Fatal(err)
	}
}

func runDump(what string) error {
	sess, err := config.Load()
	if err != nil || sess == nil {
		return fmt.Errorf("no saved session! run shigarra first to log in")
	}

	if !config.AuthIsConfigured(sess) {
		return fmt.Errorf("credential backend is not configured yet! run shigarra and complete first setup")
	}

	manager, err := auth.NewSessionManager(config.ResolveAuthBackend(sess))
	if err != nil {
		return fmt.Errorf("no credential backend available: %w", err)
	}

	secrets, err := manager.LoadSessionSecrets(sess.Faculty, sess.Username)
	if err != nil || secrets == nil {
		return fmt.Errorf("no saved secrets! run shigarra first to log in")
	}	

	client, err := api.NewClient(sess.Faculty, auth.CookiesToHTTP(secrets.Cookies))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	switch what {
	case "schedule":
		ctx, cancel := context.WithTimeout(context.Background(), api.RequestTimeout)
		lectures, err := api.FetchSchedule(ctx, client, sess.Username)
		cancel()
		if err != nil {
			return err
		}
		return enc.Encode(lectures)

	case "exams":
		profileCtx, profileCancel := context.WithTimeout(context.Background(), api.RequestTimeout)
		profile, err := api.FetchProfile(profileCtx, client, sess.Username)
		profileCancel()
		if err != nil {
			return err
		}

		ids := make([]int, 0, len(profile.Courses))
		for _, c := range profile.Courses {
			ids = append(ids, c.ID)
		}

		examsCtx, examsCancel := context.WithTimeout(context.Background(), api.RequestTimeout)
		exams, err := api.FetchExams(examsCtx, client, ids)
		examsCancel()
		if err != nil {
			return err
		}
		return enc.Encode(exams)

	default:
		return fmt.Errorf("unknown dump target %q — use 'schedule' or 'exams'", what)
	}
}
