package session

import (
	"errors"
	"fmt"
)

// Code classifies a domain failure so delivery can render it without
// inspecting messages.
type Code string

const (
	// CodeInvalid marks an input that violates a domain rule.
	CodeInvalid Code = "invalid"
	// CodeNotFound marks a missing track or take.
	CodeNotFound Code = "not_found"
	// CodeConflict marks a state clash such as a doubly armed channel.
	CodeConflict Code = "conflict"
)

// Error is a typed domain failure.
type Error struct {
	Code    Code
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// IsCode reports whether err is a domain error with the given code.
func IsCode(err error, code Code) bool {
	var domainErr *Error
	return errors.As(err, &domainErr) && domainErr.Code == code
}

func invalid(format string, args ...any) error {
	return &Error{Code: CodeInvalid, Message: fmt.Sprintf(format, args...)}
}

func notFound(format string, args ...any) error {
	return &Error{Code: CodeNotFound, Message: fmt.Sprintf(format, args...)}
}

func conflict(format string, args ...any) error {
	return &Error{Code: CodeConflict, Message: fmt.Sprintf(format, args...)}
}
