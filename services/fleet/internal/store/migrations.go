package store

import "embed"

// MigrationsFS embeds internal/store/migrations/*.sql so the sqlite
// implementation can apply them without depending on the working directory
// at runtime. Migrations live here, one level above the sqlite package,
// because the schema they define is the store's contract, not an
// implementation detail of one backend (DEC-002 keeps Postgres a later swap
// behind the same Store interface).
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
