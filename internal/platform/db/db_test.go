package db_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leha-creator/maxim/internal/platform/db"
	"github.com/leha-creator/maxim/internal/platform/dbtest"
	"github.com/leha-creator/maxim/internal/platform/ids"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/tx"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

func TestMigrationsDownAndUp(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	log := logx.Discard()
	for range 3 { // откат 00003, 00002 и 00001
		if err := db.MigrateURL(ctx, d.OwnerURL, dbtest.AppRole, db.MigrateDown, log); err != nil {
			t.Fatalf("down: %v", err)
		}
	}
	var n int
	if err := d.Owner.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename <> 'goose_db_version'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d tables left after down", n)
	}
	if err := db.MigrateURL(ctx, d.OwnerURL, dbtest.AppRole, db.MigrateUp, log); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if err := db.MigrateURL(ctx, d.OwnerURL, dbtest.AppRole, db.MigrateStatus, log); err != nil {
		t.Fatalf("status: %v", err)
	}
}

func TestImmutableTablesForAppRole(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	now := time.Now().UTC()
	auditID := ids.New()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO audit_log (id, actor_type, action, object_type, object_id, data, created_at)
		VALUES ($1, 'system', 'test.insert', 'test', 'x', '{}', $2)`, auditID, now); err != nil {
		t.Fatalf("app role cannot insert into audit_log: %v", err)
	}
	for _, stmt := range []string{
		`UPDATE audit_log SET action = 'changed'`,
		`DELETE FROM audit_log`,
		`UPDATE legal_acceptance SET ip = NULL`,
		`DELETE FROM legal_acceptance`,
	} {
		_, err := d.Pool.Exec(ctx, stmt)
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s: want permission denied, got %v", stmt, err)
		}
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE "user" SET status = 'active' WHERE false`); err != nil {
		t.Fatalf("app role cannot update user: %v", err)
	}
}

func TestInTxRollsBack(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	runner := tx.NewRunner(d.Pool, logx.Discard())
	insert := func(ctx context.Context) error {
		_, err := db.Conn(ctx, d.Pool).Exec(ctx, `INSERT INTO job (id, kind, payload, run_at, status, max_attempts, created_at, updated_at)
			VALUES ($1, 'test', '{}', now(), 'pending', 1, now(), now())`, ids.New())
		return err
	}
	boom := errors.New("boom")
	err := runner.InTx(ctx, func(ctx context.Context) error {
		if err := insert(ctx); err != nil {
			return err
		}
		// Вложенный InTx работает в той же транзакции и откатывается вместе с ней.
		if err := runner.InTx(ctx, insert); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx error = %v", err)
	}
	var n int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM job`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d rows survived rollback", n)
	}

	if err := runner.InTx(ctx, insert); err != nil {
		t.Fatal(err)
	}
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM job`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("commit: n=%d err=%v", n, err)
	}
}

func TestUniqueViolation(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	q := `INSERT INTO "user" (id, email, status, created_at, updated_at) VALUES ($1, 'A@example.ru', 'active', now(), now())`
	if _, err := d.Pool.Exec(ctx, q, ids.New()); err != nil {
		t.Fatal(err)
	}
	_, err := d.Pool.Exec(ctx, strings.Replace(q, "A@example.ru", "a@EXAMPLE.ru", 1), ids.New())
	if !db.UniqueViolation(err, "user_email_key") {
		t.Fatalf("email must be unique case-insensitively, got %v", err)
	}
}

func TestIntegrationTablesForAppRole(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO outbound_request (id, system, operation, attempt, request, created_at) VALUES ('` + ids.New().String() + `', 'max', 'GetMe', 1, '{}', now())`,
		`INSERT INTO rate_limit_bucket (key, tokens, refilled_at) VALUES ('max:global', 25, now())`,
		`UPDATE rate_limit_bucket SET tokens = 1`,
		`INSERT INTO outbound_circuit (system, updated_at) VALUES ('max', now())`,
		`INSERT INTO alert (id, reason, level, summary, first_raised_at, last_raised_at, raised_count) VALUES ('` + ids.New().String() + `', 'test', 'error', 's', now(), now(), 1)`,
		`INSERT INTO worker_heartbeat (instance, started_at, seen_at) VALUES ('w1', now(), now())`,
		`DELETE FROM outbound_request`,
	} {
		if _, err := d.Pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	insert := `INSERT INTO inbound_event (id, source, external_key, event_type, payload, status, created_at, updated_at)
		VALUES ($1, 'max', 'k1', 'bot_added', '{}', 'received', now(), now()) ON CONFLICT (source, external_key) DO NOTHING`
	for range 2 {
		if _, err := d.Pool.Exec(ctx, insert, ids.New()); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM inbound_event`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("inbound_event rows = %d, err = %v; want 1", n, err)
	}
}
