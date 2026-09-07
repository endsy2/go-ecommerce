package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"ecommerce/backend/internal/handler"
)

// Recovery turns a panic into a 500 instead of killing the process.
//
// CLAUDE.md is explicit that this is a last resort, not an error-handling
// strategy: request paths return errors, they do not panic. Reaching this
// middleware means there is a bug to fix, so it logs the full stack trace.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"request_id", handler.RequestIDFrom(c),
					"stack", string(debug.Stack()),
				)
				handler.RespondError(c, http.StatusInternalServerError,
					"internal_error", "an unexpected error occurred")
			}
		}()
		c.Next()
	}
}
