package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS allows the Next.js frontend to call the API from another origin.
//
// The allowed origins come from config rather than being "*", because the
// frontend sends its auth cookie: a wildcard origin is incompatible with
// Access-Control-Allow-Credentials, and browsers will reject the response.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" && slices.Contains(allowedOrigins, origin) {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers",
				strings.Join([]string{"Origin", "Content-Type", "Accept", "Authorization", HeaderRequestID}, ", "))
			h.Set("Access-Control-Max-Age", "86400")
			// Caches must not serve one origin's response to another.
			h.Add("Vary", "Origin")
		}

		// Preflight never reaches a route; answer it here and stop.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
