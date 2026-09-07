package model

import "github.com/google/uuid"

// OrderStatus is the fixed lifecycle from CLAUDE.md. Adding a value here also
// means a migration to widen the CHECK constraint, and explicit approval.
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// Valid reports whether s is a status the database will accept. Keep in sync
// with the CHECK constraint in 000005_create_orders.up.sql.
func (s OrderStatus) Valid() bool {
	switch s {
	case OrderStatusPending, OrderStatusPaid, OrderStatusShipped,
		OrderStatusDelivered, OrderStatusCancelled:
		return true
	default:
		return false
	}
}

// Order is a historical record of a purchase.
//
// Unlike Cart, an order never changes when the catalogue does. Its user FK is
// ON DELETE RESTRICT in the migration: deleting a customer must not silently
// erase their purchase history.
type Order struct {
	Base

	// No User association field, so the migration adds this foreign key by hand
	// as orders.user_id -> users ON DELETE RESTRICT — the rule that stops
	// deleting a customer from erasing their purchase history.
	UserID uuid.UUID `gorm:"type:uuid;not null;index"`

	// The CHECK restricting this to the five valid statuses is in the migration
	// only. On an AutoMigrate-only database this column accepts any string.
	Status OrderStatus `gorm:"type:text;not null"`

	// TotalCents is stored rather than summed from the items on read. The sum
	// is a fact about what the customer was actually charged at the time, and
	// it must survive any later correction to a line item.
	TotalCents int64 `gorm:"not null"`

	Items []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
}

func (Order) TableName() string { return "orders" }

// OrderItem is one line of a placed order.
//
// ProductName and UnitPriceCents are SNAPSHOTS, copied at checkout. CLAUDE.md
// is explicit: never join to products to display a past order. If a product is
// renamed from "Blue Mug" to "Navy Mug" or repriced from 999 to 1299, every
// historical invoice must still show what the customer actually bought and
// paid. ProductID is kept only for analytics and reordering, never for display.
type OrderItem struct {
	Base

	OrderID uuid.UUID `gorm:"type:uuid;not null;index"`

	// There is deliberately NO Product association field here, even though a
	// foreign key exists. An association would make Preload("Product") possible,
	// and that is exactly the mistake CLAUDE.md forbids: rendering a past order
	// from the live catalogue. Use the snapshot columns below instead. The
	// migration adds the foreign key by hand precisely because there is no
	// association for GORM to derive it from.
	ProductID uuid.UUID `gorm:"type:uuid;not null"`

	ProductName    string `gorm:"type:text;not null"`
	UnitPriceCents int64  `gorm:"not null"`

	Quantity int `gorm:"type:integer;not null"`
}

func (OrderItem) TableName() string { return "order_items" }
