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

import "flag"

// Options holds configuration for the Ingress-to-Gateway conversion controller
type Options struct {
	StandaloneMode       bool
	Enabled              bool
	GatewayName          string
	GatewayNamespace     string
	GatewayClassName     string
	DomainReplace        string
	DomainSuffix         string
	PropagateCertManager bool
	PropagateExternalDNS bool
}

// BindFlags registers the conversion-related flags
func (o *Options) BindFlags(fs *flag.FlagSet) {
	fs.BoolVar(&o.StandaloneMode, "ingress-conversion-only", false,
		"Run as standalone Ingress-to-Gateway converter. Disables all other controllers.")
	fs.BoolVar(&o.Enabled, "enable-ingress-conversion", false,
		"Enable automatic Ingress to HTTPRoute conversion")
	fs.StringVar(&o.GatewayName, "conversion-gateway-name", "kubelb",
		"Gateway name for converted HTTPRoutes")
	fs.StringVar(&o.GatewayNamespace, "conversion-gateway-namespace", "",
		"Gateway namespace (empty = same namespace as HTTPRoute)")
	fs.StringVar(&o.GatewayClassName, "conversion-gateway-class", "kubelb",
		"GatewayClass name for created Gateway")
	fs.StringVar(&o.DomainReplace, "conversion-domain-replace", "",
		"Domain suffix to replace in hostnames (e.g., example.com)")
	fs.StringVar(&o.DomainSuffix, "conversion-domain-suffix", "",
		"Replacement domain suffix for hostnames (e.g., new.io)")
	fs.BoolVar(&o.PropagateCertManager, "propagate-cert-manager-annotations", true,
		"Propagate cert-manager.io/* annotations to Gateway for automatic TLS")
	fs.BoolVar(&o.PropagateExternalDNS, "propagate-external-dns-annotations", true,
		"Propagate external-dns annotations to Gateway (target) and HTTPRoute (others)")
}
