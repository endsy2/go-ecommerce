// Package model holds the GORM structs that map to database tables.
//
// These types carry no `json` tags on purpose: the wire format is the job of
// internal/dto, and handlers map between the two. Coupling the table layout to
// the API response is what makes a column rename a breaking API change.
//
// The struct tags here describe the schema, because cmd/api runs AutoMigrate on
// startup in development (see internal/database/automigrate.go).
//
// They are NOT the whole schema. GORM tags cannot express a CHECK constraint, a
// partial index, an expression index such as UNIQUE(lower(email)), or ON DELETE
// behaviour — and this schema depends on all four. Those live in migrations/,
// which remains the source of truth. Treat the tags as the part of the schema
// GORM happens to be able to reproduce, and keep them in step with the SQL by
// hand: nothing checks that they agree.
//
// Integer widths are pinned with `type:integer` where the migrations chose
// integer, because GORM maps Go's int to bigint and would otherwise rewrite
// those columns on the next startup.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Base holds the three columns every table in this schema has.
//
// Embedding it (writing `Base` with no field name) promotes its fields to the
// outer struct, so product.ID and product.CreatedAt work directly. This is the
// closest Go gets to @MappedSuperclass — but it is composition, not
// inheritance: a Product is not a Base, there is no polymorphism, and nothing
// can be "cast" between them.
type Base struct {
	// Postgres mints the id via gen_random_uuid(), per CLAUDE.md.
	//
	// The `default:` tag is load-bearing, not documentation: it tells GORM to
	// leave a zero-valued ID out of the INSERT so the database default applies,
	// then read the generated value back through RETURNING. Without it GORM
	// would insert the all-zero uuid.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`

	// GORM fills these automatically on create and update, purely because of
	// their names — CreatedAt and UpdatedAt are conventions the library looks
	// for. The DEFAULT now() also means a plain SQL INSERT (a repair script, a
	// seed) cannot leave them null.
	CreatedAt time.Time `gorm:"not null;default:now()"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}
