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

// Package ui provides user interface components for the KubeLB CLI.
// It handles user-facing output, status messages, and interactive elements
// separately from diagnostic logging.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"k8c.io/kubelb/cli/internal/logger"
)

// Output manages user-facing output with support for different verbosity levels.
type Output struct {
	writer   io.Writer
	logger   *logger.Logger
	enableUI bool
}

// New creates a new Output instance.
func New() *Output {
	return &Output{
		writer:   os.Stdout,
		logger:   logger.Get(),
		enableUI: true,
	}
}

// NewWithWriter creates a new Output instance with a custom writer.
func NewWithWriter(w io.Writer) *Output {
	return &Output{
		writer:   w,
		logger:   logger.Get(),
		enableUI: true,
	}
}

// SetUIEnabled controls whether UI elements (progress, status) are shown.
func (o *Output) SetUIEnabled(enabled bool) {
	o.enableUI = enabled
}

// Status displays a status message to the user at verbosity level 1.
func (o *Output) Status(format string, args ...any) {
	if o.logger.ShouldLog(1) && o.enableUI {
		fmt.Fprintf(o.writer, format+"\n", args...)
	}
}

// StatusDetailed displays a detailed status message at verbosity level 2.
func (o *Output) StatusDetailed(format string, args ...any) {
	if o.logger.ShouldLog(2) && o.enableUI {
		fmt.Fprintf(o.writer, format+"\n", args...)
	}
}

// Success displays a success message with a checkmark.
func (o *Output) Success(format string, args ...any) {
	if o.logger.ShouldLog(1) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "✅ %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "✓ %s\n", msg)
		}
	}
}

// Error displays an error message to the user.
func (o *Output) Error(format string, args ...any) {
	if o.logger.ShouldLog(0) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "❌ %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "✗ %s\n", msg)
		}
	}
}

// Warning displays a warning message to the user.
func (o *Output) Warning(format string, args ...any) {
	if o.logger.ShouldLog(1) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "⚠️  %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "! %s\n", msg)
		}
	}
}

// Info displays an informational message.
func (o *Output) Info(format string, args ...any) {
	if o.logger.ShouldLog(1) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "ℹ️  %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "i %s\n", msg)
		}
	}
}

// Progress displays a progress message with a spinner or dots.
func (o *Output) Progress(format string, args ...any) {
	if o.logger.ShouldLog(1) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "🔄 %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "⋯ %s\n", msg)
		}
	}
}

// Connection displays connection-related messages.
func (o *Output) Connection(format string, args ...any) {
	if o.logger.ShouldLog(1) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "🔗 %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "→ %s\n", msg)
		}
	}
}

// Disconnection displays disconnection-related messages.
func (o *Output) Disconnection(format string, args ...any) {
	if o.logger.ShouldLog(1) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "💔 %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "✗ %s\n", msg)
		}
	}
}

// Health displays health-related messages.
func (o *Output) Health(format string, args ...any) {
	if o.logger.ShouldLog(2) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "🏓 %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "♡ %s\n", msg)
		}
	}
}

// Debug displays debug messages at higher verbosity levels.
func (o *Output) Debug(format string, args ...any) {
	if o.logger.ShouldLog(3) && o.enableUI {
		msg := fmt.Sprintf(format, args...)
		if shouldUseEmojis() {
			fmt.Fprintf(o.writer, "🔍 %s\n", msg)
		} else {
			fmt.Fprintf(o.writer, "» %s\n", msg)
		}
	}
}

// Header displays a section header.
func (o *Output) Header(title string) {
	if o.logger.ShouldLog(1) && o.enableUI {
		fmt.Fprintf(o.writer, "\n%s\n", title)
		if shouldUseColors() {
			fmt.Fprintf(o.writer, "\033[2m%s\033[0m\n", strings.Repeat("─", len(title)))
		} else {
			fmt.Fprintf(o.writer, "%s\n", strings.Repeat("-", len(title)))
		}
	}
}

// Table displays data in a simple table format.
func (o *Output) Table(headers []string, rows [][]string) {
	if !o.logger.ShouldLog(1) || !o.enableUI {
		return
	}

	if len(headers) == 0 || len(rows) == 0 {
		return
	}

	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print headers
	for i, header := range headers {
		fmt.Fprintf(o.writer, "%-*s", colWidths[i]+2, header)
	}
	fmt.Fprintf(o.writer, "\n")

	// Print separator
	for i := range headers {
		fmt.Fprintf(o.writer, "%s", strings.Repeat("-", colWidths[i]+2))
	}
	fmt.Fprintf(o.writer, "\n")

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				fmt.Fprintf(o.writer, "%-*s", colWidths[i]+2, cell)
			}
		}
		fmt.Fprintf(o.writer, "\n")
	}
}

// JSON outputs data in JSON format when requested.
func (o *Output) JSON(data any) error {
	// This would be handled by the logger in JSON format mode
	// For now, we delegate to structured logging
	o.logger.Info("json_output", "data", data)
	return nil
}

// PromptChoice presents options to the user and returns their selection.
// It displays a message, lists the options with keyboard shortcuts, and waits for user input.
// If timeout is reached, the defaultChoice is returned.
func (o *Output) PromptChoice(message string, options []string, defaultChoice string, timeout time.Duration) (string, error) {
	if !o.enableUI {
		return defaultChoice, nil
	}

	fmt.Fprintf(o.writer, "\n%s\n", message)

	// Display options
	for _, option := range options {
		fmt.Fprintf(o.writer, "  %s\n", option)
	}

	fmt.Fprintf(o.writer, "\nChoice (timeout in %.0fs): ", timeout.Seconds())

	// Channel to capture user input
	inputChan := make(chan string, 1)

	// Start goroutine to read user input
	go func() {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			inputChan <- defaultChoice
			return
		}
		inputChan <- strings.TrimSpace(strings.ToLower(input))
	}()

	// Wait for input or timeout
	select {
	case input := <-inputChan:
		if input == "" {
			return defaultChoice, nil
		}
		return input, nil
	case <-time.After(timeout):
		fmt.Fprintf(o.writer, "\nTimeout reached, using default: %s\n", defaultChoice)
		return defaultChoice, nil
	}
}

// shouldUseEmojis checks if emojis should be used in output.
func shouldUseEmojis() bool {
	if os.Getenv("NO_EMOJI") != "" {
		return false
	}
	if os.Getenv("KUBELB_EMOJI") == "false" {
		return false
	}
	// Default to true for interactive terminals
	return true
}

// shouldUseColors checks if colors should be used in output.
func shouldUseColors() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	// Default to true for interactive terminals
	return true
}

// Global convenience instance
var defaultOutput = New()

// Status displays a status message using the default output.
func Status(format string, args ...any) {
	defaultOutput.Status(format, args...)
}

// StatusDetailed displays a detailed status message using the default output.
func StatusDetailed(format string, args ...any) {
	defaultOutput.StatusDetailed(format, args...)
}

// Success displays a success message using the default output.
func Success(format string, args ...any) {
	defaultOutput.Success(format, args...)
}

// Error displays an error message using the default output.
func Error(format string, args ...any) {
	defaultOutput.Error(format, args...)
}

// Warning displays a warning message using the default output.
func Warning(format string, args ...any) {
	defaultOutput.Warning(format, args...)
}

// Info displays an informational message using the default output.
func Info(format string, args ...any) {
	defaultOutput.Info(format, args...)
}

// Progress displays a progress message using the default output.
func Progress(format string, args ...any) {
	defaultOutput.Progress(format, args...)
}

// Connection displays a connection message using the default output.
func Connection(format string, args ...any) {
	defaultOutput.Connection(format, args...)
}

// Disconnection displays a disconnection message using the default output.
func Disconnection(format string, args ...any) {
	defaultOutput.Disconnection(format, args...)
}

// Health displays a health message using the default output.
func Health(format string, args ...any) {
	defaultOutput.Health(format, args...)
}

// Debug displays a debug message using the default output.
func Debug(format string, args ...any) {
	defaultOutput.Debug(format, args...)
}

// Header displays a section header using the default output.
func Header(title string) {
	defaultOutput.Header(title)
}

// Table displays a table using the default output.
func Table(headers []string, rows [][]string) {
	defaultOutput.Table(headers, rows)
}

// PromptChoice presents options to the user and returns their selection
func PromptChoice(message string, options []string, defaultChoice string, timeout time.Duration) (string, error) {
	return defaultOutput.PromptChoice(message, options, defaultChoice, timeout)
}
