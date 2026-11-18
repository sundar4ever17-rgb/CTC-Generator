package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func secretEnv(name string) (string, error) {
	value, path := os.Getenv(name), os.Getenv(name+"_FILE")
	if value != "" && path != "" {
		return "", fmt.Errorf("set only %s or %s_FILE", name, name)
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("cannot read %s_FILE", name)
		}
		value = strings.TrimRight(string(b), "\r\n")
	}
	return value, nil
}

func OpenDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	env := os.Getenv("APP_ENV")
	if env != "demo" && env != "production" {
		return nil, fmt.Errorf("APP_ENV must explicitly be demo or production")
	}
	raw, err := secretEnv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, fmt.Errorf("DATABASE_URL or DATABASE_URL_FILE is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		return nil, fmt.Errorf("DATABASE_URL must be a PostgreSQL URL")
	}
	if env == "production" && parsed.Query().Get("sslmode") != "verify-full" {
		return nil, fmt.Errorf("production database requires sslmode=verify-full and a trusted CA")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid database connection configuration")
	}
	max := 20
	if v := os.Getenv("DB_POOL_MAX_CONNS"); v != "" {
		max, err = strconv.Atoi(v)
		if err != nil || max < 2 || max > 200 {
			return nil, fmt.Errorf("DB_POOL_MAX_CONNS must be 2-200")
		}
	}
	cfg.MaxConns = int32(max)
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 10 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = "ctc-generator"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "30000"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "30000"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize database connection")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database connection failed; verify availability, credentials and TLS")
	}
	return pool, nil
}

func New(ctx context.Context) (*App, error) {
	demo := os.Getenv("APP_ENV") == "demo"
	origin := strings.TrimSuffix(os.Getenv("APP_ORIGIN"), "/")
	if origin == "" && demo {
		origin = "http://localhost:3020"
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(demo && u.Scheme == "http")) {
		return nil, fmt.Errorf("APP_ORIGIN must be an exact HTTPS origin in production")
	}
	if !demo && (os.Getenv("DEMO_PASSWORD") != "" || os.Getenv("BOOTSTRAP_ADMIN_PASSWORD") != "" || os.Getenv("BOOTSTRAP_ADMIN_PASSWORD_FILE") != "") {
		return nil, fmt.Errorf("server must not receive demo or bootstrap credentials")
	}
	pool, err := OpenDatabase(ctx)
	if err != nil {
		return nil, err
	}
	a := &App{DB: pool, Origin: origin, Demo: demo, pdfSlots: make(chan struct{}, 2)}
	if demo {
		err = Migrate(ctx, pool)
		if err == nil {
			err = a.initialize(ctx)
		}
	} else {
		err = verifyMigrations(ctx, pool)
		if err == nil {
			err = a.verifyProduction(ctx)
		}
	}
	if err != nil {
		pool.Close()
		return nil, err
	}
	return a, nil
}

func (a *App) verifyProduction(ctx context.Context) error {
	var unsafeRole bool
	err := a.DB.QueryRow(ctx, `SELECT r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR has_schema_privilege(current_user,'public','CREATE') OR EXISTS(SELECT 1 FROM pg_class c WHERE c.relname='users' AND c.relnamespace='public'::regnamespace AND pg_has_role(current_user,c.relowner,'MEMBER')) FROM pg_roles r WHERE r.rolname=current_user`).Scan(&unsafeRole)
	if err != nil {
		return err
	}
	if unsafeRole {
		return fmt.Errorf("production runtime requires a non-owner, non-administrative database role without schema CREATE")
	}
	var count int
	if err = a.DB.QueryRow(ctx, "SELECT count(*) FROM users WHERE active AND lower(email) LIKE '%@demo.local'").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("production cannot start with active demo accounts; migrate approved users separately")
	}
	if err = a.DB.QueryRow(ctx, "SELECT count(*) FROM users WHERE active AND role='admin'").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no active administrator; run bootstrap-admin using the migration role")
	}
	return nil
}
