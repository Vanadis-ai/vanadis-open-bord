package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vanadis-ai/amail/internal/amail"
)

// amail — authenticated cross-machine mailbox for Claude Code (and
// any HTTP client that can carry a Bearer token). Self-hostable, no
// external dependencies except Postgres and Google OAuth.
//
// This is main.go for the live service; for local development see
// README.md (docker-compose brings up Postgres + amail + a small
// mock OAuth for integration tests).
func main() {
	databaseURL := mustEnv("DATABASE_URL")
	listen := envOr("LISTEN_ADDR", ":8080")
	publicURL := envOr("PUBLIC_URL", "https://amail.vanadis.ai")
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	db, err := amail.NewDB(databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	srv := amail.NewServer(amail.Config{
		DB:                 db,
		PublicURL:          publicURL,
		GoogleClientID:     googleClientID,
		GoogleClientSecret: googleClientSecret,
	})

	// Cleanup loop: prune expired pair codes once a minute. Kept
	// inside the service so self-hosters don't have to wire up a
	// cron job separately.
	go srv.StartCleanupLoop(time.Minute)

	log.Printf("amail listening on %s (public: %s)", listen, publicURL)
	log.Fatal(http.ListenAndServe(listen, srv.Router()))
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
