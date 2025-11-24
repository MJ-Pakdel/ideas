package types

import (
	"context"
	"log/slog"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogContext represents common context for all log entries
type LogContext struct {
	Component  string         `json:"component"`
	Operation  string         `json:"operation"`
	RequestID  string         `json:"request_id,omitempty"`
	DocumentID string         `json:"document_id,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
	Duration   time.Duration  `json:"duration,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// StandardLogger provides consistent logging methods across the application
type StandardLogger struct {
	logger *slog.Logger
	ctx    LogContext
}

// NewStandardLogger creates a new StandardLogger with base context
func NewStandardLogger(component string) *StandardLogger {
	return &StandardLogger{
		logger: slog.Default(),
		ctx: LogContext{
			Component: component,
			Metadata:  make(map[string]any),
		},
	}
}

// WithOperation returns a new logger with operation context
func (l *StandardLogger) WithOperation(operation string) *StandardLogger {
	newCtx := l.ctx
	newCtx.Operation = operation
	return &StandardLogger{
		logger: l.logger,
		ctx:    newCtx,
	}
}

// WithRequestID returns a new logger with request ID context
func (l *StandardLogger) WithRequestID(requestID string) *StandardLogger {
	newCtx := l.ctx
	newCtx.RequestID = requestID
	return &StandardLogger{
		logger: l.logger,
		ctx:    newCtx,
	}
}

// WithDocumentID returns a new logger with document ID context
func (l *StandardLogger) WithDocumentID(documentID string) *StandardLogger {
	newCtx := l.ctx
	newCtx.DocumentID = documentID
	return &StandardLogger{
		logger: l.logger,
		ctx:    newCtx,
	}
}

// WithDuration returns a new logger with duration context
func (l *StandardLogger) WithDuration(duration time.Duration) *StandardLogger {
	newCtx := l.ctx
	newCtx.Duration = duration
	return &StandardLogger{
		logger: l.logger,
		ctx:    newCtx,
	}
}

// WithMetadata returns a new logger with additional metadata
func (l *StandardLogger) WithMetadata(key string, value any) *StandardLogger {
	newCtx := l.ctx
	newMetadata := make(map[string]any)
	for k, v := range l.ctx.Metadata {
		newMetadata[k] = v
	}
	newMetadata[key] = value
	newCtx.Metadata = newMetadata
	return &StandardLogger{
		logger: l.logger,
		ctx:    newCtx,
	}
}

// buildLogArgs creates slog attributes from context
func (l *StandardLogger) buildLogArgs() []slog.Attr {
	var attrs []slog.Attr

	attrs = append(attrs, slog.String("component", l.ctx.Component))

	if l.ctx.Operation != "" {
		attrs = append(attrs, slog.String("operation", l.ctx.Operation))
	}
	if l.ctx.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", l.ctx.RequestID))
	}
	if l.ctx.DocumentID != "" {
		attrs = append(attrs, slog.String("document_id", l.ctx.DocumentID))
	}
	if l.ctx.UserID != "" {
		attrs = append(attrs, slog.String("user_id", l.ctx.UserID))
	}
	if l.ctx.Duration > 0 {
		attrs = append(attrs, slog.Duration("duration", l.ctx.Duration))
	}

	// Add metadata
	for k, v := range l.ctx.Metadata {
		attrs = append(attrs, slog.Any(k, v))
	}

	return attrs
}

// Debug logs a debug message with standard context
func (l *StandardLogger) Debug(ctx context.Context, msg string, args ...slog.Attr) {
	allArgs := append(l.buildLogArgs(), args...)
	l.logger.LogAttrs(ctx, slog.LevelDebug, msg, allArgs...)
}

// Info logs an info message with standard context
func (l *StandardLogger) Info(ctx context.Context, msg string, args ...slog.Attr) {
	allArgs := append(l.buildLogArgs(), args...)
	l.logger.LogAttrs(ctx, slog.LevelInfo, msg, allArgs...)
}

// Warn logs a warning message with standard context
func (l *StandardLogger) Warn(ctx context.Context, msg string, args ...slog.Attr) {
	allArgs := append(l.buildLogArgs(), args...)
	l.logger.LogAttrs(ctx, slog.LevelWarn, msg, allArgs...)
}

// Error logs an error message with standard context
func (l *StandardLogger) Error(ctx context.Context, msg string, err error, args ...slog.Attr) {
	allArgs := append(l.buildLogArgs(), args...)
	allArgs = append(allArgs, slog.Any("error", err))
	l.logger.LogAttrs(ctx, slog.LevelError, msg, allArgs...)
}

// LogExtractionStart logs the start of an extraction operation with standard fields
func (l *StandardLogger) LogExtractionStart(ctx context.Context, method ExtractorMethod, documentID string) {
	l.WithOperation("extraction_start").
		WithDocumentID(documentID).
		WithMetadata("method", string(method)).
		Info(ctx, "Starting extraction operation")
}

// LogExtractionComplete logs the completion of an extraction operation with metrics
func (l *StandardLogger) LogExtractionComplete(ctx context.Context, method ExtractorMethod, documentID string, resultCount int, duration time.Duration) {
	l.WithOperation("extraction_complete").
		WithDocumentID(documentID).
		WithDuration(duration).
		WithMetadata("method", string(method)).
		WithMetadata("result_count", resultCount).
		Info(ctx, "Extraction operation completed")
}

// LogExtractionError logs an extraction error with context
func (l *StandardLogger) LogExtractionError(ctx context.Context, method ExtractorMethod, documentID string, err error) {
	l.WithOperation("extraction_error").
		WithDocumentID(documentID).
		WithMetadata("method", string(method)).
		Error(ctx, "Extraction operation failed", err)
}

// LogValidationError logs a validation error with field context
func (l *StandardLogger) LogValidationError(ctx context.Context, field string, value any, err error) {
	l.WithOperation("validation_error").
		WithMetadata("field", field).
		WithMetadata("value", value).
		Error(ctx, "Validation failed", err)
}

// LogConfigurationError logs a configuration error
func (l *StandardLogger) LogConfigurationError(ctx context.Context, component string, err error) {
	l.WithOperation("configuration_error").
		WithMetadata("configuration_component", component).
		Error(ctx, "Configuration error", err)
}

// LogSystemEvent logs system-level events (startup, shutdown, etc.)
func (l *StandardLogger) LogSystemEvent(ctx context.Context, event string, details map[string]any) {
	logger := l.WithOperation("system_event").WithMetadata("event", event)
	for k, v := range details {
		logger = logger.WithMetadata(k, v)
	}
	logger.Info(ctx, "System event occurred")
}
