// Package migrations embeds the application schema.
package migrations

import "embed"

// FS contains ordered SQL migrations. Each file is applied once.
//
//go:embed *.sql
var FS embed.FS
