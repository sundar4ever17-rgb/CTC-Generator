package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"ctc/internal/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	Active             bool   `json:"active"`
	MustChangePassword bool   `json:"mustChangePassword"`
}
type session struct {
	User User
	CSRF string
}
type App struct {
	DB       *pgxpool.Pool
	Origin   string
	Demo     bool
	pdfSlots chan struct{}
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
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		fail(w, 415, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
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
	code := "unavailable"
	var pgerr *pgconn.PgError
	if errors.As(e, &pgerr) {
		code = pgerr.Code
	}
	slog.Error("database operation failed", "code", code, "request_id", w.Header().Get("X-Request-ID"))
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
	if len(c.Value) != 64 {
		return s, pgx.ErrNoRows
	}
	e = a.DB.QueryRow(r.Context(), `UPDATE sessions s SET last_seen_at=now() FROM users u,application_settings p WHERE p.id=1 AND s.user_id=u.id AND s.token_hash=$1 AND s.expires_at>now() AND s.created_at>now()-make_interval(hours=>p.session_max_hours) AND s.last_seen_at>now()-make_interval(mins=>p.session_idle_minutes) AND u.active=true AND s.auth_version=u.auth_version RETURNING u.id,u.name,u.email,u.role,u.active,u.must_change_password,s.csrf_token`, hash(c.Value)).Scan(&s.User.ID, &s.User.Name, &s.User.Email, &s.User.Role, &s.User.Active, &s.User.MustChangePassword, &s.CSRF)
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
		if s.User.MustChangePassword && r.URL.Path != "/api/auth/me" && r.URL.Path != "/api/auth/logout" && r.URL.Path != "/api/auth/password" {
			fail(w, 403, "Change your temporary password before continuing")
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
	m.HandleFunc("GET /api/live", func(w http.ResponseWriter, r *http.Request) { JSON(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("POST /api/auth/login", a.login)
	m.HandleFunc("GET /api/auth/me", a.protect(func(w http.ResponseWriter, r *http.Request, s session) {
		JSON(w, 200, map[string]any{"user": s.User, "csrfToken": s.CSRF})
	}))
	m.HandleFunc("POST /api/auth/logout", a.protect(a.logout))
	m.HandleFunc("POST /api/auth/password", a.protect(a.changePassword))
	m.HandleFunc("GET /api/bootstrap", a.protect(a.bootstrap))
	m.HandleFunc("GET /api/configurations", a.protect(a.listConfigs))
	m.HandleFunc("GET /api/configurations/{id}", a.protect(a.getConfig))
	m.HandleFunc("POST /api/configurations", a.protect(a.createConfig, "cb"))
	m.HandleFunc("POST /api/configurations/{id}/publish", a.protect(a.publishConfig, "cb"))
	m.HandleFunc("POST /api/calculate", a.protect(a.calculate, "hr", "cb", "admin"))
	m.HandleFunc("GET /api/cases", a.protect(a.listCases))
	m.HandleFunc("POST /api/cases", a.protect(a.saveCase, "hr", "cb", "admin"))
	m.HandleFunc("GET /api/cases/{id}", a.protect(a.getCase))
	m.HandleFunc("PATCH /api/cases/{id}/owner", a.protect(a.assignCase, "admin"))
	m.HandleFunc("GET /api/revisions/{id}/documents/{kind}", a.protect(a.document))
	m.HandleFunc("GET /api/users", a.protect(a.listUsers, "admin"))
	m.HandleFunc("POST /api/users", a.protect(a.createUser, "admin"))
	m.HandleFunc("PATCH /api/users/{id}", a.protect(a.updateUser, "admin"))
	m.HandleFunc("GET /api/audit", a.protect(a.listAudit))
	m.HandleFunc("GET /api/settings", a.protect(a.getSettings, "admin"))
	m.HandleFunc("PATCH /api/settings", a.protect(a.updateSettings, "admin"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		w.Header().Set("X-Request-ID", ID())
		recorded := &responseStatus{ResponseWriter: w, status: 200}
		w = recorded
		defer func() {
			if r.URL.Path != "/api/health" && r.URL.Path != "/health" && r.URL.Path != "/api/live" {
				slog.Info("http_request", "request_id", w.Header().Get("X-Request-ID"), "method", r.Method, "route", r.Pattern, "status", recorded.status, "duration_ms", time.Since(started).Milliseconds())
			}
		}()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		if !a.Demo {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		defer func() {
			if v := recover(); v != nil {
				slog.Error("handler panic", "request_id", w.Header().Get("X-Request-ID"))
				fail(w, 500, "The request could not be completed")
			}
		}()
		m.ServeHTTP(w, r)
	})
}
func (a *App) bootstrap(w http.ResponseWriter, r *http.Request, s session) {
	configs, e := a.configs(r.Context())
	if e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 200, map[string]any{"user": s.User, "csrfToken": s.CSRF, "demo": a.Demo, "configurations": configs, "components": engine.Components, "clarifications": engine.Clarifications})
}
func (a *App) listUsers(w http.ResponseWriter, r *http.Request, s session) {
	rows, e := a.DB.Query(r.Context(), "SELECT id,name,email,role,active,must_change_password FROM users ORDER BY name LIMIT 1000")
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &u.MustChangePassword); e != nil {
			a.dbError(w, e)
			return
		}
		users = append(users, u)
	}
	if e = rows.Err(); e != nil {
		a.dbError(w, e)
		return
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
	if in.Name == "" || len(in.Name) > 200 || !validEmail(in.Email) || (!a.Demo && strings.HasSuffix(in.Email, "@demo.local")) || !validRole(in.Role) || !a.passwordAllowed(in.Password) {
		fail(w, 400, "Name, valid non-demo email, valid role and a unique password (15-72 bytes in production; 12-72 in demo) are required")
		return
	}
	ph, e := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if e != nil {
		a.dbError(w, e)
		return
	}
	u := User{ID: ID(), Name: in.Name, Email: in.Email, Role: in.Role, Active: true, MustChangePassword: !a.Demo}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer tx.Rollback(r.Context())
	_, e = tx.Exec(r.Context(), "INSERT INTO users(id,name,email,password_hash,role,must_change_password) VALUES($1,$2,$3,$4,$5,$6)", u.ID, u.Name, u.Email, string(ph), u.Role, u.MustChangePassword)
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
	if in.Password != nil && !a.passwordAllowed(*in.Password) {
		fail(w, 400, "Use a unique password of 15-72 bytes in production (12-72 in demo)")
		return
	}
	if in.Name != nil && (strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 200) {
		fail(w, 400, "Name cannot be empty")
		return
	}
	if id == s.User.ID && ((in.Active != nil && !*in.Active) || (in.Role != nil && *in.Role != "admin")) {
		fail(w, 400, "You cannot deactivate or remove your own administrator role")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer tx.Rollback(r.Context())
	if _, e = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(42900822)"); e != nil {
		a.dbError(w, e)
		return
	}
	var u User
	e = tx.QueryRow(r.Context(), "SELECT id,name,email,role,active,must_change_password FROM users WHERE id=$1 FOR UPDATE", id).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &u.MustChangePassword)
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
	if before.Role == "admin" && before.Active && (u.Role != "admin" || !u.Active) {
		var admins int
		if e = tx.QueryRow(r.Context(), "SELECT count(*) FROM users WHERE role='admin' AND active").Scan(&admins); e != nil {
			a.dbError(w, e)
			return
		}
		if admins <= 1 {
			fail(w, 409, "At least one active administrator is required")
			return
		}
	}
	_, e = tx.Exec(r.Context(), "UPDATE users SET name=$2,role=$3,active=$4,updated_at=now() WHERE id=$1", id, u.Name, u.Role, u.Active)
	if e == nil && in.Password != nil {
		var ph []byte
		ph, e = bcrypt.GenerateFromPassword([]byte(*in.Password), 12)
		if e == nil {
			u.MustChangePassword = !a.Demo
			_, e = tx.Exec(r.Context(), "UPDATE users SET password_hash=$2,must_change_password=$3 WHERE id=$1", id, string(ph), u.MustChangePassword)
		}
	}
	if e == nil && (in.Role != nil || in.Active != nil || in.Password != nil) {
		_, e = tx.Exec(r.Context(), "UPDATE users SET auth_version=auth_version+1 WHERE id=$1", id)
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
	limit, offset, ok := pagination(w, r)
	if !ok {
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(search) > 200 {
		fail(w, 400, "Search must be at most 200 characters")
		return
	}
	rows, e := a.DB.Query(r.Context(), `SELECT a.id,COALESCE(u.name,'System'),a.action,a.entity_type,a.entity_id,a.details,a.created_at FROM audit_events a LEFT JOIN users u ON u.id=a.actor_id WHERE ($1<>'hr' OR a.actor_id=$2) AND ($5='' OR position(lower($5) in lower(a.action || ' ' || a.entity_type || ' ' || COALESCE(u.name,'System')))>0) ORDER BY a.created_at DESC,a.id LIMIT $3 OFFSET $4`, s.User.Role, s.User.ID, limit, offset, search)
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
	if e = rows.Err(); e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 200, out)
}
