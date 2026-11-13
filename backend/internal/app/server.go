package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"ctc/internal/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Active bool   `json:"active"`
}
type session struct {
	User User
	CSRF string
}
type limiterEntry struct {
	Count int
	Until time.Time
}
type App struct {
	DB       *pgxpool.Pool
	Origin   string
	Demo     bool
	mu       sync.Mutex
	attempts map[string]limiterEntry
}
type handler func(http.ResponseWriter, *http.Request, session)

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		fail(w, 415, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	d := json.NewDecoder(r.Body)
	if e := d.Decode(v); e != nil {
		fail(w, 400, "Invalid request: "+e.Error())
		return false
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		fail(w, 400, "Request must contain exactly one valid JSON value")
		return false
	}
	return true
}
func (a *App) dbError(w http.ResponseWriter, e error) {
	log.Printf("database operation failed: %v", e)
	fail(w, 500, "The request could not be completed; please retry")
}
func (a *App) audit(ctx context.Context, actor, action, kind, id string, details any) error {
	b, _ := json.Marshal(details)
	_, e := a.DB.Exec(ctx, "INSERT INTO audit_events(id,actor_id,action,entity_type,entity_id,details) VALUES($1,$2,$3,$4,$5,$6)", ID(), nullable(actor), action, kind, id, b)
	return e
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func decimalNullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func (a *App) getSession(r *http.Request) (session, error) {
	var s session
	c, e := r.Cookie("ctc_session")
	if e != nil {
		return s, e
	}
	e = a.DB.QueryRow(r.Context(), `SELECT u.id,u.name,u.email,u.role,u.active,s.csrf_token FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.active=true`, hash(c.Value)).Scan(&s.User.ID, &s.User.Name, &s.User.Email, &s.User.Role, &s.User.Active, &s.CSRF)
	return s, e
}
func hasRole(role string, roles []string) bool {
	for _, v := range roles {
		if role == v {
			return true
		}
	}
	return false
}
func (a *App) protect(fn handler, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, e := a.getSession(r)
		if e != nil {
			fail(w, 401, "Please sign in")
			return
		}
		if len(roles) > 0 && !hasRole(s.User.Role, roles) {
			fail(w, 403, "Your role does not permit this action")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != a.Origin {
				fail(w, 403, "Origin is not allowed")
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.CSRF)) != 1 {
				fail(w, 403, "Invalid CSRF token")
				return
			}
		}
		fn(w, r, s)
	}
}
func (a *App) Handler() http.Handler {
	m := http.NewServeMux()
	health := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if e := a.DB.Ping(ctx); e != nil {
			fail(w, 503, "Database unavailable")
			return
		}
		JSON(w, 200, map[string]string{"status": "ok", "engineVersion": engine.Version})
	}
	m.HandleFunc("GET /api/health", health)
	m.HandleFunc("GET /health", health)
	m.HandleFunc("POST /api/auth/login", a.login)
	m.HandleFunc("GET /api/auth/me", a.protect(func(w http.ResponseWriter, r *http.Request, s session) {
		JSON(w, 200, map[string]any{"user": s.User, "csrfToken": s.CSRF})
	}))
	m.HandleFunc("POST /api/auth/logout", a.protect(a.logout))
	m.HandleFunc("GET /api/bootstrap", a.protect(a.bootstrap))
	m.HandleFunc("GET /api/configurations", a.protect(a.listConfigs))
	m.HandleFunc("POST /api/configurations", a.protect(a.createConfig, "cb"))
	m.HandleFunc("POST /api/configurations/{id}/publish", a.protect(a.publishConfig, "cb"))
	m.HandleFunc("POST /api/calculate", a.protect(a.calculate, "hr", "cb", "admin"))
	m.HandleFunc("GET /api/cases", a.protect(a.listCases))
	m.HandleFunc("POST /api/cases", a.protect(a.saveCase, "hr", "cb", "admin"))
	m.HandleFunc("GET /api/cases/{id}", a.protect(a.getCase))
	m.HandleFunc("GET /api/revisions/{id}/documents/{kind}", a.protect(a.document))
	m.HandleFunc("GET /api/users", a.protect(a.listUsers, "admin"))
	m.HandleFunc("POST /api/users", a.protect(a.createUser, "admin"))
	m.HandleFunc("PATCH /api/users/{id}", a.protect(a.updateUser, "admin"))
	m.HandleFunc("GET /api/audit", a.protect(a.listAudit))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		if !a.Demo {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		defer func() {
			if v := recover(); v != nil {
				log.Printf("handler panic: %v", v)
				fail(w, 500, "The request could not be completed")
			}
		}()
		m.ServeHTTP(w, r)
	})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && origin != a.Origin {
		fail(w, 403, "Origin is not allowed")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	a.mu.Lock()
	if a.attempts == nil {
		a.attempts = map[string]limiterEntry{}
	}
	entry := a.attempts[ip]
	if time.Now().After(entry.Until) {
		entry = limiterEntry{Until: time.Now().Add(time.Minute)}
	}
	entry.Count++
	a.attempts[ip] = entry
	a.mu.Unlock()
	if entry.Count > 30 {
		fail(w, 429, "Too many sign-in attempts; retry in one minute")
		return
	}
	var u User
	var passwordHash string
	e := a.DB.QueryRow(r.Context(), "SELECT id,name,email,role,active,password_hash FROM users WHERE email=$1", strings.ToLower(strings.TrimSpace(in.Email))).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &passwordHash)
	if e != nil || !u.Active || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(in.Password)) != nil {
		fail(w, 401, "Invalid email or password")
		return
	}
	raw := token()
	csrf := token()
	_, e = a.DB.Exec(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,$2,$3,$4)", hash(raw), u.ID, csrf, time.Now().Add(8*time.Hour))
	if e != nil {
		a.dbError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "ctc_session", Value: raw, Path: "/", HttpOnly: true, Secure: !a.Demo, SameSite: http.SameSiteLaxMode, MaxAge: 28800})
	_ = a.audit(r.Context(), u.ID, "auth.login", "user", u.ID, map[string]string{"result": "success"})
	JSON(w, 200, map[string]any{"user": u, "csrfToken": csrf})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request, s session) {
	c, _ := r.Cookie("ctc_session")
	if c != nil {
		_, _ = a.DB.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "ctc_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: !a.Demo, SameSite: http.SameSiteLaxMode})
	JSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) bootstrap(w http.ResponseWriter, r *http.Request, s session) {
	configs, e := a.configs(r.Context())
	if e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 200, map[string]any{"user": s.User, "csrfToken": s.CSRF, "configurations": configs, "components": engine.Components, "clarifications": engine.Clarifications})
}
func (a *App) listUsers(w http.ResponseWriter, r *http.Request, s session) {
	rows, e := a.DB.Query(r.Context(), "SELECT id,name,email,role,active FROM users ORDER BY name")
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active); e != nil {
			a.dbError(w, e)
			return
		}
		users = append(users, u)
	}
	JSON(w, 200, users)
}
func validRole(role string) bool  { return hasRole(role, []string{"hr", "cb", "admin", "auditor"}) }
func validPassword(p string) bool { return len(p) >= 12 && len(p) <= 72 }
func (a *App) createUser(w http.ResponseWriter, r *http.Request, s session) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || !strings.Contains(in.Email, "@") || !validRole(in.Role) || !validPassword(in.Password) {
		fail(w, 400, "Name, email, valid role and a 12-72 character password are required")
		return
	}
	ph, e := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if e != nil {
		a.dbError(w, e)
		return
	}
	u := User{ID(), in.Name, in.Email, in.Role, true}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer tx.Rollback(r.Context())
	_, e = tx.Exec(r.Context(), "INSERT INTO users(id,name,email,password_hash,role) VALUES($1,$2,$3,$4,$5)", u.ID, u.Name, u.Email, string(ph), u.Role)
	if e == nil {
		e = auditTx(r.Context(), tx, s.User.ID, "user.create", "user", u.ID, map[string]string{"email": u.Email, "role": u.Role})
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		if strings.Contains(e.Error(), "23505") {
			fail(w, 409, "A user with this email already exists")
		} else {
			a.dbError(w, e)
		}
		return
	}
	JSON(w, 201, u)
}
func (a *App) updateUser(w http.ResponseWriter, r *http.Request, s session) {
	var in struct {
		Name     *string `json:"name"`
		Role     *string `json:"role"`
		Active   *bool   `json:"active"`
		Password *string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	if in.Role != nil && !validRole(*in.Role) {
		fail(w, 400, "Invalid role")
		return
	}
	if in.Password != nil && !validPassword(*in.Password) {
		fail(w, 400, "Password must contain 12-72 characters")
		return
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		fail(w, 400, "Name cannot be empty")
		return
	}
	if id == s.User.ID && ((in.Active != nil && !*in.Active) || (in.Role != nil && *in.Role != "admin")) {
		fail(w, 400, "You cannot deactivate or remove your own administrator role")
		return
	}
	var u User
	e := a.DB.QueryRow(r.Context(), "SELECT id,name,email,role,active FROM users WHERE id=$1", id).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "User not found")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	before := u
	if in.Name != nil {
		u.Name = *in.Name
	}
	if in.Role != nil {
		u.Role = *in.Role
	}
	if in.Active != nil {
		u.Active = *in.Active
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer tx.Rollback(r.Context())
	_, e = tx.Exec(r.Context(), "UPDATE users SET name=$2,role=$3,active=$4,updated_at=now() WHERE id=$1", id, u.Name, u.Role, u.Active)
	if e == nil && in.Password != nil {
		var ph []byte
		ph, e = bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if e == nil {
			_, e = tx.Exec(r.Context(), "UPDATE users SET password_hash=$2 WHERE id=$1", id, string(ph))
		}
	}
	if e == nil && (in.Role != nil || in.Active != nil || in.Password != nil) {
		_, e = tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id=$1", id)
	}
	if e == nil {
		e = auditTx(r.Context(), tx, s.User.ID, "user.update", "user", id, map[string]any{"before": before, "after": u, "passwordChanged": in.Password != nil})
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		a.dbError(w, e)
		return
	}

	JSON(w, 200, u)
}
func (a *App) listAudit(w http.ResponseWriter, r *http.Request, s session) {
	rows, e := a.DB.Query(r.Context(), `SELECT a.id,COALESCE(u.name,'System'),a.action,a.entity_type,a.entity_id,a.details,a.created_at FROM audit_events a LEFT JOIN users u ON u.id=a.actor_id WHERE ($1<>'hr' OR a.actor_id=$2) ORDER BY a.created_at DESC LIMIT 500`, s.User.Role, s.User.ID)
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, actor, action, kind, eid string
		var details []byte
		var at time.Time
		if e = rows.Scan(&id, &actor, &action, &kind, &eid, &details, &at); e != nil {
			a.dbError(w, e)
			return
		}
		var d any
		_ = json.Unmarshal(details, &d)
		out = append(out, map[string]any{"id": id, "actorName": actor, "action": action, "entityType": kind, "entityId": eid, "details": d, "createdAt": at.Format(time.RFC3339)})
	}
	JSON(w, 200, out)
}
func New(ctx context.Context) (*App, error) {
	env := os.Getenv("APP_ENV")
	if env != "demo" && env != "production" {
		return nil, fmt.Errorf("APP_ENV must explicitly be demo or production")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	pool, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, e
	}
	origin := os.Getenv("APP_ORIGIN")
	if origin == "" {
		origin = "http://localhost:3020"
	}
	a := &App{DB: pool, Origin: strings.TrimSuffix(origin, "/"), Demo: env == "demo"}
	if e = a.initialize(ctx); e != nil {
		pool.Close()
		return nil, e
	}
	return a, nil
}
