package initializers

import (
	"ai-summarizer/go-api/migrations"
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// RunMigrations applies any pending schema migrations. It is safe to call from several
// instances at once: the Postgres driver takes an advisory lock while migrating.
//
// By default it uses the same connection as the app. When MIGRATE_DSN is set it uses that one instead, only for the
// migrations and only until they are done, so the app itself can run as an account that cannot change the schema
// (see docs/least-privilege.sql).
func RunMigrations() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	if dsn := os.Getenv("MIGRATE_DSN"); dsn != "" {
		owner, err := gorm.Open(postgres.Open(dsn), GormConfig())
		if err != nil {
			return fmt.Errorf("connect with MIGRATE_DSN: %w", err)
		}
		ownerDB, err := owner.DB()
		if err != nil {
			return fmt.Errorf("get sql db for MIGRATE_DSN: %w", err)
		}
		defer func() { _ = ownerDB.Close() }()
		sqlDB = ownerDB
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	driver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
