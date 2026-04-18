// seed bootstraps the amail allowlist and issues API tokens for the
// starter emails. Run once after a fresh deploy, before Google OAuth
// is wired up, so admin + test users can talk to the service.
//
// Usage:
//   DATABASE_URL=... go run ./cmd/seed pasha@vanadis.ai ministratio@gmail.com ...
//
// Tokens are printed on stdout — copy-paste into client config and
// keep them safe (plaintext is only visible here).
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/vanadis-ai/amail/internal/amail"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: seed <email> [email ...]")
		os.Exit(2)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := amail.NewDB(dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	for _, email := range os.Args[1:] {
		if err := db.AddAllowedEmail(email, "seed"); err != nil {
			log.Printf("add %s: %v", email, err)
			continue
		}
		tok, err := db.IssueAPIToken(email, "seed")
		if err != nil {
			log.Printf("issue %s: %v", email, err)
			continue
		}
		fmt.Printf("%s\t%s\n", email, tok)
	}
}
