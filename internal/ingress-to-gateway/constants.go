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

	// Annotations
	AnnotationSkipConversion     = "kubelb.k8c.io/skip-conversion"
	AnnotationConversionStatus   = "kubelb.k8c.io/conversion-status"
	AnnotationConvertedHTTPRoute = "kubelb.k8c.io/converted-httproute"
	AnnotationConversionWarnings = "kubelb.k8c.io/conversion-warnings"

	// Legacy ingress class annotation (deprecated but still widely used)
	AnnotationIngressClass = "kubernetes.io/ingress.class"

	// Status values
	ConversionStatusConverted = "converted"
	ConversionStatusPartial   = "partial"
	ConversionStatusFailed    = "failed"

	// NGINX annotations (for skip logic)
	NginxCanary = "nginx.ingress.kubernetes.io/canary"
)
