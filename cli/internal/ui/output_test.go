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

package ui

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"k8c.io/kubelb/cli/internal/logger"
)

func TestNew(t *testing.T) {
	output := New()
	if output == nil {
		t.Fatal("New() returned nil")
	}
	if output.writer == nil {
		t.Error("New() writer is nil")
	}
	if !output.enableUI {
		t.Error("New() should enable UI by default")
	}
}

func TestNewWithWriter(t *testing.T) {
	var buf bytes.Buffer
	output := NewWithWriter(&buf)

	if output.writer != &buf {
		t.Error("NewWithWriter() should use provided writer")
	}
}

func TestOutput_SetUIEnabled(t *testing.T) {
	output := New()
	output.SetUIEnabled(false)

	if output.enableUI {
		t.Error("SetUIEnabled(false) should disable UI")
	}
}

func TestOutput_Status(t *testing.T) {
	var buf bytes.Buffer

	// Setup logger with verbosity level 1 to allow status messages
	config := logger.DefaultConfig()
	config.VerbosityLevel = 1
	config.Output = &buf // Capture logger output too
	logger.Setup(config)

	output := NewWithWriter(&buf)
	output.Status("test status %s", "message")

	result := buf.String()
	if !strings.Contains(result, "test status message") {
		t.Errorf("Status message not found in output: %s", result)
	}
}

func TestOutput_StatusDetailed(t *testing.T) {
	var buf bytes.Buffer

	// Setup logger with verbosity level 2 to allow detailed status messages
	config := logger.DefaultConfig()
	config.VerbosityLevel = 2
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)
	output.StatusDetailed("detailed status %d", 123)

	result := buf.String()
	if !strings.Contains(result, "detailed status 123") {
		t.Errorf("Detailed status message not found in output: %s", result)
	}
}

func TestOutput_Success(t *testing.T) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 1
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)

	// Test with emojis (default)
	os.Unsetenv("NO_EMOJI")
	output.Success("operation completed")

	result := buf.String()
	if !strings.Contains(result, "✅") || !strings.Contains(result, "operation completed") {
		t.Errorf("Success message should contain emoji and text: %s", result)
	}
}

func TestOutput_Error(t *testing.T) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 0 // Error messages at verbosity 0
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)
	output.Error("something failed: %s", "reason")

	result := buf.String()
	if !strings.Contains(result, "something failed: reason") {
		t.Errorf("Error message not found in output: %s", result)
	}
}

func TestOutput_Warning(t *testing.T) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 1
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)
	output.Warning("potential issue detected")

	result := buf.String()
	if !strings.Contains(result, "potential issue detected") {
		t.Errorf("Warning message not found in output: %s", result)
	}
}

func TestOutput_VerbosityLevels(t *testing.T) {
	tests := []struct {
		name              string
		verbosityLevel    int
		method            func(*Output)
		shouldShowMessage bool
	}{
		{"status at level 0", 0, func(o *Output) { o.Status("test") }, false},
		{"status at level 1", 1, func(o *Output) { o.Status("test") }, true},
		{"detailed at level 1", 1, func(o *Output) { o.StatusDetailed("test") }, false},
		{"detailed at level 2", 2, func(o *Output) { o.StatusDetailed("test") }, true},
		{"debug at level 2", 2, func(o *Output) { o.Debug("test") }, false},
		{"debug at level 3", 3, func(o *Output) { o.Debug("test") }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			config := logger.DefaultConfig()
			config.VerbosityLevel = tt.verbosityLevel
			config.Output = &buf
			logger.Setup(config)

			output := NewWithWriter(&buf)
			tt.method(output)

			result := buf.String()
			hasMessage := strings.Contains(result, "test")

			if hasMessage != tt.shouldShowMessage {
				t.Errorf("At verbosity %d, message visibility = %v, want %v. Output: %s",
					tt.verbosityLevel, hasMessage, tt.shouldShowMessage, result)
			}
		})
	}
}

func TestOutput_UIDisabled(t *testing.T) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 4 // High verbosity
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)
	output.SetUIEnabled(false)

	// All these should be suppressed when UI is disabled
	output.Status("test status")
	output.Success("test success")
	output.Error("test error")
	output.Info("test info")

	result := buf.String()
	if strings.Contains(result, "test") {
		t.Errorf("UI disabled should suppress all messages, but got: %s", result)
	}
}

func TestOutput_Table(t *testing.T) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 1
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)

	headers := []string{"Name", "Status", "Port"}
	rows := [][]string{
		{"tunnel1", "ready", "8080"},
		{"tunnel2", "pending", "3000"},
	}

	output.Table(headers, rows)

	result := buf.String()
	expectedContents := []string{"Name", "Status", "Port", "tunnel1", "ready", "8080", "tunnel2", "pending", "3000"}

	for _, expected := range expectedContents {
		if !strings.Contains(result, expected) {
			t.Errorf("Table should contain %q, got: %s", expected, result)
		}
	}
}

func TestOutput_Header(t *testing.T) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 1
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)
	output.Header("Test Section")

	result := buf.String()
	if !strings.Contains(result, "Test Section") {
		t.Errorf("Header should contain title: %s", result)
	}
	// Should contain some form of separator
	if !strings.Contains(result, "-") && !strings.Contains(result, "─") {
		t.Errorf("Header should contain separator: %s", result)
	}
}

func TestGlobalFunctions(t *testing.T) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 4 // High verbosity for all messages
	config.Output = &buf
	logger.Setup(config)

	// Redirect default output to our buffer
	oldDefault := defaultOutput
	defaultOutput = NewWithWriter(&buf)
	defer func() { defaultOutput = oldDefault }()

	// Test global functions
	Status("test status")
	Success("test success")
	Error("test error")
	Warning("test warning")
	Info("test info")
	Progress("test progress")
	Debug("test debug")

	result := buf.String()

	expectedMessages := []string{
		"test status", "test success", "test error",
		"test warning", "test info", "test progress", "test debug",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(result, expected) {
			t.Errorf("Global function output should contain %q, got: %s", expected, result)
		}
	}
}

func TestShouldUseEmojis(t *testing.T) {
	// Test NO_EMOJI environment variable
	os.Setenv("NO_EMOJI", "1")
	defer os.Unsetenv("NO_EMOJI")

	if shouldUseEmojis() {
		t.Error("shouldUseEmojis() should return false when NO_EMOJI is set")
	}

	os.Unsetenv("NO_EMOJI")

	// Test KUBELB_EMOJI=false
	os.Setenv("KUBELB_EMOJI", "false")
	defer os.Unsetenv("KUBELB_EMOJI")

	if shouldUseEmojis() {
		t.Error("shouldUseEmojis() should return false when KUBELB_EMOJI=false")
	}
}

func TestShouldUseColors(t *testing.T) {
	// Test NO_COLOR environment variable
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	if shouldUseColors() {
		t.Error("shouldUseColors() should return false when NO_COLOR is set")
	}
}

func BenchmarkOutput_Status(b *testing.B) {
	var buf bytes.Buffer

	config := logger.DefaultConfig()
	config.VerbosityLevel = 1
	config.Output = &buf
	logger.Setup(config)

	output := NewWithWriter(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		output.Status("benchmark message %d", i)
	}
}
