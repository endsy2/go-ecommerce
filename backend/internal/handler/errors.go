package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"ecommerce/backend/internal/domain"
)

// HandleError is the single place a domain error becomes an HTTP response.
//
// Handlers call this and return; they never build error JSON inline. That is
// what keeps the status code for "not found" identical across every endpoint,
// and it is the one function to change if the error envelope ever evolves.
//
// This is the Go equivalent of a Spring @ControllerAdvice, minus the reflection:
// it is an ordinary function called explicitly at each return site.
func HandleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		RespondError(c, http.StatusNotFound, "not_found", err.Error())

	case errors.Is(err, domain.ErrConflict):
		RespondError(c, http.StatusConflict, "conflict", err.Error())

	case errors.Is(err, domain.ErrForbidden):
		RespondError(c, http.StatusForbidden, "forbidden", err.Error())

	case errors.Is(err, domain.ErrUnauthorized):
		RespondError(c, http.StatusUnauthorized, "unauthorized", err.Error())

	case errors.Is(err, domain.ErrValidation):
		RespondError(c, http.StatusUnprocessableEntity, "validation_failed", err.Error())

	default:
		// An unmapped error is a bug or an infrastructure failure. Log the real
		// cause with the request id for correlation, but return a generic
		// message: internal errors often quote SQL or connection strings.
		slog.Error("unhandled error",
			"error", err,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"request_id", RequestIDFrom(c),
		)
		RespondError(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}

// HandleBindingError converts a request-binding failure into a 422 with
// field-level detail, as CLAUDE.md requires for invalid input.
func HandleBindingError(c *gin.Context, err error) {
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		fields := make([]FieldError, 0, len(validationErrs))
		for _, fe := range validationErrs {
			fields = append(fields, FieldError{
				Field:   fe.Field(),
				Message: describeValidation(fe),
			})
		}
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, errorBody{
			Error: errorDetail{
				Code:    "validation_failed",
				Message: "one or more fields are invalid",
				Fields:  fields,
			},
		})
		return
	}

	// Malformed JSON, wrong type, unreadable body: the request never became a
	// struct, so there are no per-field errors to report.
	RespondError(c, http.StatusBadRequest, "invalid_request", "request body could not be parsed")
}

// describeValidation turns a validator tag into a message a client can act on.
func describeValidation(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param()
	case "max":
		return "must be at most " + fe.Param()
	case "gt":
		return "must be greater than " + fe.Param()
	case "gte":
		return "must be greater than or equal to " + fe.Param()
	case "oneof":
		return "must be one of: " + fe.Param()
	default:
		return "is invalid"
	}
}
