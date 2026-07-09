/*
Copyright 2026 The KubeLB Authors.

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

package ingress

import (
	"testing"
)

func TestParseIngressName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		defaultNS   string
		wantNS      string
		wantIngName string
	}{
		{
			name:        "simple name uses default namespace",
			input:       "my-app",
			defaultNS:   "default",
			wantNS:      "default",
			wantIngName: "my-app",
		},
		{
			name:        "namespace/name format",
			input:       "prod/my-app",
			defaultNS:   "default",
			wantNS:      "prod",
			wantIngName: "my-app",
		},
		{
			name:        "empty default namespace",
			input:       "my-app",
			defaultNS:   "",
			wantNS:      "",
			wantIngName: "my-app",
		},
		{
			name:        "namespace/name overrides default",
			input:       "staging/app",
			defaultNS:   "production",
			wantNS:      "staging",
			wantIngName: "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNS, gotName := parseIngressName(tt.input, tt.defaultNS)
			if gotNS != tt.wantNS {
				t.Errorf("parseIngressName() namespace = %v, want %v", gotNS, tt.wantNS)
			}
			if gotName != tt.wantIngName {
				t.Errorf("parseIngressName() name = %v, want %v", gotName, tt.wantIngName)
			}
		})
	}
}

// Note: Tests for ShouldSkip, MatchesIngressClass, and DetermineConversionStatus
// are now in the kubelb package (pkg/conversion/) as the logic was moved there.
