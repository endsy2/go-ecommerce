package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"ecommerce/backend/internal/handler"
)

// Logger writes one structured line per request.
//
// gin.Default() installs gin's own text logger; we use gin.New() and this
// instead so output is structured (slog) and carries the request id.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"request_id", handler.RequestIDFrom(c),
		}
		if query != "" {
			attrs = append(attrs, "query", query)
		}

		switch {
		case c.Writer.Status() >= 500:
			slog.Error("request failed", attrs...)
		case c.Writer.Status() >= 400:
			slog.Warn("request rejected", attrs...)
		default:
			slog.Info("request", attrs...)
		}
	}
}
