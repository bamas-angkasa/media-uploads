package migrations

import "embed"

// FS embeds all SQL migration files into the binary.
// goose reads from this FS so no external files are needed at runtime.
//
//go:embed *.sql
var FS embed.FS
