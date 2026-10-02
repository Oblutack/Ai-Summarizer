package initializers

import (
	"log"
	"log/slog"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// GormConfig returns the GORM settings shared by the app and the tests. By default GORM prints
// every "record not found" (a normal outcome for First) in colour to stdout, which would litter
// the structured logs; only real problems and slow queries are reported.
func GormConfig() *gorm.Config {
	return &gorm.Config{
		Logger: gormlogger.New(log.New(os.Stdout, "", log.LstdFlags), gormlogger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
	}
}

func ConnectToDB() {
	var err error
	DB, err = gorm.Open(postgres.Open(os.Getenv("DSN")), GormConfig())

	if err != nil {
		slog.Error("failed to connect to the database", "error", err)
		os.Exit(1)
	}
}
