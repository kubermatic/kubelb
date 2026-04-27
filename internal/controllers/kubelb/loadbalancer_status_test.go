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
	"sort"
	"testing"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TestBuildServicePortStatus_SortedVsUnsortedNames is the regression test for
// the multi-port index-mismatch bug. Service ports are sorted alphabetically
// by Name (CreateServicePorts), but LoadBalancer endpoint ports are not.
// Pairing by index scrambled UpstreamTargetPort against Name, breaking
// LoadState match on manager restart.
func TestBuildServicePortStatus_SortedVsUnsortedNames(t *testing.T) {
	svcPorts := []corev1.ServicePort{
		{Name: "a", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(10001)},
		{Name: "b", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(10002)},
		{Name: "c", Port: 8080, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(10003)},
	}
	endpointPorts := []kubelbv1alpha1.EndpointPort{
		{Name: "c", Port: 30003, Protocol: corev1.ProtocolTCP},
		{Name: "a", Port: 30001, Protocol: corev1.ProtocolTCP},
		{Name: "b", Port: 30002, Protocol: corev1.ProtocolTCP},
	}

	out := buildServicePortStatus(logr.Discard(), svcPorts, endpointPorts)

	if got, want := len(out), len(svcPorts); got != want {
		t.Fatalf("got %d entries, want %d", got, want)
	}
	wantUpstream := map[string]int32{"a": 30001, "b": 30002, "c": 30003}
	for _, p := range out {
		if p.UpstreamTargetPort != wantUpstream[p.Name] {
			t.Errorf("port %q: UpstreamTargetPort=%d, want %d", p.Name, p.UpstreamTargetPort, wantUpstream[p.Name])
		}
	}
}

func TestBuildServicePortStatus_AlreadySorted(t *testing.T) {
	svcPorts := []corev1.ServicePort{
		{Name: "a", Port: 80, Protocol: corev1.ProtocolTCP},
		{Name: "b", Port: 443, Protocol: corev1.ProtocolTCP},
	}
	endpointPorts := []kubelbv1alpha1.EndpointPort{
		{Name: "a", Port: 30001, Protocol: corev1.ProtocolTCP},
		{Name: "b", Port: 30002, Protocol: corev1.ProtocolTCP},
	}

	out := buildServicePortStatus(logr.Discard(), svcPorts, endpointPorts)
	if out[0].UpstreamTargetPort != 30001 || out[1].UpstreamTargetPort != 30002 {
		t.Fatalf("unexpected upstream ports: %+v", out)
	}
}

func TestBuildServicePortStatus_UnnamedMatches(t *testing.T) {
	svcPorts := []corev1.ServicePort{
		{Port: 80, Protocol: corev1.ProtocolTCP},
	}
	endpointPorts := []kubelbv1alpha1.EndpointPort{
		{Port: 30001, Protocol: corev1.ProtocolTCP},
	}

	out := buildServicePortStatus(logr.Discard(), svcPorts, endpointPorts)
	if len(out) != 1 || out[0].UpstreamTargetPort != 30001 {
		t.Fatalf("single-port case: %+v", out)
	}
}

// TestBuildServicePortStatus_MismatchedNamesYieldZero verifies the safety
// property: when names don't align, UpstreamTargetPort stays zero rather than
// silently pairing with a random endpoint port.
func TestBuildServicePortStatus_MismatchedNamesYieldZero(t *testing.T) {
	svcPorts := []corev1.ServicePort{
		{Name: "only-in-service", Port: 80, Protocol: corev1.ProtocolTCP},
	}
	endpointPorts := []kubelbv1alpha1.EndpointPort{
		{Name: "only-in-endpoint", Port: 30001, Protocol: corev1.ProtocolTCP},
	}

	out := buildServicePortStatus(logr.Discard(), svcPorts, endpointPorts)
	if len(out) != 1 || out[0].UpstreamTargetPort != 0 {
		t.Fatalf("expected UpstreamTargetPort=0 on name mismatch, got %+v", out)
	}
}

// TestBuildServicePortStatus_DistinctProtocolsSameName covers the case where
// TCP and UDP ports share a Name. Match must include Protocol.
func TestBuildServicePortStatus_DistinctProtocolsSameName(t *testing.T) {
	svcPorts := []corev1.ServicePort{
		{Name: "dns", Port: 53, Protocol: corev1.ProtocolTCP},
		{Name: "dns", Port: 53, Protocol: corev1.ProtocolUDP},
	}
	endpointPorts := []kubelbv1alpha1.EndpointPort{
		{Name: "dns", Port: 30053, Protocol: corev1.ProtocolUDP},
		{Name: "dns", Port: 31053, Protocol: corev1.ProtocolTCP},
	}

	out := buildServicePortStatus(logr.Discard(), svcPorts, endpointPorts)
	sort.Slice(out, func(i, j int) bool { return out[i].Protocol < out[j].Protocol })
	if out[0].Protocol != corev1.ProtocolTCP || out[0].UpstreamTargetPort != 31053 {
		t.Errorf("TCP dns: got %+v", out[0])
	}
	if out[1].Protocol != corev1.ProtocolUDP || out[1].UpstreamTargetPort != 30053 {
		t.Errorf("UDP dns: got %+v", out[1])
	}
}
