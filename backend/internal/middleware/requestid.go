// Package middleware holds cross-cutting HTTP concerns: recovery, logging,
// request correlation, CORS. Order of installation matters and is fixed in
// internal/router.
package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"ecommerce/backend/internal/handler"
)

// HeaderRequestID is the header carrying the correlation id, both inbound
// (honoured if a proxy already set one) and outbound.
const HeaderRequestID = "X-Request-ID"

// RequestID assigns every request a correlation id and echoes it back.
//
// This must be installed before the logger so that every log line, and every
// error response, can be tied to the exact request a user is complaining about.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = newRequestID()
		}
		c.Set(handler.RequestIDKey, id)
		c.Writer.Header().Set(HeaderRequestID, id)
		c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is not recoverable in any useful way here, and a
		// request must not be dropped over a missing correlation id.
		return "unknown"
	}
	return hex.EncodeToString(b)
}
