package initializers

import (
	"log/slog"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectToDB() {
	var err error
	dsn := os.Getenv("DSN") 
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		slog.Error("failed to connect to the database", "error", err)
		os.Exit(1)
	}
}