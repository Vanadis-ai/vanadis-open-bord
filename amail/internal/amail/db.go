// Package amail implements the authenticated cross-machine mailbox
// server: connections are created by authorised users (the allowlist
// is managed via Jarl, or seeded at bootstrap), messages are stored
// single-read, and everything is keyed by opaque tokens so revocation
// costs a single DELETE.
package amail

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps the Postgres connection + schema migrations. Same style
// and conventions as relay (HashSecret for token storage, inline
// CREATE TABLE IF NOT EXISTS migration on start) so self-hosters
// don't have to learn a new pattern.
type DB struct {
	conn *sql.DB
}

// NewDB connects, pings, migrates. Returns a ready-to-use handle.
func NewDB(databaseURL string) (*DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS amail_allowlist (
			email       TEXT PRIMARY KEY,
			added_by    TEXT,
			added_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS amail_api_tokens (
			token_hash  TEXT PRIMARY KEY,
			email       TEXT NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_used   TIMESTAMPTZ,
			revoked_at  TIMESTAMPTZ,
			label       TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_amail_api_tokens_email
			ON amail_api_tokens(email) WHERE revoked_at IS NULL`,
		`CREATE TABLE IF NOT EXISTS amail_connections (
			id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			pair_code      TEXT UNIQUE,
			token_a_hash   TEXT NOT NULL,
			token_b_hash   TEXT,
			creator_email  TEXT NOT NULL,
			acceptor_email TEXT,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
			paired_at      TIMESTAMPTZ,
			expires_at     TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_amail_connections_pair_code
			ON amail_connections(pair_code) WHERE pair_code IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS amail_messages (
			id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			connection_id  UUID NOT NULL REFERENCES amail_connections(id) ON DELETE CASCADE,
			from_side      TEXT NOT NULL CHECK (from_side IN ('a','b')),
			text           TEXT NOT NULL,
			sent_at        TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_amail_messages_connection_sent
			ON amail_messages(connection_id, sent_at)`,
		`CREATE TABLE IF NOT EXISTS amail_oauth_states (
			state          TEXT PRIMARY KEY,
			code_verifier  TEXT NOT NULL,
			redirect_uri   TEXT NOT NULL,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, s := range stmts {
		if _, err := db.conn.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// --- allowlist ---

func (db *DB) IsEmailAllowed(email string) (bool, error) {
	var exists bool
	err := db.conn.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM amail_allowlist WHERE email = $1)`,
		email,
	).Scan(&exists)
	return exists, err
}

func (db *DB) AddAllowedEmail(email, addedBy string) error {
	_, err := db.conn.Exec(
		`INSERT INTO amail_allowlist (email, added_by) VALUES ($1, $2)
		 ON CONFLICT (email) DO NOTHING`,
		email, addedBy,
	)
	return err
}

func (db *DB) RemoveAllowedEmail(email string) error {
	_, err := db.conn.Exec(`DELETE FROM amail_allowlist WHERE email = $1`, email)
	return err
}

func (db *DB) ListAllowedEmails() ([]AllowlistEntry, error) {
	rows, err := db.conn.Query(
		`SELECT email, added_by, added_at FROM amail_allowlist ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AllowlistEntry
	for rows.Next() {
		var e AllowlistEntry
		if err := rows.Scan(&e.Email, &e.AddedBy, &e.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- api tokens ---

// IssueAPIToken creates a long-lived token for a user. Email must be
// in the allowlist before calling this. Returns the plain token
// (only time the caller sees it) plus its hash + record id.
func (db *DB) IssueAPIToken(email, label string) (plain string, err error) {
	plain, err = newSecret(32)
	if err != nil {
		return "", err
	}
	_, err = db.conn.Exec(
		`INSERT INTO amail_api_tokens (token_hash, email, label)
		 VALUES ($1, $2, $3)`,
		HashSecret(plain), email, label,
	)
	if err != nil {
		return "", fmt.Errorf("insert api token: %w", err)
	}
	return plain, nil
}

// ResolveAPIToken looks up an active (not revoked) token. Updates
// last_used async-is-fine on success.
func (db *DB) ResolveAPIToken(tokenHash string) (email string, err error) {
	err = db.conn.QueryRow(
		`SELECT email FROM amail_api_tokens
		  WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
	).Scan(&email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInvalidAPIToken
		}
		return "", fmt.Errorf("resolve api token: %w", err)
	}
	_, _ = db.conn.Exec(
		`UPDATE amail_api_tokens SET last_used = now() WHERE token_hash = $1`,
		tokenHash)
	return email, nil
}

// RevokeAPIToken marks a token as revoked. Idempotent.
func (db *DB) RevokeAPIToken(tokenHash string) error {
	_, err := db.conn.Exec(
		`UPDATE amail_api_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash)
	return err
}

// --- oauth state ---

func (db *DB) SaveOAuthState(state, verifier, redirect string) error {
	_, err := db.conn.Exec(
		`INSERT INTO amail_oauth_states (state, code_verifier, redirect_uri)
		 VALUES ($1, $2, $3)`,
		state, verifier, redirect,
	)
	return err
}

// ConsumeOAuthState returns (verifier, redirect_uri) and deletes the
// row so the state value is strictly single-use — a pre-condition
// for CSRF protection.
func (db *DB) ConsumeOAuthState(state string) (verifier, redirect string, err error) {
	err = db.conn.QueryRow(
		`DELETE FROM amail_oauth_states WHERE state = $1
		 RETURNING code_verifier, redirect_uri`,
		state,
	).Scan(&verifier, &redirect)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrInvalidOAuthState
		}
		return "", "", fmt.Errorf("consume oauth state: %w", err)
	}
	return verifier, redirect, nil
}

// --- connections ---

func (db *DB) CreateConnection(pairCode, creatorEmail string, ttl time.Duration) (connID, tokenA string, err error) {
	tokenA, err = newSecret(32)
	if err != nil {
		return "", "", err
	}
	expiresAt := time.Now().Add(ttl)
	err = db.conn.QueryRow(
		`INSERT INTO amail_connections (pair_code, token_a_hash, creator_email, expires_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		pairCode, HashSecret(tokenA), creatorEmail, expiresAt,
	).Scan(&connID)
	if err != nil {
		return "", "", fmt.Errorf("insert connection: %w", err)
	}
	return connID, tokenA, nil
}

func (db *DB) AcceptConnection(pairCode, acceptorEmail string) (connID, tokenB string, err error) {
	tokenB, err = newSecret(32)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	err = db.conn.QueryRow(
		`UPDATE amail_connections
		    SET token_b_hash = $1, paired_at = $2, pair_code = NULL,
		        expires_at = NULL, acceptor_email = $3
		  WHERE pair_code = $4 AND token_b_hash IS NULL
		    AND (expires_at IS NULL OR expires_at > $2)
		  RETURNING id`,
		HashSecret(tokenB), now, acceptorEmail, pairCode,
	).Scan(&connID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrInvalidPairCode
		}
		return "", "", fmt.Errorf("accept connection: %w", err)
	}
	return connID, tokenB, nil
}

func (db *DB) ResolveConnectionToken(tokenHash string) (connID, side string, err error) {
	err = db.conn.QueryRow(
		`SELECT id, CASE WHEN token_a_hash = $1 THEN 'a' ELSE 'b' END
		   FROM amail_connections
		  WHERE token_a_hash = $1 OR token_b_hash = $1`,
		tokenHash,
	).Scan(&connID, &side)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrInvalidConnectionToken
		}
		return "", "", fmt.Errorf("resolve connection token: %w", err)
	}
	return connID, side, nil
}

func (db *DB) GetConnection(connID string) (*Connection, error) {
	c := &Connection{}
	err := db.conn.QueryRow(
		`SELECT id, pair_code, creator_email, acceptor_email, paired_at
		   FROM amail_connections WHERE id = $1`,
		connID,
	).Scan(&c.ID, &c.PairCode, &c.CreatorEmail, &c.AcceptorEmail, &c.PairedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConnectionNotFound
		}
		return nil, fmt.Errorf("get connection: %w", err)
	}
	return c, nil
}

func (db *DB) InsertMessage(connID, fromSide, text string) (msgID string, err error) {
	err = db.conn.QueryRow(
		`INSERT INTO amail_messages (connection_id, from_side, text)
		 VALUES ($1, $2, $3) RETURNING id`,
		connID, fromSide, text,
	).Scan(&msgID)
	if err != nil {
		return "", fmt.Errorf("insert message: %w", err)
	}
	return msgID, nil
}

func (db *DB) FetchAndDeleteInbox(connID, mySide string) ([]Message, error) {
	peerSide := "b"
	if mySide == "b" {
		peerSide = "a"
	}
	rows, err := db.conn.Query(
		`DELETE FROM amail_messages
		  WHERE connection_id = $1 AND from_side = $2
		  RETURNING id, connection_id, from_side, text, sent_at`,
		connID, peerSide,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch-delete inbox: %w", err)
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConnectionID, &m.FromSide, &m.Text, &m.SentAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (db *DB) DeleteConnection(connID string) error {
	_, err := db.conn.Exec(`DELETE FROM amail_connections WHERE id = $1`, connID)
	return err
}

func (db *DB) CleanupExpiredPairCodes() (int64, error) {
	res, err := db.conn.Exec(
		`DELETE FROM amail_connections
		  WHERE expires_at IS NOT NULL
		    AND expires_at < now() AND paired_at IS NULL`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- primitives ---

// HashSecret returns SHA256 hex of a secret. Safe for opaque 256-bit
// random values; not a password hash.
func HashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

func newSecret(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// --- types ---

type AllowlistEntry struct {
	Email   string    `json:"email"`
	AddedBy string    `json:"added_by"`
	AddedAt time.Time `json:"added_at"`
}

type Connection struct {
	ID            string         `json:"id"`
	PairCode      sql.NullString `json:"-"`
	CreatorEmail  string         `json:"creator_email"`
	AcceptorEmail sql.NullString `json:"-"`
	PairedAt      sql.NullTime   `json:"-"`
}

type Message struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	FromSide     string    `json:"from_side"`
	Text         string    `json:"text"`
	SentAt       time.Time `json:"sent_at"`
}

// --- sentinel errors ---

var (
	ErrInvalidPairCode        = errors.New("invalid or expired pair code")
	ErrInvalidAPIToken        = errors.New("invalid api token")
	ErrInvalidConnectionToken = errors.New("invalid connection token")
	ErrInvalidOAuthState      = errors.New("invalid oauth state")
	ErrConnectionNotFound     = errors.New("connection not found")
)
