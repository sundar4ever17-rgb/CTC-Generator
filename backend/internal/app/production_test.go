package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductionPasswordPolicy(t *testing.T) {
	for _, p := range []string{"", "DemoCTC!2026", "password123456789", "aaaaaaaaaaaaaaaa", "has surrounding spaces ", strings.Repeat("x", 73)} {
		if productionPassword(p) {
			t.Fatalf("weak/invalid password accepted")
		}
	}
	if !productionPassword("LongUnique!Phrase2026") {
		t.Fatal("strong passphrase rejected")
	}
}
func TestStrictRequestShape(t *testing.T) {
	for _, tc := range []struct {
		body, kind string
		status     int
	}{{`{"name":"ok","unexpected":true}`, "application/json", 400}, {`{"name":"ok"}`, "application/jsonp", 415}, {strings.Repeat(" ", 1024*1024) + `{}`, "application/json", 400}} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.kind)
		rr := httptest.NewRecorder()
		var value struct {
			Name string `json:"name"`
		}
		if readJSON(rr, req, &value) || rr.Code != tc.status {
			t.Fatalf("unexpected validation status %d", rr.Code)
		}
	}
}
func TestProductionEnvironmentFailsClosedBeforeDatabase(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://test:testing@localhost/test?sslmode=disable")
	t.Setenv("DATABASE_URL_FILE", "")
	if _, err := OpenDatabase(context.Background()); err == nil || !strings.Contains(err.Error(), "verify-full") {
		t.Fatalf("insecure database accepted: %v", err)
	}
	t.Setenv("APP_ORIGIN", "http://example.test")
	if _, err := New(context.Background()); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("insecure origin accepted: %v", err)
	}
	t.Setenv("APP_ORIGIN", "https://example.test")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "secret-not-for-server")
	if _, err := New(context.Background()); err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("bootstrap secret leaked into runtime: %v", err)
	}
}
func TestPaginationBounds(t *testing.T) {
	for _, query := range []string{"?limit=0", "?limit=501", "?offset=-1", "?limit=x"} {
		rr := httptest.NewRecorder()
		_, _, ok := pagination(rr, httptest.NewRequest("GET", "/"+query, nil))
		if ok || rr.Code != 400 {
			t.Fatal("invalid pagination accepted")
		}
	}
}
