/*
Copyright 2025 The KubeLB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package logger provides structured logging capabilities for the KubeLB CLI.
// It uses Go's standard slog package with custom handlers for CLI-friendly output.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Level represents the logging level for the CLI.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
	levelInfoStr = "info"
)

// String returns the string representation of the logging level.
func (l Level) String() string {
	switch l {
	case LevelError:
		return "error"
	case LevelWarn:
		return "warn"
	case LevelInfo:
		return levelInfoStr
	case LevelDebug:
		return "debug"
	case LevelTrace:
		return "trace"
	default:
		return levelInfoStr
	}
}

// ToSlogLevel converts our custom Level to slog.Level.
func (l Level) ToSlogLevel() slog.Level {
	switch l {
	case LevelError:
		return slog.LevelError
	case LevelWarn:
		return slog.LevelWarn
	case LevelInfo:
		return slog.LevelInfo
	case LevelDebug:
		return slog.LevelDebug
	case LevelTrace:
		return slog.LevelDebug - 4 // More verbose than debug
	default:
		return slog.LevelInfo
	}
}

// ParseLevel parses a string level into a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "error", "err":
		return LevelError
	case "warn", "warning":
		return LevelWarn
	case levelInfoStr, "information":
		return LevelInfo
	case "debug":
		return LevelDebug
	case "trace":
		return LevelTrace
	default:
		return LevelInfo
	}
}

// Format represents the output format for logs.
type Format string

const (
	// FormatCLI provides human-readable output with colors and emojis
	FormatCLI Format = "cli"
	// FormatJSON provides structured JSON output for tooling
	FormatJSON Format = "json"
	// FormatText provides plain text output without colors
	FormatText Format = "text"
)

// Config holds the configuration for the logger.
type Config struct {
	// Level sets the minimum logging level
	Level Level
	// Format sets the output format
	Format Format
	// Output sets the output destination (default: stderr)
	Output io.Writer
	// AddSource adds source file information to logs (default: false)
	AddSource bool
	// VerbosityLevel sets the CLI verbosity level (0-4)
	VerbosityLevel int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		Level:          LevelInfo,
		Format:         FormatCLI,
		Output:         os.Stderr,
		AddSource:      false,
		VerbosityLevel: 1, // Default to basic operations
	}
}

// Logger wraps slog.Logger with additional CLI-specific functionality.
type Logger struct {
	*slog.Logger
	config *Config
	mu     sync.RWMutex
}

var (
	// globalLogger holds the singleton logger instance
	globalLogger *Logger
	// globalMu protects access to globalLogger
	globalMu sync.RWMutex
)

// New creates a new logger with the provided configuration.
func New(config *Config) *Logger {
	if config == nil {
		config = DefaultConfig()
	}

	var handler slog.Handler

	switch config.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(config.Output, &slog.HandlerOptions{
			Level:     config.Level.ToSlogLevel(),
			AddSource: config.AddSource,
		})
	case FormatText:
		handler = slog.NewTextHandler(config.Output, &slog.HandlerOptions{
			Level:     config.Level.ToSlogLevel(),
			AddSource: config.AddSource,
		})
	case FormatCLI:
		fallthrough
	default:
		handler = NewCLIHandler(config)
	}

	return &Logger{
		Logger: slog.New(handler),
		config: config,
	}
}

// Setup initializes the global logger with the provided configuration.
func Setup(config *Config) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = New(config)
}

// Get returns the global logger instance.
func Get() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger == nil {
		// Initialize with default config if not set
		globalLogger = New(nil)
	}
	return globalLogger
}

// WithContext adds context to the logger for request tracing.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		Logger: l.With(contextFields(ctx)...),
		config: l.config,
	}
}

// WithOperation adds operation-specific fields to the logger.
func (l *Logger) WithOperation(operation string) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		Logger: l.With("operation", operation),
		config: l.config,
	}
}

// WithTunnel adds tunnel-specific fields to the logger.
func (l *Logger) WithTunnel(name string) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		Logger: l.With("tunnel", name),
		config: l.config,
	}
}

// WithFields adds arbitrary key-value pairs to the logger.
func (l *Logger) WithFields(fields ...any) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		Logger: l.With(fields...),
		config: l.config,
	}
}

// ShouldLog checks if a message at the given verbosity level should be logged.
func (l *Logger) ShouldLog(verbosityLevel int) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return verbosityLevel <= l.config.VerbosityLevel
}

// GetVerbosityLevel returns the current verbosity level.
func (l *Logger) GetVerbosityLevel() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config.VerbosityLevel
}

// SetVerbosityLevel updates the verbosity level.
func (l *Logger) SetVerbosityLevel(level int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.VerbosityLevel = level
}

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	tenantKey    contextKey = "tenant"
)

// contextFields extracts logging fields from context.
func contextFields(ctx context.Context) []any {
	var fields []any

	if requestID := ctx.Value(requestIDKey); requestID != nil {
		fields = append(fields, "request_id", requestID)
	}

	if tenant := ctx.Value(tenantKey); tenant != nil {
		fields = append(fields, "tenant", tenant)
	}

	return fields
}

// isTerminal checks if the writer is a terminal (for color detection).
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isatty(f.Fd())
	}
	return false
}

// Global convenience functions that use the global logger instance

// Error logs an error message.
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// Warn logs a warning message.
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Info logs an informational message.
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Debug logs a debug message.
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// With returns a logger with additional fields.
func With(args ...any) *Logger {
	return Get().WithFields(args...)
}

// WithOperation returns a logger with operation context.
func WithOperation(operation string) *Logger {
	return Get().WithOperation(operation)
}

// WithTunnel returns a logger with tunnel context.
func WithTunnel(name string) *Logger {
	return Get().WithTunnel(name)
}

// WithContext returns a logger with context.
func WithContext(ctx context.Context) *Logger {
	return Get().WithContext(ctx)
}
