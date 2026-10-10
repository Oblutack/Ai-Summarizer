package initializers

import (
	"log"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
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

// unencryptedRemoteHost returns the host when a connection string asks for no encryption (sslmode=disable) to a
// database that is not on this machine or on a private network name (a Docker service like "postgres" has no dots):
// everything the app sends, passwords and documents included, would cross the network in the clear.
func unencryptedRemoteHost(dsn string) (string, bool) {
	var host, sslmode string
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if u, err := url.Parse(dsn); err == nil {
			host, sslmode = u.Hostname(), u.Query().Get("sslmode")
		}
	} else {
		for _, field := range strings.Fields(dsn) {
			if value, ok := strings.CutPrefix(field, "host="); ok {
				host = value
			}
			if value, ok := strings.CutPrefix(field, "sslmode="); ok {
				sslmode = value
			}
		}
	}
	if sslmode != "disable" || host == "" || strings.HasPrefix(host, "/") || host == "localhost" || !strings.Contains(host, ".") {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		return "", false
	}
	return host, true
}

func ConnectToDB() {
	var err error
	if host, bad := unencryptedRemoteHost(os.Getenv("DSN")); bad {
		slog.Warn("the database connection is not encrypted (sslmode=disable) and the database is not on this machine: use sslmode=require", "host", host)
	}
	DB, err = gorm.Open(postgres.Open(os.Getenv("DSN")), GormConfig())

	if err != nil {
		slog.Error("failed to connect to the database", "error", err)
		os.Exit(1)
	}
}
