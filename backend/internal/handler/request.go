package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ecommerce/backend/internal/domain"
)

// Pagination defaults and the hard ceiling from CLAUDE.md. The cap exists so a
// client cannot ask for the whole table in one request: without it, ?limit=
// is an unauthenticated way to make the database do unbounded work.
const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// Pagination is a validated ?page=&limit= pair. Handlers get one from
// ParsePagination and pass Limit and Offset down to a repository; nothing below
// the handler layer ever reads a query string.
type Pagination struct {
	Page  int
	Limit int
}

// Offset converts the 1-based page number into the row offset SQL wants.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.Limit
}

// ParsePagination reads ?page= and ?limit=, applying the defaults and the cap.
//
// A limit above the maximum is CLAMPED to it rather than rejected: ?limit=500
// is a client asking for as much as it can get, not an error, and answering it
// with 100 rows plus an honest meta.limit is more useful than a 422. Garbage
// (?page=abc, ?limit=-1) is a genuine mistake and does return 422.
func ParsePagination(c *gin.Context) (Pagination, error) {
	page, err := positiveQuery(c, "page", DefaultPage)
	if err != nil {
		return Pagination{}, err
	}

	limit, err := positiveQuery(c, "limit", DefaultLimit)
	if err != nil {
		return Pagination{}, err
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	return Pagination{Page: page, Limit: limit}, nil
}

// positiveQuery reads one query parameter as a positive integer, falling back
// when it is absent.
func positiveQuery(c *gin.Context, name string, fallback int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		// domain.Validationf, not a bare error: HandleError turns it into the
		// same 422 shape a failed binding tag produces, so a client sees one
		// kind of validation failure rather than two.
		return 0, domain.Validationf("%s must be a positive integer", name)
	}
	return v, nil
}

// NewMeta builds the pagination envelope for a list response.
func NewMeta(p Pagination, total int64) Meta {
	// Ceiling division: 101 items at 20 per page is 6 pages, not 5. Done in
	// integer arithmetic because converting to float to call math.Ceil would
	// lose precision on large counts.
	totalPages := int((total + int64(p.Limit) - 1) / int64(p.Limit))

	return Meta{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
	}
}

// uuidParam reads a path parameter that must be a uuid.
//
// A malformed id is a 422 rather than a 404: the request is unprocessable, and
// answering 404 would imply the server looked for it and found nothing.
func uuidParam(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, domain.Validationf("%s must be a valid uuid", name)
	}
	return id, nil
}
