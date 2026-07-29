package sqlite

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func downOne(t *testing.T, db *sql.DB) error {
	t.Helper()
	return runGoose(t, func() error {
		return goose.Down(db, "migrations")
	})
}

func TestMigration0041PreservesV1AndRefusesV2Downgrade(t *testing.T) {
	db, err := sql.Open(
		"sqlite",
		"file:"+filepath.Join(t.TempDir(), "ao.db")+pragmas,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	upTo(t, db, 40)

	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	if _, err := db.Exec(
		`INSERT INTO projects (id, path, registered_at)
		 VALUES ('goalchainer', '/repos/goalchainer', ?)`,
		now,
	); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	if _, err := db.Exec(
		`INSERT INTO goalchainer_projections
		 (project_id, protocol, source_revision, projection, updated_at)
		 VALUES ('goalchainer', 'goalchainer-ui-v1', ?, '{"protocol":"goalchainer-ui-v1"}', ?)`,
		digest,
		now,
	); err != nil {
		t.Fatal(err)
	}

	upTo(t, db, 41)
	var protocol, projection string
	if err := db.QueryRow(
		`SELECT protocol, projection FROM goalchainer_projections
		 WHERE project_id = 'goalchainer'`,
	).Scan(&protocol, &projection); err != nil {
		t.Fatal(err)
	}
	if protocol != "goalchainer-ui-v1" ||
		projection != `{"protocol":"goalchainer-ui-v1"}` {
		t.Fatalf("v1 row after upgrade = %q, %q", protocol, projection)
	}
	if err := downOne(t, db); err != nil {
		t.Fatalf("v1 downgrade: %v", err)
	}

	upTo(t, db, 41)
	if _, err := db.Exec(
		`UPDATE goalchainer_projections
		 SET protocol = 'goalchainer-ui-v2',
		     projection = '{"protocol":"goalchainer-ui-v2","projectId":"goalchainer"}'`,
	); err != nil {
		t.Fatal(err)
	}
	if err := downOne(t, db); err == nil {
		t.Fatal("v2 downgrade succeeded, want lossless refusal")
	}
	if err := db.QueryRow(
		`SELECT protocol, projection FROM goalchainer_projections
		 WHERE project_id = 'goalchainer'`,
	).Scan(&protocol, &projection); err != nil {
		t.Fatalf("v2 row missing after refused downgrade: %v", err)
	}
	if protocol != "goalchainer-ui-v2" ||
		!strings.Contains(projection, `"projectId":"goalchainer"`) {
		t.Fatalf("v2 row after refused downgrade = %q, %q", protocol, projection)
	}
}
