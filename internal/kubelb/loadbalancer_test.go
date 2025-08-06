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

package kubelb

import (
	"testing"
)

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{
			name:     "valid hostname",
			hostname: "app.example.com",
			want:     true,
		},
		{
			name:     "valid hostname with numbers",
			hostname: "app123.example456.com",
			want:     true,
		},
		{
			name:     "valid hostname with hyphens",
			hostname: "my-app.example-domain.com",
			want:     true,
		},
		{
			name:     "valid long subdomain",
			hostname: "very-long-subdomain-name-that-is-exactly-63-characters-long123.example.com",
			want:     true,
		},
		{
			name:     "empty hostname",
			hostname: "",
			want:     false,
		},
		{
			name:     "hostname too long",
			hostname: "this-is-a-very-long-hostname-that-exceeds-the-maximum-allowed-length-of-253-characters-in-total-including-all-labels-and-dots-between-them-which-makes-it-invalid-according-to-dns-standards-so-it-should-fail-validation-when-we-check-it-against-the-rules.com",
			want:     false,
		},
		{
			name:     "label too long",
			hostname: "this-label-is-way-too-long-and-exceeds-63-characters-which-is-not-allowed.example.com",
			want:     false,
		},
		{
			name:     "consecutive dots",
			hostname: "app..example.com",
			want:     false,
		},
		{
			name:     "leading dot",
			hostname: ".example.com",
			want:     false,
		},
		{
			name:     "trailing dot",
			hostname: "example.com.",
			want:     false,
		},
		{
			name:     "single label",
			hostname: "localhost",
			want:     false,
		},
		{
			name:     "label starting with hyphen",
			hostname: "-app.example.com",
			want:     false,
		},
		{
			name:     "label ending with hyphen",
			hostname: "app-.example.com",
			want:     false,
		},
		{
			name:     "special characters",
			hostname: "app@.example.com",
			want:     false,
		},
		{
			name:     "uppercase letters (should be valid as DNS is case insensitive)",
			hostname: "App.Example.COM",
			want:     true,
		},
		{
			name:     "wildcard domain",
			hostname: "*.example.com",
			want:     false, // wildcards should be stripped before validation
		},
		{
			name:     "hex prefix from generation",
			hostname: "a1b2c3d4e5f6a7b8.example.com",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidHostname(tt.hostname); got != tt.want {
				t.Errorf("isValidHostname(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}
