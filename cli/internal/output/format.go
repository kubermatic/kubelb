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

package output

import (
	"fmt"
	"os"
	"strings"
)

// ANSI color codes
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Underline  = "\033[4m"
	Blue       = "\033[34m"
	BrightBlue = "\033[94m"
	Cyan       = "\033[36m"
)

// isTerminalSupported checks if the current terminal supports ANSI colors
func isTerminalSupported() bool {
	term := os.Getenv("TERM")
	if term == "" {
		return false
	}

	// Check for common terminals that support ANSI colors
	supportedTerms := []string{
		"xterm", "screen", "tmux", "color", "ansi",
	}

	for _, supportedTerm := range supportedTerms {
		if strings.Contains(term, supportedTerm) {
			return true
		}
	}

	// Check if NO_COLOR environment variable is set
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	return true
}

// FormatURL formats a URL to be clickable and highlighted in the terminal
func FormatURL(url string) string {
	if !isTerminalSupported() {
		return url
	}
	clickableURL := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, url)
	styledURL := fmt.Sprintf("%s%s%s%s%s", BrightBlue, Bold, clickableURL, Reset, Reset)
	return styledURL
}

// FormatConnectionInfo formats the connection info with highlighted URL
func FormatConnectionInfo(url, localPort string) string {
	formattedURL := FormatURL(url)
	return fmt.Sprintf("   🌐 %s → localhost:%s", formattedURL, localPort)
}

// FormatPublicURL formats a public URL announcement
func FormatPublicURL(url string) string {
	formattedURL := FormatURL(url)
	return fmt.Sprintf("   🌐 Public URL: %s", formattedURL)
}
