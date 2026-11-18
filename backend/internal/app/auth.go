package app

import (
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func validEmail(s string) bool {
	if len(s) > 254 || strings.ContainsAny(s, "\r\n") {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && strings.Contains(s, "@")
}
func productionPassword(p string) bool {
	return utf8.RuneCountInString(p) >= 15 && len(p) <= 72 && strings.TrimSpace(p) == p && !strings.Contains(strings.ToLower(p), "democtc") && !strings.Contains(strings.ToLower(p), "password") && len(strings.Trim(p, string([]rune(p)[0]))) > 0
}
func (a *App) passwordAllowed(p string) bool {
	if a.Demo {
		return validPassword(p)
	}
	return productionPassword(p)
}

// Fixed windows are atomic in PostgreSQL and shared by every API replica. Keys
// are hashed; proxy headers are deliberately not trusted. The IP limit is a
// coarse safeguard; per-account limits protect users behind shared proxies.
func (a *App) throttle(w http.ResponseWriter, r *http.Request, key string, limit int, window time.Duration) bool {
	var count int
	err := a.DB.QueryRow(r.Context(), `INSERT INTO auth_rate_limits(key_hash,attempts,window_end) VALUES($1,1,now()+$2::interval) ON CONFLICT(key_hash) DO UPDATE SET attempts=CASE WHEN auth_rate_limits.window_end<=now() THEN 1 ELSE auth_rate_limits.attempts+1 END,window_end=CASE WHEN auth_rate_limits.window_end<=now() THEN EXCLUDED.window_end ELSE auth_rate_limits.window_end END RETURNING attempts`, hash(key), window.String()).Scan(&count)
	if err != nil {
		a.dbError(w, err)
		return false
	}
	if count > limit {
		w.Header().Set("Retry-After", "900")
		fail(w, 429, "Too many authentication attempts; retry after 15 minutes")
		return false
	}
	return true
}

var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("constant timing-only credential"), 12)

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
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmail(in.Email) || len(in.Password) > 72 || len(in.Password) == 0 {
		fail(w, 401, "Invalid email or password")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	if !a.throttle(w, r, "login-ip:"+ip, 600, 15*time.Minute) || !a.throttle(w, r, "login-email:"+in.Email, 15, 15*time.Minute) {
		return
	}
	var u User
	var ph string
	var version int64
	err := a.DB.QueryRow(r.Context(), "SELECT id,name,email,role,active,must_change_password,password_hash,auth_version FROM users WHERE email=$1", in.Email).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &u.MustChangePassword, &ph, &version)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		a.dbError(w, err)
		return
	}
	if ph == "" {
		ph = string(dummyPasswordHash)
	}
	passwordErr := bcrypt.CompareHashAndPassword([]byte(ph), []byte(in.Password))
	if err != nil || !u.Active || passwordErr != nil || (!a.Demo && strings.HasSuffix(in.Email, "@demo.local")) {
		if err = a.audit(r.Context(), "", "auth.login_failed", "authentication", hash(in.Email), map[string]string{"result": "denied"}); err != nil {
			a.dbError(w, err)
			return
		}
		fail(w, 401, "Invalid email or password")
		return
	}
	raw, csrf := token(), token()
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	// Lock and re-check to prevent an in-flight login resurrecting a revoked session.
	var currentVersion int64
	var active bool
	err = tx.QueryRow(r.Context(), "SELECT auth_version,active FROM users WHERE id=$1 FOR UPDATE", u.ID).Scan(&currentVersion, &active)
	if err != nil {
		a.dbError(w, err)
		return
	}
	if currentVersion != version || !active {
		fail(w, 401, "Credentials changed; sign in again")
		return
	}
	var maxHours, idleMinutes int
	if err = tx.QueryRow(r.Context(), "SELECT session_max_hours,session_idle_minutes FROM application_settings WHERE id=1").Scan(&maxHours, &idleMinutes); err != nil {
		a.dbError(w, err)
		return
	}
	if c, e := r.Cookie("ctc_session"); e == nil {
		_, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hash(c.Value))
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id=$1 AND (expires_at<=now() OR last_seen_at<=now()-make_interval(mins=>$2))", u.ID, idleMinutes)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,auth_version) VALUES($1,$2,$3,$4,$5)", hash(raw), u.ID, csrf, time.Now().Add(time.Duration(maxHours)*time.Hour), version)
	}
	if err == nil {
		err = auditTx(r.Context(), tx, u.ID, "auth.login", "user", u.ID, map[string]string{"result": "success"})
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "ctc_session", Value: raw, Path: "/", HttpOnly: true, Secure: !a.Demo, SameSite: http.SameSiteStrictMode, MaxAge: maxHours * 3600})
	JSON(w, 200, map[string]any{"user": u, "csrfToken": csrf})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request, s session) {
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if c, e := r.Cookie("ctc_session"); e == nil {
		_, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hash(c.Value))
	}
	if err == nil {
		err = auditTx(r.Context(), tx, s.User.ID, "auth.logout", "user", s.User.ID, map[string]bool{"ok": true})
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	a.clearCookie(w)
	JSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "ctc_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: !a.Demo, SameSite: http.SameSiteStrictMode})
}

func (a *App) changePassword(w http.ResponseWriter, r *http.Request, s session) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if !a.passwordAllowed(in.NewPassword) || in.NewPassword == in.CurrentPassword {
		fail(w, 400, "Use a new unique password: 15-72 bytes in production (12-72 in demo)")
		return
	}
	if !a.throttle(w, r, "password:"+s.User.ID, 10, 15*time.Minute) {
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var ph string
	err = tx.QueryRow(r.Context(), "SELECT password_hash FROM users WHERE id=$1 FOR UPDATE", s.User.ID).Scan(&ph)
	if err != nil {
		a.dbError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(ph), []byte(in.CurrentPassword)) != nil {
		fail(w, 400, "Current password is incorrect")
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), 12)
	if err == nil {
		_, err = tx.Exec(r.Context(), "UPDATE users SET password_hash=$2,must_change_password=false,auth_version=auth_version+1,updated_at=now() WHERE id=$1", s.User.ID, string(newHash))
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), "DELETE FROM sessions WHERE user_id=$1", s.User.ID)
	}
	if err == nil {
		err = auditTx(r.Context(), tx, s.User.ID, "auth.password_changed", "user", s.User.ID, map[string]bool{"sessionsRevoked": true})
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	a.clearCookie(w)
	JSON(w, 200, map[string]bool{"ok": true})
}

type responseStatus struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *responseStatus) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *responseStatus) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
