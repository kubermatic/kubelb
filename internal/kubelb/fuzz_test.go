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

func FuzzGenerateName(f *testing.F) {
	f.Add("my-service", "default")
	f.Add("", "")
	f.Add(strings.Repeat("a", 100), "tenant-xyz")
	f.Add("svc.with.dots", "ns_with_underscores")
	f.Add("üñïçödé-name", "ns")

	f.Fuzz(func(t *testing.T, name, namespace string) {
		output := GenerateName(name, namespace)
		if len(output) > MaxNameLength {
			t.Errorf("GenerateName(%q, %q) = %q, length %d exceeds %d", name, namespace, output, len(output), MaxNameLength)
		}
	})
}

func FuzzGenerateRouteServiceName(f *testing.F) {
	f.Add("route", "service", "default")
	f.Add("", "", "")
	f.Add(strings.Repeat("x", 200), "svc", "ns")
	f.Add("UPPER.Case", "-leading-dash", "9starts-with-digit")
	f.Add("üñïçödé", "\x00\xff", "ns")

	f.Fuzz(func(t *testing.T, routeName, serviceName, namespace string) {
		output := GenerateRouteServiceName(routeName, serviceName, namespace)
		if errs := validation.IsDNS1035Label(output); len(errs) > 0 {
			t.Errorf("GenerateRouteServiceName(%q, %q, %q) = %q, not a valid DNS-1035 label: %v", routeName, serviceName, namespace, output, errs)
		}
		if len(output) > MaxNameLength {
			t.Errorf("GenerateRouteServiceName(%q, %q, %q) = %q, length %d exceeds %d", routeName, serviceName, namespace, output, len(output), MaxNameLength)
		}
	})
}

func FuzzIsValidHostname(f *testing.F) {
	f.Add("example.com")
	f.Add("sub.example.com")
	f.Add("single-label")
	f.Add("a..b")
	f.Add(".leading.dot")
	f.Add("trailing.dot.")
	f.Add(strings.Repeat("a", 63) + ".com")
	f.Add(strings.Repeat("a.", 127) + "a")
	f.Add("üñïçödé.com")

	f.Fuzz(func(t *testing.T, hostname string) {
		if isValidHostname(hostname) {
			if errs := validation.IsDNS1123Subdomain(strings.ToLower(hostname)); len(errs) > 0 {
				t.Errorf("isValidHostname(%q) = true, but not a valid DNS-1123 subdomain: %v", hostname, errs)
			}
		}
	})
}

func FuzzMatchPattern(f *testing.F) {
	f.Add("*.example.com", "foo.example.com")
	f.Add("[invalid", "key")
	f.Add("", "")
	f.Add("a[b-", "a[b-")

	f.Fuzz(func(t *testing.T, pattern, key string) {
		matchPattern(pattern, key)
	})
}
