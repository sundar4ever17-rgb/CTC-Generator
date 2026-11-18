package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This opt-in test requires a newly created, empty database whose name ends in
// _test. It never drops tables or deletes records from an existing database.
func TestConfiguredIdlePeriodPreservesValidSessionOnConcurrentLogin(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL to a new empty isolated *_test database")
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.HasSuffix(u.Path, "_test") || os.Getenv("CTC_TEST_FIXTURES") != "1" {
		t.Fatal("session integration test requires CTC_TEST_FIXTURES=1 and an isolated *_test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal("cannot open the isolated integration database")
	}
	defer pool.Close()
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("integration database must be newly created and empty; existing data is preserved")
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEMO_PASSWORD", "IdleFixture!2026")
	a := &App{DB: pool, Demo: true, Origin: "http://localhost:3020", pdfSlots: make(chan struct{}, 2)}
	if err = a.initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE application_settings SET session_idle_minutes=60 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	handler := a.Handler()
	login := func() *http.Cookie {
		t.Helper()
		body, err := json.Marshal(map[string]string{"email": "hr@demo.local", "password": "IdleFixture!2026"})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body)).WithContext(ctx)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", a.Origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("fixture login returned %d", response.Code)
		}
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == "ctc_session" {
				return cookie
			}
		}
		t.Fatal("fixture login did not set a session")
		return nil
	}
	status := func(cookie *http.Cookie) int {
		request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil).WithContext(ctx)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}
	first := login()
	if _, err = pool.Exec(ctx, "UPDATE sessions SET last_seen_at=now()-interval '40 minutes' WHERE token_hash=$1", hash(first.Value)); err != nil {
		t.Fatal(err)
	}
	second := login()
	if got := status(first); got != http.StatusOK {
		t.Fatalf("40-minute idle session must survive another login under the configured 60-minute policy; got %d", got)
	}
	if got := status(second); got != http.StatusOK {
		t.Fatalf("new session must remain valid; got %d", got)
	}
	if _, err = pool.Exec(ctx, "UPDATE sessions SET last_seen_at=now()-interval '61 minutes' WHERE token_hash=$1", hash(first.Value)); err != nil {
		t.Fatal(err)
	}
	if got := status(first); got != http.StatusUnauthorized {
		t.Fatalf("61-minute idle session must expire under the configured 60-minute policy; got %d", got)
	}
	if got := status(second); got != http.StatusOK {
		t.Fatalf("expiring another session must not revoke the active session; got %d", got)
	}
}
