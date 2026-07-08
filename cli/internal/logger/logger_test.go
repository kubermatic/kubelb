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

package logger

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelError, "error"},
		{LevelWarn, "warn"},
		{LevelInfo, "info"},
		{LevelDebug, "debug"},
		{LevelTrace, "trace"},
		{Level(999), "info"}, // Unknown level defaults to info
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestLevel_ToSlogLevel(t *testing.T) {
	tests := []struct {
		level    Level
		expected slog.Level
	}{
		{LevelError, slog.LevelError},
		{LevelWarn, slog.LevelWarn},
		{LevelInfo, slog.LevelInfo},
		{LevelDebug, slog.LevelDebug},
		{LevelTrace, slog.LevelDebug - 4},
	}

	for _, tt := range tests {
		if got := tt.level.ToSlogLevel(); got != tt.expected {
			t.Errorf("Level.ToSlogLevel() = %v, want %v", got, tt.expected)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"error", LevelError},
		{"err", LevelError},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"info", LevelInfo},
		{"information", LevelInfo},
		{"debug", LevelDebug},
		{"trace", LevelTrace},
		{"unknown", LevelInfo}, // Unknown level defaults to info
		{"", LevelInfo},        // Empty string defaults to info
	}

	for _, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.expected {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Level != LevelInfo {
		t.Errorf("DefaultConfig().Level = %v, want %v", config.Level, LevelInfo)
	}
	if config.Format != FormatCLI {
		t.Errorf("DefaultConfig().Format = %v, want %v", config.Format, FormatCLI)
	}
	if config.Output == nil {
		t.Error("DefaultConfig().Output is nil")
	}
	if config.VerbosityLevel != 1 {
		t.Errorf("DefaultConfig().VerbosityLevel = %v, want %v", config.VerbosityLevel, 1)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
		},
		{
			name: "custom config",
			config: &Config{
				Level:          LevelDebug,
				Format:         FormatJSON,
				Output:         &bytes.Buffer{},
				VerbosityLevel: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.config)
			if logger == nil {
				t.Fatal("New() returned nil")
			}
			if logger.Logger == nil {
				t.Error("New().Logger is nil")
			}
		})
	}
}

func TestLogger_WithContext(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(config)

	ctx := context.WithValue(context.Background(), requestIDKey, "test-123")
	ctx = context.WithValue(ctx, tenantKey, "test-tenant")

	contextLogger := logger.WithContext(ctx)
	contextLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test-123") {
		t.Error("Context logger should include request_id")
	}
	if !strings.Contains(output, "test-tenant") {
		t.Error("Context logger should include tenant")
	}
}

func TestLogger_WithOperation(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(config)
	opLogger := logger.WithOperation("test_operation")
	opLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test_operation") {
		t.Error("Operation logger should include operation field")
	}
}

func TestLogger_WithTunnel(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(config)
	tunnelLogger := logger.WithTunnel("my-tunnel")
	tunnelLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "my-tunnel") {
		t.Error("Tunnel logger should include tunnel field")
	}
}

func TestLogger_ShouldLog(t *testing.T) {
	config := &Config{
		Level:          LevelInfo,
		VerbosityLevel: 2,
		Output:         &bytes.Buffer{},
	}

	logger := New(config)

	tests := []struct {
		verbosity int
		expected  bool
	}{
		{0, true},  // Should always log level 0
		{1, true},  // Should log level 1 with verbosity 2
		{2, true},  // Should log level 2 with verbosity 2
		{3, false}, // Should not log level 3 with verbosity 2
		{4, false}, // Should not log level 4 with verbosity 2
	}

	for _, tt := range tests {
		if got := logger.ShouldLog(tt.verbosity); got != tt.expected {
			t.Errorf("ShouldLog(%d) = %v, want %v", tt.verbosity, got, tt.expected)
		}
	}
}

func TestSetup_Get(t *testing.T) {
	// Save original global logger
	originalLogger := globalLogger
	defer func() {
		globalMu.Lock()
		globalLogger = originalLogger
		globalMu.Unlock()
	}()

	config := &Config{
		Level:  LevelDebug,
		Output: &bytes.Buffer{},
	}

	Setup(config)
	logger := Get()

	if logger == nil {
		t.Error("Get() returned nil after Setup()")
	}

	// Test that Get() initializes with default config if not set
	globalMu.Lock()
	globalLogger = nil
	globalMu.Unlock()

	logger = Get()
	if logger == nil {
		t.Error("Get() returned nil when globalLogger was nil")
	}
}

func TestGlobalFunctions(t *testing.T) {
	// Save original global logger
	originalLogger := globalLogger
	defer func() {
		globalMu.Lock()
		globalLogger = originalLogger
		globalMu.Unlock()
	}()

	var buf bytes.Buffer
	config := &Config{
		Level:  LevelTrace,
		Format: FormatJSON,
		Output: &buf,
	}

	Setup(config)

	// Test global logging functions
	Error("error message", "key", "value")
	Warn("warn message", "key", "value")
	Info("info message", "key", "value")
	Debug("debug message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Error("Global Error() function not working")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Global Warn() function not working")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Global Info() function not working")
	}
	if !strings.Contains(output, "debug message") {
		t.Error("Global Debug() function not working")
	}
}

func TestGlobalWith(t *testing.T) {
	// Save original global logger
	originalLogger := globalLogger
	defer func() {
		globalMu.Lock()
		globalLogger = originalLogger
		globalMu.Unlock()
	}()

	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: &buf,
	}

	Setup(config)

	// Test global With functions
	With("key", "value").Info("test message")
	WithOperation("test_op").Info("test message")
	WithTunnel("test_tunnel").Info("test message")

	output := buf.String()
	if !strings.Contains(output, "\"key\":\"value\"") {
		t.Error("Global With() function not working")
	}
	if !strings.Contains(output, "test_op") {
		t.Error("Global WithOperation() function not working")
	}
	if !strings.Contains(output, "test_tunnel") {
		t.Error("Global WithTunnel() function not working")
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Test environment variable detection functions
	// These are integration-style tests that may be environment dependent

	// Test NO_COLOR detection
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	if ShouldUseColor() {
		t.Error("ShouldUseColor() should return false when NO_COLOR is set")
	}
}
