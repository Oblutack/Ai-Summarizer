// Package testutil holds helpers shared by the integration tests. It is only imported from tests.
package testutil

import (
	"ai-summarizer/go-api/initializers"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB returns a freshly migrated database for one test package and installs it as initializers.DB.
// It skips the test unless TEST_DSN is set, e.g.
//
//	TEST_DSN="host=localhost port=5433 user=user password=... dbname=summarizer_test sslmode=disable"
//
// Every package gets its own Postgres schema (search_path), because `go test ./...` runs packages
// in parallel and they would otherwise wipe each other's tables. The schema is dropped and
// recreated, so the database name must contain "test".
func DB(t *testing.T, schema string) *gorm.DB {
	t.Helper()
	db := EmptyDB(t, schema)
	if err := initializers.RunMigrations(); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

// EmptyDB is DB without running the migrations, for tests that need to start from a specific schema.
func EmptyDB(t *testing.T, schema string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN not set; skipping Postgres integration test")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	var name string
	if err := admin.Raw("SELECT current_database()").Scan(&name).Error; err != nil || !strings.Contains(name, "test") {
		t.Fatalf("refusing to wipe database %q: its name must contain \"test\"", name)
	}
	if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec(`CREATE SCHEMA ` + schema).Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := admin.DB(); err == nil {
		sqlDB.Close()
	}

	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), initializers.GormConfig())
	if err != nil {
		t.Fatalf("connect to schema %s: %v", schema, err)
	}
	initializers.DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})

	return db
}
