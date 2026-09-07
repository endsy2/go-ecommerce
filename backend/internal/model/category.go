package model

// Category groups products. Flat for the MVP: there is no parent_id, so no
// nesting. Adding a hierarchy later is a migration plus a self-referencing FK,
// and CLAUDE.md requires approval before that.
type Category struct {
	Base

	Name string `gorm:"type:text;not null"`

	// Slug is the URL-safe identifier (/categories/garden-tools). It is unique,
	// so it can be used to look a category up from the frontend without
	// exposing a uuid in the URL.
	Slug string `gorm:"type:text;not null;uniqueIndex"`
}

func (Category) TableName() string { return "categories" }
