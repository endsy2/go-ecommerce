package dto

import "time"

// ProductResponse is the wire shape of a product.
//
// PriceCents is int64 and named for its unit, per CLAUDE.md: the API never
// sends a formatted price or a float. 1999 means £19.99, and the client decides
// how to render it.
type ProductResponse struct {
	ID          string    `json:"id"`
	CategoryID  string    `json:"category_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	PriceCents  int64     `json:"price_cents"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Category is present only on endpoints that preload it. A pointer with
	// omitempty, so a list that did not load categories omits the key entirely
	// rather than sending an object full of empty strings.
	Category *CategoryResponse `json:"category,omitempty"`
}

// CreateProductRequest is the POST /api/v1/products body.
//
// PriceCents and Stock are POINTERS, and this is the reason: validator treats
// the zero value of a plain int as "absent", so `binding:"required"` on an
// int64 would reject a free product and `binding:"gte=0"` alone would silently
// accept a request that omitted the price and store it as 0. A pointer makes
// "sent as zero" and "not sent" different values, so required means what it
// says. This is the Go answer to Java's Integer-vs-int distinction.
type CreateProductRequest struct {
	CategoryID  string `json:"category_id" binding:"required,uuid"`
	Name        string `json:"name" binding:"required,min=1,max=200"`
	Slug        string `json:"slug" binding:"omitempty,min=1,max=200"`
	Description string `json:"description" binding:"max=5000"`
	PriceCents  *int64 `json:"price_cents" binding:"required,gte=0"`
	Stock       *int   `json:"stock" binding:"required,gte=0"`
}

// UpdateProductRequest is the PUT /api/v1/products/:id body.
//
// PUT replaces the product, so every field is required — a partial update would
// be PATCH, which this API does not offer. Slug is absent for the same reason
// as on a category: it is frozen once the product exists.
type UpdateProductRequest struct {
	CategoryID  string `json:"category_id" binding:"required,uuid"`
	Name        string `json:"name" binding:"required,min=1,max=200"`
	Description string `json:"description" binding:"max=5000"`
	PriceCents  *int64 `json:"price_cents" binding:"required,gte=0"`
	Stock       *int   `json:"stock" binding:"required,gte=0"`
}
