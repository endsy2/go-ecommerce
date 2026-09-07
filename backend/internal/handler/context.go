package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ecommerce/backend/internal/model"
)

// The gin context keys set by middleware and read by handlers.
//
// They live in handler rather than middleware so that middleware can import
// handler (for RespondError when rejecting a request) without a cycle. The
// dependency runs one way: middleware -> handler.
//
// Handlers must never write these string literals themselves. That is the whole
// point of the accessors below: a typo in "user_id" at one call site is a
// compile error here and a silent nil at runtime there.
const (
	RequestIDKey = "request_id"
	UserIDKey    = "user_id"
	UserRoleKey  = "user_role"
)

// RequestIDFrom returns the current request's id, or "" if the middleware is not
// installed. Include it in logs so a client-reported error can be traced back to
// a single request.
func RequestIDFrom(c *gin.Context) string {
	if v, ok := c.Get(RequestIDKey); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// UserIDFrom returns the authenticated user's id. The bool is false when
// RequireAuth did not run on this route, which for a route inside the protected
// group is a wiring bug rather than an anonymous request.
//
// Gin's context is a map[string]any, so the value has to be type-asserted
// somewhere. Doing it here, once, is what lets every handler work with a real
// uuid.UUID instead of repeating an assertion that could be wrong in one place
// and right in twenty others.
func UserIDFrom(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(UserIDKey)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// UserRoleFrom returns the authenticated user's role, on the same terms as
// UserIDFrom.
//
// This reports what the token claimed at the moment it was issued. For an
// authorisation check on a route that is already role-gated it is exactly right;
// for anything that must reflect the user's role right now, read the row.
func UserRoleFrom(c *gin.Context) (model.Role, bool) {
	v, ok := c.Get(UserRoleKey)
	if !ok {
		return "", false
	}
	role, ok := v.(model.Role)
	return role, ok
}
