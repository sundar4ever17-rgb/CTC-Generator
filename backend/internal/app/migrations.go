package app

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"ctc/internal/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct{ version, sql string }

func migrations() []migration {
	items := []migration{{"001_baseline", schema}}
	for _, name := range []string{"002_production", "003_settings", "004_configurable_rules"} {
		b, err := migrationFiles.ReadFile("migrations/" + name + ".sql")
		if err != nil {
			panic(err)
		}
		items = append(items, migration{name, string(b)})
	}
	return items
}

// Migrate is explicit in production. The baseline adopts the demo schema without
// deleting records. All changes and checksums commit together under a DB lock.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(42900821)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	registered := migrations()
	known := make(map[string]bool, len(registered))
	for _, m := range registered {
		known[m.version] = true
	}
	rows, err := tx.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return err
	}
	for rows.Next() {
		var version string
		if err = rows.Scan(&version); err != nil {
			rows.Close()
			return err
		}
		if !known[version] {
			rows.Close()
			return fmt.Errorf("database contains unknown migration %s; use a compatible application release", version)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, m := range registered {
		checksum := hash(strings.ReplaceAll(m.sql, "\r\n", "\n"))
		var saved string
		err = tx.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version=$1", m.version).Scan(&saved)
		if err == nil {
			if saved != checksum {
				return fmt.Errorf("migration %s checksum mismatch", m.version)
			}
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.version, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)", m.version, checksum); err != nil {
			return err
		}
	}
	for _, c := range engine.Components {
		if _, err = tx.Exec(ctx, "INSERT INTO salary_components(code,name) VALUES($1,$2) ON CONFLICT DO NOTHING", c.Code, c.Name); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func verifyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	for _, m := range migrations() {
		var saved string
		if err := pool.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version=$1", m.version).Scan(&saved); err != nil {
			return fmt.Errorf("database schema is not current; run migrate with the migration role")
		}
		if saved != hash(strings.ReplaceAll(m.sql, "\r\n", "\n")) {
			return fmt.Errorf("migration %s checksum mismatch", m.version)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		return err
	}
	if count != len(migrations()) {
		return fmt.Errorf("database schema is newer than this application; use a compatible release")
	}
	return nil
}

// BootstrapAdmin is a one-time administrative command, never run by the server.
func BootstrapAdmin(ctx context.Context, pool *pgxpool.Pool) error {
	if err := verifyMigrations(ctx, pool); err != nil {
		return err
	}
	email, err := secretEnv("BOOTSTRAP_ADMIN_EMAIL")
	if err != nil {
		return err
	}
	password, err := secretEnv("BOOTSTRAP_ADMIN_PASSWORD")
	if err != nil {
		return err
	}
	name, err := secretEnv("BOOTSTRAP_ADMIN_NAME")
	if err != nil {
		return err
	}
	if name == "" {
		name = "Administrator"
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || strings.HasSuffix(email, "@demo.local") || !productionPassword(password) || len(name) > 200 {
		return fmt.Errorf("bootstrap requires a non-demo email, name and a unique password of 15-72 bytes")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(42900822)"); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE role='admin' AND active=true").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("an active administrator already exists; use user management")
	}
	ph, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	id := ID()
	if _, err = tx.Exec(ctx, "INSERT INTO users(id,name,email,password_hash,role,must_change_password) VALUES($1,$2,$3,$4,'admin',true)", id, name, email, string(ph)); err != nil {
		return err
	}
	if err = auditTx(ctx, tx, id, "user.bootstrap", "user", id, map[string]string{"role": "admin"}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
