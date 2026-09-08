package util

import (
	"regexp"
	"strings"
)

// nonSlugChars matches every run of characters that cannot appear in a slug.
//
// Compiled once at package level, not inside Slugify: regexp.MustCompile parses
// the pattern, and doing that on every call would be real work repeated for no
// reason. A package-level var is the idiomatic home for a compiled regexp.
var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a display name into a URL-safe identifier:
// "Blue Ceramic Mug!" becomes "blue-ceramic-mug".
//
// It returns "" when the name contains nothing usable — a name of only
// punctuation, or one written entirely in a non-Latin script. Callers must
// treat that as a validation failure rather than storing an empty slug, which
// would collide with every other unusable name.
func Slugify(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	dashed := nonSlugChars.ReplaceAllString(lower, "-")
	return strings.Trim(dashed, "-")
}
