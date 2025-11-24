package types

import (
	"errors"
	"fmt"
)

// Common error categories for consistent error handling
const (
	ErrorCategoryValidation    = "validation"
	ErrorCategoryNotFound      = "not_found"
	ErrorCategoryTimeout       = "timeout"
	ErrorCategoryUnavailable   = "unavailable"
	ErrorCategoryInternal      = "internal"
	ErrorCategoryExtraction    = "extraction"
	ErrorCategoryConfiguration = "configuration"
)

// Common error codes for structured error handling
const (
	ErrorCodeInvalidConfig      = "invalid_config"
	ErrorCodeInvalidInput       = "invalid_input"
	ErrorCodeResourceNotFound   = "resource_not_found"
	ErrorCodeTimeout            = "timeout"
	ErrorCodeServiceUnavailable = "service_unavailable"
	ErrorCodeInternalError      = "internal_error"
	ErrorCodeExtractionFailed   = "extraction_failed"
	ErrorCodeValidationFailed   = "validation_failed"
)

// Standard errors for common cases
var (
	ErrInvalidConfig      = &ConfigurationError{Field: "config", Reason: "configuration is invalid"}
	ErrInvalidInput       = &ValidationError{Field: "input", Message: "input validation failed"}
	ErrResourceNotFound   = fmt.Errorf("%s: resource not found", ErrorCodeResourceNotFound)
	ErrTimeout            = fmt.Errorf("%s: operation timed out", ErrorCodeTimeout)
	ErrServiceUnavailable = fmt.Errorf("%s: service unavailable", ErrorCodeServiceUnavailable)
	ErrInternalError      = fmt.Errorf("%s: internal system error", ErrorCodeInternalError)
	ErrExtractionFailed   = fmt.Errorf("%s: extraction operation failed", ErrorCodeExtractionFailed)
)

// ExtractorError represents errors that occur during extraction operations
type ExtractorError struct {
	Category   string
	Code       string
	Message    string
	Details    string
	Method     ExtractorMethod
	DocumentID string
	Cause      error
}

func (e *ExtractorError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ExtractorError) Unwrap() error {
	return e.Cause
}

// NewExtractorError creates a new ExtractorError
func NewExtractorError(category, code, message, details string, method ExtractorMethod, documentID string, cause error) *ExtractorError {
	return &ExtractorError{
		Category:   category,
		Code:       code,
		Message:    message,
		Details:    details,
		Method:     method,
		DocumentID: documentID,
		Cause:      cause,
	}
}

// ConfigurationError represents configuration validation errors
type ConfigurationError struct {
	Field  string
	Value  any
	Reason string
	Cause  error
}

func (e *ConfigurationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("configuration error in field '%s': %s (caused by: %v)", e.Field, e.Reason, e.Cause)
	}
	return fmt.Sprintf("configuration error in field '%s': %s", e.Field, e.Reason)
}

func (e *ConfigurationError) Unwrap() error {
	return e.Cause
}

// NewConfigurationError creates a new ConfigurationError
func NewConfigurationError(field string, value any, reason string, cause error) *ConfigurationError {
	return &ConfigurationError{
		Field:  field,
		Value:  value,
		Reason: reason,
		Cause:  cause,
	}
}

// ValidationError represents input validation errors
type ValidationError struct {
	Field   string
	Value   any
	Rule    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError
func NewValidationError(field string, value any, rule, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Rule:    rule,
		Message: message,
	}
}

// WrapError wraps an error with additional context using fmt.Errorf with %w verb
func WrapError(err error, message string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(message+": %w", append(args, err)...)
}

// IsErrorCategory checks if an error belongs to a specific category
func IsErrorCategory(err error, category string) bool {
	var extractorErr *ExtractorError
	if errors.As(err, &extractorErr) {
		return extractorErr.Category == category
	}

	var configErr *ConfigurationError
	if errors.As(err, &configErr) {
		return category == ErrorCategoryConfiguration
	}

	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return category == ErrorCategoryValidation
	}

	return false
}

// IsErrorCode checks if an error has a specific error code
func IsErrorCode(err error, code string) bool {
	var extractorErr *ExtractorError
	if errors.As(err, &extractorErr) {
		return extractorErr.Code == code
	}

	// Check against standard validation errors
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		// ValidationError doesn't have a code field, so we map based on standard codes
		switch code {
		case ErrorCodeValidationFailed:
			return true
		case ErrorCodeInvalidInput:
			return true
		}
	}

	// Check against configuration errors
	var configErr *ConfigurationError
	if errors.As(err, &configErr) {
		// ConfigurationError doesn't have a code field, so we map based on standard codes
		switch code {
		case ErrorCodeInvalidConfig:
			return true
		case ErrorCodeInvalidInput:
			return true
		}
	}

	return false
}
