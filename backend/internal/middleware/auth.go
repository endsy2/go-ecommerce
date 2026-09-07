package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"ecommerce/backend/internal/handler"
	"ecommerce/backend/internal/model"
	"ecommerce/backend/internal/util"
)

// authScheme is the HTTP authentication scheme this API accepts, used both when
// parsing the Authorization header and when telling a rejected client what it
// should have sent.
const authScheme = "Bearer"

// TokenValidator is the slice of *util.JWT this middleware needs.
//
// Declared here, by the consumer, exactly as service.tokenIssuer is: the
// middleware asks for one method rather than a whole type, and a test satisfies
// it with a four-line struct instead of minting real HS256 tokens. It is
// exported only because router.New has to name the type to pass one through.
type TokenValidator interface {
	ValidateToken(raw string) (*util.Claims, error)
}

// RequireAuth rejects any request that does not carry a valid token, and puts
// the caller's id and role on the context for everything downstream.
//
// Attach it to a route GROUP rather than to individual routes. A group cannot
// have a route added to it that skips its middleware, so the protection is
// structural instead of something a future handler has to remember.
func RequireAuth(tokens TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := credential(c)
		if !ok {
			unauthorized(c, "authentication required")
			return
		}

		claims, err := tokens.ValidateToken(raw)
		if err != nil {
			// Deliberately one message for every rejection reason. util.JWT
			// already refuses to say whether a token was expired or forged;
			// repeating that distinction here would give it away anyway.
			unauthorized(c, "invalid or expired token")
			return
		}

		userID, err := claims.UserID()
		if err != nil {
			// ValidateToken has already checked the subject parses, so this is
			// unreachable in practice. Handled rather than ignored because the
			// alternative is putting uuid.Nil on the context and letting a
			// handler scope a query to it.
			unauthorized(c, "invalid or expired token")
			return
		}

		// Typed values, not strings: handler.UserIDFrom does the one type
		// assertion so no handler has to.
		c.Set(handler.UserIDKey, userID)
		c.Set(handler.UserRoleKey, claims.Role)

		c.Next()
	}
}

// RequireRole rejects an authenticated caller whose role is not in the allowed
// set. It must run after RequireAuth — nest its group inside the authenticated
// one so that ordering is structural rather than conventional.
func RequireRole(roles ...model.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := handler.UserRoleFrom(c)
		if !ok {
			// No role on the context means RequireAuth never ran: this route is
			// mis-wired. Fail closed with 401 rather than 403 — "we do not know
			// who you are" is the true statement, and it can never accidentally
			// admit an unauthenticated request.
			unauthorized(c, "authentication required")
			return
		}

		if !slices.Contains(roles, role) {
			// 403, not 404. Hiding the existence of admin routes from a
			// logged-in customer buys nothing: the frontend bundle lists them.
			handler.RespondError(c, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}

		c.Next()
	}
}

// credential pulls the token out of the request, preferring the cookie.
//
// The cookie is httpOnly, so JavaScript cannot read it and an XSS bug cannot
// exfiltrate it — it is the credential a browser should be using. The Bearer
// header is the escape hatch for clients with no cookie jar: curl, a mobile app,
// an integration test. Cookie wins when both are present, so an attacker-supplied
// header cannot downgrade a request that already proved it holds the cookie.
func credential(c *gin.Context) (string, bool) {
	if cookie, err := c.Cookie(handler.AuthCookieName); err == nil && cookie != "" {
		return cookie, true
	}

	header := c.GetHeader("Authorization")
	if header == "" {
		return "", false
	}

	// "Bearer <token>" split on the first space only: a token never contains
	// one, but splitting on all of them would silently accept a malformed
	// header with trailing junk.
	scheme, token, found := strings.Cut(header, " ")

	// EqualFold because RFC 7235 defines the scheme as case-insensitive, and
	// real clients do send "bearer".
	if !found || !strings.EqualFold(scheme, authScheme) {
		return "", false
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

// unauthorized writes the 401 body plus the header RFC 7235 requires on one:
// a 401 is supposed to tell the client how to authenticate, not just that it
// failed.
//
// No explicit c.Abort() is needed — handler.RespondError goes through
// c.AbortWithStatusJSON, which both writes the body and stops the chain.
func unauthorized(c *gin.Context, message string) {
	c.Writer.Header().Set("WWW-Authenticate", authScheme)
	handler.RespondError(c, http.StatusUnauthorized, "unauthorized", message)
}
