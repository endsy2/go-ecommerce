// Package handler binds and validates requests, calls services, and writes
// responses. It holds no business logic and never imports GORM.
package handler

import "github.com/gin-gonic/gin"

// The wire format from CLAUDE.md: every response is either
//
//	{"data": ..., "meta": ...}   on success
//	{"error": {"code", "message"}}   on failure
//
// Handlers must go through these helpers rather than calling c.JSON with an
// inline map, so the envelope stays identical across every endpoint.

type successBody struct {
	Data any   `json:"data"`
	Meta *Meta `json:"meta,omitempty"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

// Meta carries pagination for list endpoints, which CLAUDE.md requires to always
// be paginated.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// FieldError is one field-level validation failure, returned with 422.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// OK writes 200 with a data envelope.
func OK(c *gin.Context, data any) {
	Respond(c, 200, data)
}

// Created writes 201 with a data envelope.
func Created(c *gin.Context, data any) {
	Respond(c, 201, data)
}

// Respond writes an arbitrary status with a data envelope.
func Respond(c *gin.Context, status int, data any) {
	c.JSON(status, successBody{Data: data})
}

// NoContent writes 204 with no body, for a successful delete.
//
// There is deliberately no envelope here: 204 means "done, and there is nothing
// to send", and a body would contradict the status line.
func NoContent(c *gin.Context) {
	c.Status(204)
}

// RespondList writes 200 with a data envelope plus pagination meta.
func RespondList(c *gin.Context, data any, meta Meta) {
	c.JSON(200, successBody{Data: data, Meta: &meta})
}

// RespondError writes an error envelope. Prefer HandleError, which derives the
// status and code from a domain error instead of hardcoding them at the call site.
func RespondError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
