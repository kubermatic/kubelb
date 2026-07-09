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
	"os"
)

// isatty checks if a file descriptor refers to a terminal.
// This is a simplified implementation that works across platforms.
func isatty(fd uintptr) bool {
	// Simple heuristic: check if it's one of the standard file descriptors
	// and if the associated file has a name (pipes/redirects typically don't)
	switch fd {
	case os.Stdin.Fd():
		stat, err := os.Stdin.Stat()
		return err == nil && (stat.Mode()&os.ModeCharDevice) != 0
	case os.Stdout.Fd():
		stat, err := os.Stdout.Stat()
		return err == nil && (stat.Mode()&os.ModeCharDevice) != 0
	case os.Stderr.Fd():
		stat, err := os.Stderr.Stat()
		return err == nil && (stat.Mode()&os.ModeCharDevice) != 0
	default:
		return false
	}
}

// ShouldUseColor determines if color output should be used.
func ShouldUseColor() bool {
	// Respect NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if stdout is a terminal
	return isTerminal(os.Stdout)
}

// ShouldUseEmoji determines if emoji output should be used.
func ShouldUseEmoji() bool {
	// Disable emojis in CI environments
	if isCIEnvironment() {
		return false
	}

	// Check for explicit emoji preference
	if emojiPref := os.Getenv("KUBELB_EMOJI"); emojiPref != "" {
		return emojiPref == "1" || emojiPref == "true"
	}

	return isTerminal(os.Stdout)
}

// isCIEnvironment checks if we're running in a known CI environment.
func isCIEnvironment() bool {
	ciVars := []string{
		"CI", "CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS", "GITLAB_CI", "TRAVIS",
		"CIRCLECI", "JENKINS_URL", "BUILDKITE",
	}

	for _, env := range ciVars {
		if os.Getenv(env) != "" {
			return true
		}
	}

	return false
}
