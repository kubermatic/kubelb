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
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// CLIHandler implements a slog.Handler that provides CLI-friendly output
// with colors, emojis, and human-readable formatting.
type CLIHandler struct {
	config *Config
	attrs  []slog.Attr
	groups []string
}

// NewCLIHandler creates a new CLI handler with the provided configuration.
func NewCLIHandler(config *Config) *CLIHandler {
	return &CLIHandler{
		config: config,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *CLIHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.config.Level.ToSlogLevel()
}

// Handle handles the Record.
func (h *CLIHandler) Handle(_ context.Context, r slog.Record) error {
	buf := make([]byte, 0, 1024)

	// Add timestamp for debug and trace levels
	if h.config.Level >= LevelDebug {
		buf = fmt.Appendf(buf, "%s ", formatTime(r.Time))
	}

	// Add level with color and emoji
	buf = append(buf, h.formatLevel(r.Level)...)
	buf = append(buf, ' ')

	// Add message
	buf = append(buf, r.Message...)

	// Add attributes
	if r.NumAttrs() > 0 || len(h.attrs) > 0 {
		buf = append(buf, ' ')
		buf = h.appendAttrs(buf, r)
	}

	// Add source information if enabled
	if h.config.AddSource && r.PC != 0 {
		buf = append(buf, ' ')
		buf = h.appendSource(buf, r.PC)
	}

	buf = append(buf, '\n')

	_, err := h.config.Output.Write(buf)
	return err
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments.
func (h *CLIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &CLIHandler{
		config: h.config,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups.
func (h *CLIHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &CLIHandler{
		config: h.config,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

// formatLevel formats the log level with appropriate colors and emojis.
func (h *CLIHandler) formatLevel(level slog.Level) string {
	var levelStr, emoji, color string

	switch {
	case level >= slog.LevelError:
		levelStr = "ERROR"
		emoji = "❌"
		color = colorRed
	case level >= slog.LevelWarn:
		levelStr = "WARN"
		emoji = "⚠️"
		color = colorYellow
	case level >= slog.LevelInfo:
		levelStr = "INFO"
		emoji = "ℹ️"
		color = colorBlue
	case level >= slog.LevelDebug:
		levelStr = "DEBUG"
		emoji = "🐛"
		color = colorMagenta
	default: // trace level
		levelStr = "TRACE"
		emoji = "🔍"
		color = colorCyan
	}

	// Build the level string
	var parts []string

	// Check environment variables for emoji/color preferences
	useEmoji := os.Getenv("NO_EMOJI") == "" && os.Getenv("KUBELB_EMOJI") != "false"
	useColor := os.Getenv("NO_COLOR") == "" && isTerminal(h.config.Output)

	if useEmoji {
		parts = append(parts, emoji)
	}

	if useColor {
		parts = append(parts, color+levelStr+colorReset)
	} else {
		parts = append(parts, levelStr)
	}

	return strings.Join(parts, " ")
}

// appendAttrs appends formatted attributes to the buffer.
func (h *CLIHandler) appendAttrs(buf []byte, r slog.Record) []byte {
	var attrs []slog.Attr

	// Add handler's persistent attributes
	attrs = append(attrs, h.attrs...)

	// Add record's attributes
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	if len(attrs) == 0 {
		return buf
	}

	// Format attributes
	var pairs []string
	for _, attr := range attrs {
		pairs = append(pairs, h.formatAttr(attr))
	}

	useColor := os.Getenv("NO_COLOR") == "" && isTerminal(h.config.Output)
	if useColor {
		buf = fmt.Appendf(buf, "%s[%s]%s", colorFaint, strings.Join(pairs, " "), colorReset)
	} else {
		buf = fmt.Appendf(buf, "[%s]", strings.Join(pairs, " "))
	}

	return buf
}

// formatAttr formats a single attribute.
func (h *CLIHandler) formatAttr(attr slog.Attr) string {
	key := attr.Key

	// Apply group prefix if any
	if len(h.groups) > 0 {
		key = strings.Join(h.groups, ".") + "." + key
	}

	return fmt.Sprintf("%s=%v", key, attr.Value)
}

// appendSource appends source information to the buffer.
func (h *CLIHandler) appendSource(buf []byte, pc uintptr) []byte {
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()

	if f.File != "" {
		// Get just the filename, not the full path
		parts := strings.Split(f.File, "/")
		filename := parts[len(parts)-1]

		useColor := os.Getenv("NO_COLOR") == "" && isTerminal(h.config.Output)
		if useColor {
			buf = fmt.Appendf(buf, "%s(%s:%d)%s", colorFaint, filename, f.Line, colorReset)
		} else {
			buf = fmt.Appendf(buf, "(%s:%d)", filename, f.Line)
		}
	}

	return buf
}

// formatTime formats a timestamp for display.
func formatTime(t time.Time) string {
	return t.Format("15:04:05.000")
}

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorFaint   = "\033[2m"
)

// Compile-time check that CLIHandler implements slog.Handler
var _ slog.Handler = (*CLIHandler)(nil)
