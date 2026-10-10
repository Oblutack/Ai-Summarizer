package initializers

import "testing"

func TestAnUnencryptedConnectionToARemoteDatabaseIsNoticed(t *testing.T) {
	bad := []string{
		"host=db.example.com user=u password=p dbname=d sslmode=disable",
		"host=ep-cool-123.eu-central-1.aws.neon.tech user=u password=p dbname=d sslmode=disable",
		"postgres://u:p@db.example.com:5432/d?sslmode=disable",
		"postgresql://u:p@203.0.113.7/d?sslmode=disable",
		"host=203.0.113.7 user=u sslmode=disable",
	}
	for _, dsn := range bad {
		if host, ok := unencryptedRemoteHost(dsn); !ok || host == "" {
			t.Errorf("should be noticed: %s", dsn)
		}
	}
}

func TestLocalAndEncryptedConnectionsAreLeftAlone(t *testing.T) {
	fine := []string{
		"host=postgres user=u password=p dbname=d port=5432 sslmode=disable", // a Docker service name
		"host=localhost user=u sslmode=disable",
		"host=127.0.0.1 user=u sslmode=disable",
		"host=::1 user=u sslmode=disable",
		"host=10.0.3.4 user=u sslmode=disable",
		"host=192.168.1.20 user=u sslmode=disable",
		"host=/var/run/postgresql user=u sslmode=disable",
		"host=db.example.com user=u sslmode=require",
		"host=db.example.com user=u sslmode=verify-full",
		"host=db.example.com user=u", // not stated: the driver tries encryption first
		"postgres://u:p@db.example.com/d?sslmode=require",
		"postgres://u:p@localhost/d?sslmode=disable",
		"",
		"not a connection string",
	}
	for _, dsn := range fine {
		if host, ok := unencryptedRemoteHost(dsn); ok {
			t.Errorf("should be left alone (%s): %s", host, dsn)
		}
	}
}
