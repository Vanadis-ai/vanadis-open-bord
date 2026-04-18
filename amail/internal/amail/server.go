package amail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config holds everything Server needs at runtime. GoogleClient* are
// optional — when unset, the /auth/google endpoints return 501 and
// self-hosters can still seed API tokens via the seed binary.
type Config struct {
	DB                 *DB
	PublicURL          string // "https://amail.vanadis.ai"
	GoogleClientID     string
	GoogleClientSecret string
	HTTPClient         *http.Client // optional (test override)
}

type Server struct {
	cfg Config
	hc  *http.Client
}

func NewServer(cfg Config) *Server {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Server{cfg: cfg, hc: hc}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)

	// Google OAuth — the plugin's /amail-auth command funnels through here.
	mux.HandleFunc("GET /auth/google/start", s.googleStart)
	mux.HandleFunc("GET /auth/google/callback", s.googleCallback)

	// API-token management (user-scoped).
	mux.HandleFunc("POST /api/tokens", s.requireUser(s.issueToken))
	mux.HandleFunc("DELETE /api/tokens/current", s.requireUser(s.revokeCurrentToken))

	// Admin: allowlist CRUD. Guarded by requireAdmin (superadmin email).
	mux.HandleFunc("GET /admin/allowlist", s.requireAdmin(s.listAllowlist))
	mux.HandleFunc("POST /admin/allowlist", s.requireAdmin(s.addAllowlist))
	mux.HandleFunc("DELETE /admin/allowlist/{email}", s.requireAdmin(s.removeAllowlist))

	// Mailbox endpoints. All require a valid API token.
	mux.HandleFunc("POST /amail/connection/create", s.requireUser(s.createConnection))
	mux.HandleFunc("POST /amail/connection/accept", s.requireUser(s.acceptConnection))
	mux.HandleFunc("GET /amail/connection/status", s.requireConn(s.connectionStatus))
	mux.HandleFunc("DELETE /amail/connection", s.requireConn(s.disconnectConnection))
	mux.HandleFunc("POST /amail/send", s.requireConn(s.sendMessage))
	mux.HandleFunc("GET /amail/inbox", s.requireConn(s.readInbox))

	return withCORS(mux)
}

// StartCleanupLoop runs the expired-pair-code cleanup forever. Call
// in a goroutine from main.
func (s *Server) StartCleanupLoop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		n, err := s.cfg.DB.CleanupExpiredPairCodes()
		if err != nil {
			log.Printf("cleanup: %v", err)
			continue
		}
		if n > 0 {
			log.Printf("cleanup: removed %d expired pair-code(s)", n)
		}
	}
}

// --- middleware ---

type userCtxKey struct{}
type connCtxKey struct{}

type userContext struct {
	Email     string
	TokenHash string
}
type connContext struct {
	ConnectionID string
	Side         string
	Email        string
}

func (s *Server) requireUser(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		email, err := s.cfg.DB.ResolveAPIToken(HashSecret(token))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey{}, userContext{Email: email, TokenHash: HashSecret(token)})
		h(w, r.WithContext(ctx))
	}
}

func (s *Server) requireConn(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		connID, side, err := s.cfg.DB.ResolveConnectionToken(HashSecret(token))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// connection-token flow doesn't know the owner email, but we
		// can fetch it cheaply from the row itself.
		conn, err := s.cfg.DB.GetConnection(connID)
		if err != nil {
			http.Error(w, "connection not found", http.StatusNotFound)
			return
		}
		email := conn.CreatorEmail
		if side == "b" && conn.AcceptorEmail.Valid {
			email = conn.AcceptorEmail.String
		}
		ctx := context.WithValue(r.Context(), connCtxKey{}, connContext{
			ConnectionID: connID, Side: side, Email: email,
		})
		h(w, r.WithContext(ctx))
	}
}

func (s *Server) requireAdmin(h http.HandlerFunc) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		u, _ := r.Context().Value(userCtxKey{}).(userContext)
		if !isAdminEmail(u.Email) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	})
}

// isAdminEmail is intentionally simple: admin == Pasha. Add more
// emails here once the instance actually has other admins.
func isAdminEmail(email string) bool {
	switch email {
	case "pasha@vanadis.ai", "ministratio@gmail.com":
		return true
	}
	return false
}

// --- basic handlers ---

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// --- Google OAuth flow ---
//
// Installed-app (loopback) variant: the CLI opens a local HTTP
// listener on a random port, then opens the browser at our
// /auth/google/start?redirect_uri=http://localhost:<port>/callback.
// We mint a state + PKCE verifier, save them, and 302 the user to
// Google. Google redirects back to /auth/google/callback with ?code.
// We exchange the code for a userinfo email, check allowlist, issue
// an API token, and redirect to the CLI's local listener with the
// token as a query parameter. CLI picks up the token, closes the
// listener, and stores the token.

func (s *Server) googleStart(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GoogleClientID == "" {
		http.Error(w, "google oauth not configured", http.StatusNotImplemented)
		return
	}
	redirect := r.URL.Query().Get("redirect_uri")
	if !validLoopbackRedirect(redirect) {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	state, verifier, challenge, err := mintPKCE()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.cfg.DB.SaveOAuthState(state, verifier, redirect); err != nil {
		log.Printf("oauth save state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	google := url.Values{}
	google.Set("client_id", s.cfg.GoogleClientID)
	google.Set("redirect_uri", s.cfg.PublicURL+"/auth/google/callback")
	google.Set("response_type", "code")
	google.Set("scope", "openid email profile")
	google.Set("state", state)
	google.Set("code_challenge", challenge)
	google.Set("code_challenge_method", "S256")
	google.Set("access_type", "online")
	google.Set("prompt", "select_account")
	target := "https://accounts.google.com/o/oauth2/v2/auth?" + google.Encode()
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GoogleClientID == "" || s.cfg.GoogleClientSecret == "" {
		http.Error(w, "google oauth not configured", http.StatusNotImplemented)
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	verifier, loopbackRedirect, err := s.cfg.DB.ConsumeOAuthState(state)
	if err != nil {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	email, err := s.exchangeGoogleCode(code, verifier)
	if err != nil {
		log.Printf("oauth exchange: %v", err)
		http.Error(w, "oauth exchange failed", http.StatusBadRequest)
		return
	}
	allowed, err := s.cfg.DB.IsEmailAllowed(email)
	if err != nil {
		log.Printf("allowlist check: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !allowed {
		s.renderHTML(w, 403, fmt.Sprintf(
			"<h2>Access denied</h2><p>%s is not on the allowlist. Ask the admin to add you, then try again.</p>",
			htmlEscape(email)))
		return
	}
	token, err := s.cfg.DB.IssueAPIToken(email, "browser-login")
	if err != nil {
		log.Printf("issue token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	dst, err := url.Parse(loopbackRedirect)
	if err != nil {
		http.Error(w, "bad redirect", http.StatusBadRequest)
		return
	}
	q := dst.Query()
	q.Set("token", token)
	q.Set("email", email)
	dst.RawQuery = q.Encode()
	http.Redirect(w, r, dst.String(), http.StatusFound)
}

func (s *Server) exchangeGoogleCode(code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("client_id", s.cfg.GoogleClientID)
	form.Set("client_secret", s.cfg.GoogleClientSecret)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", s.cfg.PublicURL+"/auth/google/callback")
	resp, err := s.hc.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token endpoint %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", err
	}
	return fetchGoogleEmail(s.hc, tok.AccessToken)
}

func fetchGoogleEmail(hc *http.Client, accessToken string) (string, error) {
	req, _ := http.NewRequest("GET", "https://openidconnect.googleapis.com/v1/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("userinfo %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var u struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	if u.Email == "" || !u.EmailVerified {
		return "", errors.New("email missing or unverified")
	}
	return strings.ToLower(u.Email), nil
}

// --- API token lifecycle ---

func (s *Server) issueToken(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userCtxKey{}).(userContext)
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	token, err := s.cfg.DB.IssueAPIToken(u.Email, body.Label)
	if err != nil {
		log.Printf("issue token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"token": token})
}

func (s *Server) revokeCurrentToken(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userCtxKey{}).(userContext)
	_ = s.cfg.DB.RevokeAPIToken(u.TokenHash)
	writeJSON(w, map[string]bool{"revoked": true})
}

// --- allowlist admin ---

func (s *Server) listAllowlist(w http.ResponseWriter, _ *http.Request) {
	list, err := s.cfg.DB.ListAllowedEmails()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"emails": list})
}

func (s *Server) addAllowlist(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userCtxKey{}).(userContext)
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if err := s.cfg.DB.AddAllowedEmail(email, u.Email); err != nil {
		log.Printf("add allowlist: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) removeAllowlist(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(r.PathValue("email"))
	if err := s.cfg.DB.RemoveAllowedEmail(email); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// --- mailbox endpoints ---

const (
	PairCodeTTL      = 15 * time.Minute
	PairCodeLen      = 4
	PairCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	MaxMessageBytes  = 32 * 1024
)

func (s *Server) createConnection(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userCtxKey{}).(userContext)
	code, err := generatePairCode()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	connID, tokenA, err := s.cfg.DB.CreateConnection(code, u.Email, PairCodeTTL)
	if err != nil {
		log.Printf("create connection: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"connection_id":  connID,
		"pair_code":      code,
		"token":          tokenA,
		"expires_in_sec": int(PairCodeTTL.Seconds()),
	})
}

func (s *Server) acceptConnection(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userCtxKey{}).(userContext)
	var body struct {
		PairCode string `json:"pair_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PairCode == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	connID, tokenB, err := s.cfg.DB.AcceptConnection(strings.ToUpper(body.PairCode), u.Email)
	if err != nil {
		if errors.Is(err, ErrInvalidPairCode) {
			http.Error(w, "invalid or expired pair code", http.StatusNotFound)
			return
		}
		log.Printf("accept: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"connection_id": connID,
		"token":         tokenB,
	})
}

func (s *Server) connectionStatus(w http.ResponseWriter, r *http.Request) {
	cc := r.Context().Value(connCtxKey{}).(connContext)
	conn, err := s.cfg.DB.GetConnection(cc.ConnectionID)
	if err != nil {
		http.Error(w, "connection not found", http.StatusNotFound)
		return
	}
	resp := map[string]any{
		"connection_id": conn.ID,
		"paired":        conn.PairedAt.Valid,
		"my_email":      cc.Email,
	}
	if conn.PairedAt.Valid {
		resp["paired_at"] = conn.PairedAt.Time
		if cc.Side == "a" && conn.AcceptorEmail.Valid {
			resp["peer_email"] = conn.AcceptorEmail.String
		} else if cc.Side == "b" {
			resp["peer_email"] = conn.CreatorEmail
		}
	}
	writeJSON(w, resp)
}

func (s *Server) disconnectConnection(w http.ResponseWriter, r *http.Request) {
	cc := r.Context().Value(connCtxKey{}).(connContext)
	if err := s.cfg.DB.DeleteConnection(cc.ConnectionID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"deleted": true})
}

func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	cc := r.Context().Value(connCtxKey{}).(connContext)
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxMessageBytes+1024)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	if len(body.Text) > MaxMessageBytes {
		http.Error(w, "message too large", http.StatusRequestEntityTooLarge)
		return
	}
	conn, err := s.cfg.DB.GetConnection(cc.ConnectionID)
	if err != nil || !conn.PairedAt.Valid {
		http.Error(w, "connection not paired", http.StatusConflict)
		return
	}
	msgID, err := s.cfg.DB.InsertMessage(cc.ConnectionID, cc.Side, body.Text)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"message_id": msgID})
}

func (s *Server) readInbox(w http.ResponseWriter, r *http.Request) {
	cc := r.Context().Value(connCtxKey{}).(connContext)
	msgs, err := s.cfg.DB.FetchAndDeleteInbox(cc.ConnectionID, cc.Side)
	if err != nil {
		log.Printf("inbox: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	resp := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		resp = append(resp, map[string]any{
			"id":            m.ID,
			"connection_id": m.ConnectionID,
			"from_side":     m.FromSide,
			"text":          m.Text,
			"sent_at":       m.SentAt,
		})
	}
	writeJSON(w, map[string]any{"messages": resp})
}

// --- helpers ---

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func validLoopbackRedirect(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	if p.Scheme != "http" && p.Scheme != "https" {
		return false
	}
	host := p.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func mintPKCE() (state, verifier, challenge string, err error) {
	state, err = newSecret(16)
	if err != nil {
		return "", "", "", err
	}
	verifier, err = newSecret(32)
	if err != nil {
		return "", "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return state, verifier, challenge, nil
}

func generatePairCode() (string, error) {
	max := big.NewInt(int64(len(PairCodeAlphabet)))
	out := make([]byte, PairCodeLen)
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = PairCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) renderHTML(w http.ResponseWriter, status int, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<!doctype html><html><body style=\"font-family:system-ui;padding:2em;max-width:40em;margin:auto\">%s</body></html>", html)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

// withCORS is permissive for /auth and /admin (Jarl) but the service
// is fine being accessed from anywhere for /amail/* since auth is
// bearer-token based.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
