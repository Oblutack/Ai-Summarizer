package initializers_test

import (
	"os"
	"strings"
	"testing"

	"ai-summarizer/go-api/models"
	"ai-summarizer/go-api/testutil"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// docs/least-privilege.sql is run against a real database, and the account it makes is tried out: it must be able to do
// everything the app does (read and write rows, use the sequences, reach tables added later) and nothing more.

const testRole = "inkling_app_test"
const testSchema = "privtest"

// dsnFor is TEST_DSN with another account in it, working in the test schema.
func dsnFor(t *testing.T, user, password string) string {
	t.Helper()
	var kept []string
	for _, field := range strings.Fields(os.Getenv("TEST_DSN")) {
		if !strings.HasPrefix(field, "user=") && !strings.HasPrefix(field, "password=") {
			kept = append(kept, field)
		}
	}
	return strings.Join(append(kept, "user="+user, "password="+password, "search_path="+testSchema), " ")
}

func TestTheLimitedAccountCanDoWhatTheAppDoesAndNothingMore(t *testing.T) {
	testutil.DB(t, testSchema) // a migrated schema, so there are tables to protect (including schema_migrations)
	script, err := os.ReadFile("../../docs/least-privilege.sql")
	if err != nil {
		t.Fatal(err)
	}
	// the same script, for a test role and the test schema; the simple protocol lets one Exec run all its statements
	sql := strings.NewReplacer("inkling_app", testRole, "CHANGE_ME", "test-password", "SCHEMA public", "SCHEMA "+testSchema).Replace(string(script))
	sql = strings.Replace(sql, "to_regclass('schema_migrations')", "to_regclass('"+testSchema+".schema_migrations')", 1)

	owner, err := gorm.Open(postgres.Open(dsnFor(t, ownerOf(t), passwordOf(t))+" default_query_exec_mode=simple_protocol"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		owner.Exec("DROP OWNED BY " + testRole)
		owner.Exec("DROP ROLE IF EXISTS " + testRole)
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		if sqlDB, err := owner.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := owner.Exec(sql).Error; err != nil {
		t.Fatalf("the script: %v", err)
	}

	app, err := gorm.Open(postgres.Open(dsnFor(t, testRole, "test-password")), &gorm.Config{})
	if err != nil {
		t.Fatalf("the limited account cannot connect: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := app.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	// what the app does all day
	user := models.User{Email: "limited@example.com", Password: "x", HasPassword: true}
	if err := app.Create(&user).Error; err != nil || user.ID == 0 {
		t.Fatalf("insert (and use a sequence): %v", err)
	}
	if err := app.Model(&user).Update("email", "changed@example.com").Error; err != nil {
		t.Errorf("update: %v", err)
	}
	var found models.User
	if err := app.First(&found, user.ID).Error; err != nil || found.Email != "changed@example.com" {
		t.Errorf("select: %v", err)
	}
	if err := app.Exec("SELECT id FROM users WHERE id = ? FOR UPDATE", user.ID).Error; err != nil {
		t.Errorf("select for update (used to count the keys): %v", err)
	}
	if err := app.Unscoped().Delete(&user).Error; err != nil {
		t.Errorf("delete: %v", err)
	}

	// a table a later migration adds is reachable too, without running the script again
	if err := owner.Exec("CREATE TABLE " + testSchema + ".later_table (id serial PRIMARY KEY, note text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Exec("INSERT INTO later_table (note) VALUES ('hello')").Error; err != nil {
		t.Errorf("a table added later: %v", err)
	}

	// what it must not be able to do
	forbidden := map[string]string{
		"create a table":          "CREATE TABLE evil (id int)",
		"drop a table":            "DROP TABLE users",
		"change a table":          "ALTER TABLE users ADD COLUMN evil text",
		"empty a table":           "TRUNCATE users",
		"drop an index":           "DROP INDEX IF EXISTS idx_documents_share_token",
		"read the migration list": "SELECT * FROM schema_migrations",
		"edit the migration list": "DELETE FROM schema_migrations",
		"make a role":             "CREATE ROLE evil LOGIN",
		"make a database":         "CREATE DATABASE evil",
		"make a schema":           "CREATE SCHEMA evil",
	}
	for what, statement := range forbidden {
		if err := app.Exec(statement).Error; err == nil {
			t.Errorf("the limited account could %s", what)
		}
	}

	// granting itself more only earns a warning from Postgres (it holds no right to give), and changes nothing
	_ = app.Exec("GRANT ALL ON TABLE users TO " + testRole).Error
	if err := app.Exec("TRUNCATE users").Error; err == nil {
		t.Error("the limited account gave itself the right to empty a table")
	}
}

// ownerOf and passwordOf read the account of TEST_DSN.
func ownerOf(t *testing.T) string { return field(t, "user") }

func passwordOf(t *testing.T) string { return field(t, "password") }

func field(t *testing.T, name string) string {
	t.Helper()
	for _, f := range strings.Fields(os.Getenv("TEST_DSN")) {
		if value, ok := strings.CutPrefix(f, name+"="); ok {
			return value
		}
	}
	t.Fatalf("TEST_DSN has no %s", name)
	return ""
}
