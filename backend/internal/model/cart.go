package model

import "github.com/google/uuid"

// Cart is a user's live basket. One per user, enforced by a unique index on
// user_id in the migration.
//
// A cart is scratch state, not a record: it is deleted with the user, and its
// contents are re-priced on every read. Contrast with Order below.
type Cart struct {
	Base

	// No User association field, so GORM cannot create this foreign key: the
	// migration adds carts.user_id -> users ON DELETE CASCADE by hand.
	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`

	// Items is populated by Preload("Items"). Loading a cart and then querying
	// its items in a loop is the N+1 that CLAUDE.md tells us to flag.
	//
	// CASCADE is correct here: a cart is scratch state, so its lines have no
	// meaning once the cart is gone.
	Items []CartItem `gorm:"foreignKey:CartID;constraint:OnDelete:CASCADE"`
}

func (Cart) TableName() string { return "carts" }

// CartItem is one line in a live basket.
//
// It deliberately stores NO price. Per CLAUDE.md a cart reads the current price
// from products at read time, so a repricing is reflected in the basket
// immediately and a customer cannot hold a stale price by leaving a tab open.
// This is the exact opposite of OrderItem, which snapshots everything.
type CartItem struct {
	Base

	// The composite uniqueIndex is what makes "add the same product twice" an
	// upsert rather than a duplicate row. Both fields must name the same index.
	CartID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_cart_items_cart_product"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_cart_items_cart_product"`

	// CHECK (quantity > 0) is in the migration only.
	Quantity int `gorm:"type:integer;not null"`

	// Product is populated by Preload("Product") — this is where the current
	// price comes from when rendering the cart.
	//
	// RESTRICT rather than CASCADE: products are soft-deleted, so a hard delete
	// should never happen, and if one ever does it must fail loudly instead of
	// emptying baskets without a trace.
	Product *Product `gorm:"foreignKey:ProductID;constraint:OnDelete:RESTRICT"`
}

func (CartItem) TableName() string { return "cart_items" }
