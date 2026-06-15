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

package kubelb

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

func TestGenerateRouteServiceNameReturnsDNS1035Name(t *testing.T) {
	tests := []struct {
		name        string
		routeName   string
		serviceName string
		namespace   string
		want        string
	}{
		{
			name:        "short name is unchanged",
			routeName:   "route",
			serviceName: "service",
			namespace:   "default",
			want:        "default-route-service",
		},
		{
			name:        "route dots are normalized",
			routeName:   "app.example.com",
			serviceName: "service",
			namespace:   "default",
			want:        "default-app-example-com-service",
		},
		{
			name:        "uppercase chars are lowercased",
			routeName:   "Route",
			serviceName: "Service",
			namespace:   "Default",
			want:        "default-route-service",
		},
		{
			name:        "invalid separators are normalized",
			routeName:   "route/name",
			serviceName: "service_name",
			namespace:   "default",
			want:        "default-route-name-service-name",
		},
		{
			name:        "digit-leading namespace is prefixed",
			routeName:   "route",
			serviceName: "service",
			namespace:   "1default",
			want:        "k-1default-route-service",
		},
		{
			name:        "trailing invalid chars are trimmed",
			routeName:   "route.",
			serviceName: "service.",
			namespace:   "default.",
			want:        "default--route--service",
		},
		{
			name:        "all invalid chars fall back to valid name",
			routeName:   "...",
			serviceName: "___",
			namespace:   "///",
			want:        "k",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateRouteServiceName(tt.routeName, tt.serviceName, tt.namespace)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
			if errs := validation.IsDNS1035Label(got); len(errs) > 0 {
				t.Fatalf("expected DNS-1035 service name, got %q: %v", got, errs)
			}
		})
	}
}

func TestGenerateRouteServiceNameTruncatesLongName(t *testing.T) {
	got := GenerateRouteServiceName(strings.Repeat("a", 52), "bbbbbbbb", "a")

	if len(got) > MaxNameLength {
		t.Fatalf("expected name length <= %d, got %d: %q", MaxNameLength, len(got), got)
	}
	if errs := validation.IsDNS1035Label(got); len(errs) > 0 {
		t.Fatalf("expected DNS-1035 service name, got %q: %v", got, errs)
	}
}

func TestGenerateRouteServiceNameTruncatesIssue470Name(t *testing.T) {
	const invalidName = "argocd-default-xx-yyyy-argocd-server-default-xx-yyyy-argocd-"

	got := GenerateRouteServiceName(
		"default-xx-yyyy-argocd-server",
		"default-xx-yyyy-argocd-server",
		"argocd",
	)

	if got == invalidName {
		t.Fatalf("expected name not to match reported invalid name %q", invalidName)
	}
	if len(got) > MaxNameLength {
		t.Fatalf("expected name length <= %d, got %d: %q", MaxNameLength, len(got), got)
	}
	if errs := validation.IsDNS1035Label(got); len(errs) > 0 {
		t.Fatalf("expected DNS-1035 service name, got %q: %v", got, errs)
	}
}
