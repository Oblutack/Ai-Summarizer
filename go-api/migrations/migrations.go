// Package migrations embeds the SQL schema migrations so the binary carries them with it
// (the production image is built FROM scratch and has no migration files on disk).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
