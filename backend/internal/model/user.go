package model

// Role is what a user is allowed to do. It is a named string type rather than a
// bare string so the compiler rejects a stray literal: a function taking a Role
// cannot be handed "amdin" by accident, because an untyped constant only
// converts where the type matches.
//
// The database CHECK constraint is the real enforcement — the Go type just
// catches the mistake earlier, at compile time instead of on INSERT.
type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

// Valid reports whether r is one of the roles the database will accept. Keep
// this in sync with the CHECK constraint in 000001_create_users.up.sql.
func (r Role) Valid() bool {
	return r == RoleCustomer || r == RoleAdmin
}

// User is an account. Email uniqueness is enforced case-insensitively by a
// unique index on lower(email), so Bob@example.com and bob@example.com cannot
// both exist.
type User struct {
	Base

	// Deliberately NOT tagged uniqueIndex. A tag can only produce a
	// case-SENSITIVE unique index, which would let Bob@example.com register
	// alongside bob@example.com. The migration creates a unique index on
	// lower(email) instead — stricter, and the index the repository's
	// lower(email) = lower(?) lookup actually uses.
	Email string `gorm:"type:text;not null"`

	// The bcrypt/argon2 digest, never the password itself. Named PasswordHash
	// rather than Password so that a log line or a careless dto mapping that
	// leaks it is at least obviously wrong when read.
	PasswordHash string `gorm:"type:text;not null"`

	Name string `gorm:"type:text;not null"`

	// The CHECK constraint restricting this to customer/admin exists only in the
	// migration. AutoMigrate cannot create one, so on an AutoMigrate-only
	// database any string is accepted here.
	Role Role `gorm:"type:text;not null;default:customer"`
}

// TableName pins the table explicitly.
//
// GORM would infer "users" here anyway via its pluralising naming strategy, but
// that inference is a runtime behaviour that a library upgrade could change,
// and it is not obvious for every name. Stating it costs one line and makes the
// struct-to-table mapping greppable.
func (User) TableName() string { return "users" }
