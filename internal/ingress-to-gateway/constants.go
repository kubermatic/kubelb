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

const (
	ControllerName = "ingress-conversion-controller"

	// Common string constants
	boolTrue = "true"

	// Annotations
	AnnotationSkipConversion     = "kubelb.k8c.io/skip-conversion"
	AnnotationConversionStatus   = "kubelb.k8c.io/conversion-status"
	AnnotationConvertedHTTPRoute = "kubelb.k8c.io/converted-httproute"
	AnnotationConvertedGRPCRoute = "kubelb.k8c.io/converted-grpcroute"
	AnnotationConversionWarnings = "kubelb.k8c.io/conversion-warnings"

	// Legacy ingress class annotation (deprecated but still widely used)
	AnnotationIngressClass = "kubernetes.io/ingress.class"

	// Status values
	ConversionStatusConverted = "converted"
	ConversionStatusPartial   = "partial"
	ConversionStatusPending   = "pending" // Routes created, awaiting verification
	ConversionStatusFailed    = "failed"
	ConversionStatusSkipped   = "skipped"

	// Verification annotation - tracks when routes were first verified
	// Used to require a second verification pass before marking as converted
	AnnotationVerificationTimestamp = "kubelb.k8c.io/verification-timestamp"

	// Skip reason annotation
	AnnotationConversionSkipReason = "kubelb.k8c.io/conversion-skip-reason"

	// Skip reasons
	SkipReasonCanary = "canary annotations are not supported"

	// NGINX annotations (for skip logic and protocol detection)
	NginxCanary          = "nginx.ingress.kubernetes.io/canary"
	NginxBackendProtocol = "nginx.ingress.kubernetes.io/backend-protocol"

	// external-dns annotations
	ExternalDNSAnnotationPrefix = "external-dns.alpha.kubernetes.io/"
	ExternalDNSTarget           = "external-dns.alpha.kubernetes.io/target"
	ExternalDNSHostname         = "external-dns.alpha.kubernetes.io/hostname"
	ExternalDNSTTL              = "external-dns.alpha.kubernetes.io/ttl"

	// Gateway labels
	LabelManagedBy = "kubelb.k8c.io/managed-by"
)
