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

package ingressconversion

import (
	"testing"
	"time"

	"k8c.io/kubelb/pkg/conversion"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestIsAlreadyConverted(t *testing.T) {
	tests := []struct {
		name    string
		ingress *networkingv1.Ingress
		want    bool
	}{
		{
			name:    "nil annotations",
			ingress: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			want:    false,
		},
		{
			name: "no conversion status",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{"other": "annotation"},
				},
			},
			want: false,
		},
		{
			name: "status converted",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{conversion.AnnotationConversionStatus: conversion.ConversionStatusConverted},
				},
			},
			want: true,
		},
		{
			name: "status partial",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{conversion.AnnotationConversionStatus: conversion.ConversionStatusPartial},
				},
			},
			want: true,
		},
		{
			name: "status pending",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{conversion.AnnotationConversionStatus: conversion.ConversionStatusPending},
				},
			},
			want: false,
		},
		{
			name: "status skipped",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{conversion.AnnotationConversionStatus: conversion.ConversionStatusSkipped},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAlreadyConverted(tt.ingress)
			if got != tt.want {
				t.Errorf("isAlreadyConverted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcilerDisableEnvoyGatewayFeatures(t *testing.T) {
	tests := []struct {
		name     string
		disabled bool
	}{
		{
			name:     "features enabled",
			disabled: false,
		},
		{
			name:     "features disabled",
			disabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				DisableEnvoyGatewayFeatures: tt.disabled,
			}

			if r.DisableEnvoyGatewayFeatures != tt.disabled {
				t.Errorf("DisableEnvoyGatewayFeatures = %v, want %v", r.DisableEnvoyGatewayFeatures, tt.disabled)
			}
		})
	}
}

func TestShouldConvert(t *testing.T) {
	tests := []struct {
		name         string
		ingress      *networkingv1.Ingress
		ingressClass string
		want         bool
	}{
		{
			name: "basic ingress, no filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			ingressClass: "",
			want:         true,
		},
		{
			name: "skip-conversion annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.AnnotationSkipConversion: "true",
					},
				},
			},
			ingressClass: "",
			want:         false,
		},
		{
			name: "canary ingress",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.NginxCanary: "true",
					},
				},
			},
			ingressClass: "",
			want:         false,
		},
		{
			name: "spec.ingressClassName matches filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
				},
			},
			ingressClass: "nginx",
			want:         true,
		},
		{
			name: "spec.ingressClassName does not match filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("traefik"),
				},
			},
			ingressClass: "nginx",
			want:         false,
		},
		{
			name: "annotation ingressClass matches filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.AnnotationIngressClass: "nginx",
					},
				},
			},
			ingressClass: "nginx",
			want:         true,
		},
		{
			name: "annotation ingressClass does not match filter",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.AnnotationIngressClass: "traefik",
					},
				},
			},
			ingressClass: "nginx",
			want:         false,
		},
		{
			name: "no class specified, filter set",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			ingressClass: "nginx",
			want:         false,
		},
		{
			name: "spec.ingressClassName takes precedence over annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.AnnotationIngressClass: "traefik",
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
				},
			},
			ingressClass: "nginx",
			want:         true,
		},
		{
			name: "already converted - status converted",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.AnnotationConversionStatus: conversion.ConversionStatusConverted,
					},
				},
			},
			ingressClass: "",
			want:         false,
		},
		{
			name: "already converted - status partial",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.AnnotationConversionStatus: conversion.ConversionStatusPartial,
					},
				},
			},
			ingressClass: "",
			want:         false,
		},
		{
			name: "pending status should reconvert",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						conversion.AnnotationConversionStatus: conversion.ConversionStatusPending,
					},
				},
			},
			ingressClass: "",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				IngressClass: tt.ingressClass,
			}
			got := r.shouldConvert(tt.ingress)
			if got.shouldConvert != tt.want {
				t.Errorf("shouldConvert().shouldConvert = %v, want %v", got.shouldConvert, tt.want)
			}
		})
	}
}

func TestDetermineConversionStatus(t *testing.T) {
	r := &Reconciler{}

	tests := []struct {
		name            string
		annotations     map[string]string
		routeAcceptance map[string]bool
		warnings        []string
		hasTimeout      bool
		wantStatus      string
		wantRequeue     bool
		wantClearVerify bool
	}{
		{
			name:            "timeout always returns pending and requeues",
			annotations:     map[string]string{},
			routeAcceptance: map[string]bool{"route1": true},
			warnings:        nil,
			hasTimeout:      true,
			wantStatus:      conversion.ConversionStatusPending,
			wantRequeue:     true,
			wantClearVerify: true,
		},
		{
			name:            "route not accepted returns partial",
			annotations:     map[string]string{},
			routeAcceptance: map[string]bool{"route1": false},
			warnings:        nil,
			hasTimeout:      false,
			wantStatus:      conversion.ConversionStatusPartial,
			wantRequeue:     false,
			wantClearVerify: true,
		},
		{
			name:            "first verification - no timestamp, returns pending",
			annotations:     map[string]string{},
			routeAcceptance: map[string]bool{"route1": true},
			warnings:        nil,
			hasTimeout:      false,
			wantStatus:      conversion.ConversionStatusPending,
			wantRequeue:     true,
			wantClearVerify: false,
		},
		{
			name: "second verification - timestamp too recent, returns pending",
			annotations: map[string]string{
				conversion.AnnotationVerificationTimestamp: time.Now().Format(time.RFC3339),
			},
			routeAcceptance: map[string]bool{"route1": true},
			warnings:        nil,
			hasTimeout:      false,
			wantStatus:      conversion.ConversionStatusPending,
			wantRequeue:     true,
			wantClearVerify: false,
		},
		{
			name: "second verification - timestamp old enough, no warnings - converted",
			annotations: map[string]string{
				conversion.AnnotationVerificationTimestamp: time.Now().Add(-6 * time.Second).Format(time.RFC3339),
			},
			routeAcceptance: map[string]bool{"route1": true},
			warnings:        nil,
			hasTimeout:      false,
			wantStatus:      conversion.ConversionStatusConverted,
			wantRequeue:     false,
			wantClearVerify: true,
		},
		{
			name: "second verification - timestamp old enough, has warnings - partial",
			annotations: map[string]string{
				conversion.AnnotationVerificationTimestamp: time.Now().Add(-6 * time.Second).Format(time.RFC3339),
			},
			routeAcceptance: map[string]bool{"route1": true},
			warnings:        []string{"some warning"},
			hasTimeout:      false,
			wantStatus:      conversion.ConversionStatusPartial,
			wantRequeue:     false,
			wantClearVerify: true,
		},
		{
			name: "invalid timestamp - treat as first pass",
			annotations: map[string]string{
				conversion.AnnotationVerificationTimestamp: "invalid-timestamp",
			},
			routeAcceptance: map[string]bool{"route1": true},
			warnings:        nil,
			hasTimeout:      false,
			wantStatus:      conversion.ConversionStatusPending,
			wantRequeue:     true,
			wantClearVerify: false,
		},
		{
			name:            "multiple routes - one not accepted returns partial",
			annotations:     map[string]string{},
			routeAcceptance: map[string]bool{"route1": true, "route2": false},
			warnings:        nil,
			hasTimeout:      false,
			wantStatus:      conversion.ConversionStatusPartial,
			wantRequeue:     false,
			wantClearVerify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: tt.annotations,
				},
			}

			got := r.determineConversionStatus(ingress, tt.routeAcceptance, tt.warnings, tt.hasTimeout)

			if got.status != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.status, tt.wantStatus)
			}
			if got.requeue != tt.wantRequeue {
				t.Errorf("requeue = %v, want %v", got.requeue, tt.wantRequeue)
			}
			if got.clearVerifyTime != tt.wantClearVerify {
				t.Errorf("clearVerifyTime = %v, want %v", got.clearVerifyTime, tt.wantClearVerify)
			}
		})
	}
}
