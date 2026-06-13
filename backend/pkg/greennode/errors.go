// Package greennode is a standalone, vendor-isolated Go client for the
// GreenNode AgentBase platform (config resolution, IAM/OAuth2 auth, the Memory
// API and the agent-runtime HTTP contract).
//
// It is intentionally self-contained: nothing here imports the application's
// internal packages. The application talks to GreenNode only through the
// domain ports defined under internal/domain, with a thin adapter wrapping this
// SDK. That keeps the rest of the codebase free of any vendor coupling — if we
// later self-host memory or move off AgentBase, only the adapter changes.
package greennode

import (
	"errors"
	"fmt"
)

// Error is the common error type returned by the SDK. StatusCode is set for
// HTTP failures (0 otherwise). Use errors.As to inspect it.
type Error struct {
	Message    string
	StatusCode int
	cause      error
}

func (e *Error) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("greennode: %s (status %d)", e.Message, e.StatusCode)
	}
	return "greennode: " + e.Message
}

func (e *Error) Unwrap() error { return e.cause }

// ConfigError signals missing or invalid configuration (e.g. credentials).
func ConfigError(msg string, cause error) *Error {
	return &Error{Message: msg, cause: cause}
}

// RequestError signals a failed HTTP request to a GreenNode service.
func RequestError(msg string, statusCode int, cause error) *Error {
	return &Error{Message: msg, StatusCode: statusCode, cause: cause}
}

// ErrCredentialsMissing is returned when IAM credentials are required but absent.
var ErrCredentialsMissing = errors.New("greennode: IAM credentials are required but not available")
