package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Product is an item for sale.
//
// Products are soft-deleted, never removed: order_items reference them, and a
// past order must stay readable after a product is withdrawn from sale.
type Product struct {
	Base

	CategoryID uuid.UUID `gorm:"type:uuid;not null;index"`

	// Category is populated only by an explicit Preload("Category"). It is a
	// pointer so an unloaded association is nil rather than a zero-valued
	// Category that looks like a real row with an empty name.
	//
	// constraint:OnDelete:RESTRICT is what creates the foreign key, and it is
	// load-bearing: deleting a category that still has products must fail
	// rather than silently take the catalogue with it.
	Category *Category `gorm:"foreignKey:CategoryID;constraint:OnDelete:RESTRICT"`

	Name string `gorm:"type:text;not null"`

	// Slug is the URL-safe identifier the storefront reads by
	// (/api/v1/products/blue-ceramic-mug). It is derived from Name when the
	// product is created and then FROZEN: renaming a product must not change
	// its slug, or every existing link and bookmark to it breaks. That is also
	// why admin writes address a product by :id instead.
	//
	// The tag produces a plain unique index. The real one, created by migration
	// 000006, is PARTIAL — unique only WHERE deleted_at IS NULL — so a
	// withdrawn product does not hold its slug hostage forever. Both carry the
	// name idx_products_slug, so AutoMigrate sees it already exists and leaves
	// the migration's version alone.
	Slug string `gorm:"type:text;not null;uniqueIndex:idx_products_slug"`

	Description string `gorm:"type:text;not null;default:''"`

	// Money is int64 cents everywhere, per CLAUDE.md: 1999 is £19.99. Never
	// float64 — 0.1 + 0.2 != 0.3 in binary floating point, and a rounding error
	// in a price is a real financial bug. Formatting happens at the API edge.
	PriceCents int64 `gorm:"not null"`

	// Stock is the source of truth for availability. CLAUDE.md forbids the
	// read-then-write pattern: services must change it with a conditional
	// UPDATE (`SET stock = stock - ? WHERE id = ? AND stock >= ?`) or a
	// SELECT ... FOR UPDATE inside a transaction, or two concurrent checkouts
	// will both pass the check and oversell.
	//
	// The CHECK (stock >= 0) backstop is in the migration only. type:integer
	// pins the width: Go's int would otherwise become bigint, and AutoMigrate
	// would rewrite the column on the next startup.
	Stock int `gorm:"type:integer;not null;default:0"`

	// DeletedAt turns on GORM's soft delete. This one field changes every query
	// on this model: GORM silently appends `WHERE deleted_at IS NULL` to reads
	// and turns Delete() into an UPDATE that stamps the column.
	//
	// Worth knowing loudly, because it is invisible at the call site — a
	// repository that needs withdrawn products back must ask for them with
	// Unscoped().
	DeletedAt gorm.DeletedAt
}

func (Product) TableName() string { return "products" }
