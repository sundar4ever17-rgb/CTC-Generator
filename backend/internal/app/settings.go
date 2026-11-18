package app

import (
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

type SecuritySettings struct {
	SessionIdleMinutes int `json:"sessionIdleMinutes"`
	SessionMaxHours    int `json:"sessionMaxHours"`
}

func (a *App) getSettings(w http.ResponseWriter, r *http.Request, s session) {
	var settings SecuritySettings
	if err := a.DB.QueryRow(r.Context(), "SELECT session_idle_minutes,session_max_hours FROM application_settings WHERE id=1").Scan(&settings.SessionIdleMinutes, &settings.SessionMaxHours); err != nil {
		a.dbError(w, err)
		return
	}
	JSON(w, 200, settings)
}

func (a *App) updateSettings(w http.ResponseWriter, r *http.Request, s session) {
	var in struct {
		SessionIdleMinutes int    `json:"sessionIdleMinutes"`
		SessionMaxHours    int    `json:"sessionMaxHours"`
		CurrentPassword    string `json:"currentPassword"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.SessionIdleMinutes < 5 || in.SessionIdleMinutes > 60 || in.SessionMaxHours < 1 || in.SessionMaxHours > 24 {
		fail(w, 400, "Idle expiry must be 5-60 minutes; absolute expiry must be 1-24 hours")
		return
	}
	if !a.throttle(w, r, "settings:"+s.User.ID, 10, 15*time.Minute) {
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var ph, role string
	var active bool
	err = tx.QueryRow(r.Context(), "SELECT password_hash,role,active FROM users WHERE id=$1 FOR SHARE", s.User.ID).Scan(&ph, &role, &active)
	if err != nil {
		a.dbError(w, err)
		return
	}
	if role != "admin" || !active || bcrypt.CompareHashAndPassword([]byte(ph), []byte(in.CurrentPassword)) != nil {
		fail(w, 400, "Current administrator password is incorrect")
		return
	}
	var before SecuritySettings
	err = tx.QueryRow(r.Context(), "SELECT session_idle_minutes,session_max_hours FROM application_settings WHERE id=1 FOR UPDATE").Scan(&before.SessionIdleMinutes, &before.SessionMaxHours)
	after := SecuritySettings{in.SessionIdleMinutes, in.SessionMaxHours}
	if err == nil {
		_, err = tx.Exec(r.Context(), "UPDATE application_settings SET session_idle_minutes=$1,session_max_hours=$2,updated_at=now(),updated_by=$3 WHERE id=1", in.SessionIdleMinutes, in.SessionMaxHours, s.User.ID)
	}
	if err == nil {
		err = auditTx(r.Context(), tx, s.User.ID, "settings.update", "settings", "security", map[string]any{"before": before, "after": after})
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	JSON(w, 200, after)
}
