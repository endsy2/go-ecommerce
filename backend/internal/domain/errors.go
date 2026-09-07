// Package domain holds the error vocabulary shared by every layer.
//
// It lives in its own package rather than in service/ because repositories also
// need to return these (translating gorm.ErrRecordNotFound into ErrNotFound),
// and service imports repository — putting them in service would create an
// import cycle the moment the first repository is written.
package domain

import (
	"errors"
	"fmt"
)

// The sentinel errors services return. Handlers never inspect the concrete
// cause; they pass the error to handler.HandleError, which maps it to a status.
//
// Compared to Spring: these are the equivalent of a small set of exception types
// plus one @ControllerAdvice, except the compiler cannot forget to handle them
// because errors are ordinary return values.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation failed")
)

// Error wraps a sentinel with a message safe to show the caller.
//
// Wrapping rather than replacing is what makes errors.Is(err, ErrNotFound) still
// work several layers up the call stack.
type Error struct {
	sentinel error
	message  string
}

func (e *Error) Error() string {
	if e.message == "" {
		return e.sentinel.Error()
	}
	return e.message
}

// Unwrap lets errors.Is match against the sentinel.
func (e *Error) Unwrap() error { return e.sentinel }

// NotFoundf builds an ErrNotFound carrying a caller-safe message.
func NotFoundf(format string, args ...any) *Error {
	return &Error{sentinel: ErrNotFound, message: fmt.Sprintf(format, args...)}
}

// Conflictf builds an ErrConflict carrying a caller-safe message.
func Conflictf(format string, args ...any) *Error {
	return &Error{sentinel: ErrConflict, message: fmt.Sprintf(format, args...)}
}

// Forbiddenf builds an ErrForbidden carrying a caller-safe message.
func Forbiddenf(format string, args ...any) *Error {
	return &Error{sentinel: ErrForbidden, message: fmt.Sprintf(format, args...)}
}

// Unauthorizedf builds an ErrUnauthorized carrying a caller-safe message.
func Unauthorizedf(format string, args ...any) *Error {
	return &Error{sentinel: ErrUnauthorized, message: fmt.Sprintf(format, args...)}
}

// Validationf builds an ErrValidation carrying a caller-safe message.
func Validationf(format string, args ...any) *Error {
	return &Error{sentinel: ErrValidation, message: fmt.Sprintf(format, args...)}
}
