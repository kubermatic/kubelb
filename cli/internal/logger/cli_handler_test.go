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
	"strings"
	"testing"
	"time"
)

func TestCLIHandler_Enabled(t *testing.T) {
	tests := []struct {
		name         string
		logLevel     Level
		recordLevel  slog.Level
		shouldEnable bool
	}{
		{"error level enables error", LevelError, slog.LevelError, true},
		{"error level disables warn", LevelError, slog.LevelWarn, false},
		{"info level enables info", LevelInfo, slog.LevelInfo, true},
		{"info level disables debug", LevelInfo, slog.LevelDebug, false},
		{"debug level enables debug", LevelDebug, slog.LevelDebug, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Level:  tt.logLevel,
				Output: &bytes.Buffer{},
			}
			handler := NewCLIHandler(config)

			enabled := handler.Enabled(context.Background(), tt.recordLevel)
			if enabled != tt.shouldEnable {
				t.Errorf("Enabled() = %v, want %v", enabled, tt.shouldEnable)
			}
		})
	}
}

func TestCLIHandler_Handle(t *testing.T) {
	tests := []struct {
		name              string
		level             slog.Level
		message           string
		addSource         bool
		expectContains    []string
		expectNotContains []string
	}{
		{
			name:           "error level",
			level:          slog.LevelError,
			message:        "test error",
			expectContains: []string{"ERROR", "test error"},
		},
		{
			name:           "info level",
			level:          slog.LevelInfo,
			message:        "test info",
			expectContains: []string{"INFO", "test info"},
		},
		{
			name:           "debug with source",
			level:          slog.LevelDebug,
			message:        "test debug",
			addSource:      true,
			expectContains: []string{"DEBUG", "test debug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := &Config{
				Level:     LevelTrace, // Allow all levels
				Output:    &buf,
				AddSource: tt.addSource,
			}
			handler := NewCLIHandler(config)

			record := slog.NewRecord(time.Now(), tt.level, tt.message, 0)

			err := handler.Handle(context.Background(), record)
			if err != nil {
				t.Errorf("Handle() error = %v", err)
				return
			}

			output := buf.String()

			for _, expect := range tt.expectContains {
				if !strings.Contains(output, expect) {
					t.Errorf("Output should contain %q, got: %s", expect, output)
				}
			}

			for _, notExpect := range tt.expectNotContains {
				if strings.Contains(output, notExpect) {
					t.Errorf("Output should not contain %q, got: %s", notExpect, output)
				}
			}
		})
	}
}

func TestCLIHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	handler := NewCLIHandler(config)
	handlerWithAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("key1", "value1"),
		slog.String("key2", "value2"),
	})

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	err := handlerWithAttrs.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Error("Output should contain key1=value1")
	}
	if !strings.Contains(output, "key2=value2") {
		t.Error("Output should contain key2=value2")
	}
}

func TestCLIHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	handler := NewCLIHandler(config)
	handlerWithGroup := handler.WithGroup("testgroup")
	handlerWithAttrs := handlerWithGroup.WithAttrs([]slog.Attr{
		slog.String("key", "value"),
	})

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	err := handlerWithAttrs.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "testgroup.key=value") {
		t.Errorf("Output should contain grouped attribute, got: %s", output)
	}
}

func TestCLIHandler_RecordAttrs(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	handler := NewCLIHandler(config)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.Add("request_id", "123")
	record.Add("method", "GET")

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "request_id=123") {
		t.Error("Output should contain request_id=123")
	}
	if !strings.Contains(output, "method=GET") {
		t.Error("Output should contain method=GET")
	}
}

func TestFormatLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    slog.Level
		contains []string
	}{
		{
			name:     "error level",
			level:    slog.LevelError,
			contains: []string{"ERROR"},
		},
		{
			name:     "warn level",
			level:    slog.LevelWarn,
			contains: []string{"WARN"},
		},
		{
			name:     "info level",
			level:    slog.LevelInfo,
			contains: []string{"INFO"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := &Config{
				Output: &buf,
			}
			handler := &CLIHandler{config: config}

			result := handler.formatLevel(tt.level)

			for _, expect := range tt.contains {
				if !strings.Contains(result, expect) {
					t.Errorf("formatLevel should contain %q, got: %s", expect, result)
				}
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	testTime := time.Date(2023, 12, 25, 15, 30, 45, 123456789, time.UTC)
	result := formatTime(testTime)

	expected := "15:30:45.123"
	if result != expected {
		t.Errorf("formatTime() = %s, want %s", result, expected)
	}
}

func BenchmarkCLIHandler_Handle(b *testing.B) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}
	handler := NewCLIHandler(config)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "benchmark message", 0)
	record.Add("key1", "value1")
	record.Add("key2", "value2")
	record.Add("key3", 123)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = handler.Handle(context.Background(), record)
	}
}
