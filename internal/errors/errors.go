// Package errors provides standardized error handling for the CDV service.
package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents a standardized error code for the CDV service.
type ErrorCode string

const (
	// Validation errors
	CDV_VALIDATION     ErrorCode = "CDV_VALIDATION"     // General validation error
	CDV_SCHEMA_REJECT  ErrorCode = "CDV_SCHEMA_REJECT"  // Schema validation failed
	CDV_BAD_REQUEST    ErrorCode = "CDV_BAD_REQUEST"    // Bad request
	CDV_CURSOR_INVALID ErrorCode = "CDV_CURSOR_INVALID" // Invalid cursor

	// Authentication/Authorization errors
	CDV_AUTHZ        ErrorCode = "CDV_AUTHZ"        // Authorization failed
	CDV_AUTHN        ErrorCode = "CDV_AUTHN"        // Authentication failed
	CDV_JWT_INVALID  ErrorCode = "CDV_JWT_INVALID"  // Invalid JWT
	CDV_JWT_EXPIRED  ErrorCode = "CDV_JWT_EXPIRED"  // Expired JWT
	CDV_JWT_MALFORMED ErrorCode = "CDV_JWT_MALFORMED" // Malformed JWT
	CDV_DID_MISMATCH ErrorCode = "CDV_DID_MISMATCH" // DID mismatch

	// Resource errors
	CDV_NOT_FOUND      ErrorCode = "CDV_NOT_FOUND"      // Resource not found
	CDV_CONFLICT       ErrorCode = "CDV_CONFLICT"       // Resource conflict
	CDV_MEDIA_CHECKSUM ErrorCode = "CDV_MEDIA_CHECKSUM" // Media checksum mismatch
	CDV_MEDIA_SIZE     ErrorCode = "CDV_MEDIA_SIZE"     // Media size limit exceeded
	CDV_MEDIA_TYPE     ErrorCode = "CDV_MEDIA_TYPE"     // Media type not allowed

	// Rate limiting
	CDV_RATE_LIMIT ErrorCode = "CDV_RATE_LIMIT" // Rate limit exceeded

	// Server errors
	CDV_INTERNAL     ErrorCode = "CDV_INTERNAL"     // Internal server error
	CDV_UNAVAILABLE  ErrorCode = "CDV_UNAVAILABLE"  // Service unavailable
	CDV_NOT_IMPLEMENTED ErrorCode = "CDV_NOT_IMPLEMENTED" // Not implemented
)

// Error represents a standardized error response.
type Error struct {
	Code         ErrorCode `json:"code"`
	Message      string    `json:"message"`
	CorrelationID string    `json:"correlationId"`
	Details      interface{} `json:"details,omitempty"`
	HTTPStatus   int       `json:"-"`
}

// New creates a new Error with the specified code and message.
func New(code ErrorCode, message string, correlationID string) *Error {
	return &Error{
		Code:         code,
		Message:      message,
		CorrelationID: correlationID,
		HTTPStatus:   httpStatusCodeForCode(code),
	}
}

// NewWithDetails creates a new Error with the specified code, message, and details.
func NewWithDetails(code ErrorCode, message string, correlationID string, details interface{}) *Error {
	return &Error{
		Code:         code,
		Message:      message,
		CorrelationID: correlationID,
		Details:      details,
		HTTPStatus:   httpStatusCodeForCode(code),
	}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Details != nil {
		return fmt.Sprintf("%s: %s (details: %v)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// httpStatusCodeForCode maps error codes to HTTP status codes.
func httpStatusCodeForCode(code ErrorCode) int {
	switch code {
	case CDV_VALIDATION, CDV_SCHEMA_REJECT, CDV_BAD_REQUEST, CDV_CURSOR_INVALID:
		return http.StatusBadRequest
	case CDV_AUTHZ, CDV_DID_MISMATCH:
		return http.StatusForbidden
	case CDV_AUTHN, CDV_JWT_INVALID, CDV_JWT_EXPIRED, CDV_JWT_MALFORMED:
		return http.StatusUnauthorized
	case CDV_NOT_FOUND:
		return http.StatusNotFound
	case CDV_CONFLICT:
		return http.StatusConflict
	case CDV_MEDIA_CHECKSUM, CDV_MEDIA_SIZE, CDV_MEDIA_TYPE:
		return http.StatusBadRequest
	case CDV_RATE_LIMIT:
		return http.StatusTooManyRequests
	case CDV_UNAVAILABLE:
		return http.StatusServiceUnavailable
	case CDV_NOT_IMPLEMENTED:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
