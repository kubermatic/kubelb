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
	"flag"
	"strings"
)

// Options holds configuration for the Ingress-to-Gateway conversion controller
type Options struct {
	StandaloneMode              bool
	Enabled                     bool
	GatewayName                 string
	GatewayNamespace            string
	GatewayClassName            string
	IngressClass                string
	DomainReplace               string
	DomainSuffix                string
	PropagateExternalDNS        bool
	CleanupStale                bool
	GatewayAnnotations          map[string]string
	DisableEnvoyGatewayFeatures bool
}

// gatewayAnnotationsFlag implements flag.Value for parsing comma-separated key=value pairs
type gatewayAnnotationsFlag struct {
	target *map[string]string
}

func (f *gatewayAnnotationsFlag) String() string {
	if f.target == nil || *f.target == nil {
		return ""
	}
	var pairs []string
	for k, v := range *f.target {
		pairs = append(pairs, k+"="+v)
	}
	return strings.Join(pairs, ",")
}

func (f *gatewayAnnotationsFlag) Set(value string) error {
	if *f.target == nil {
		*f.target = make(map[string]string)
	}
	if value == "" {
		return nil
	}
	for _, pair := range strings.Split(value, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			(*f.target)[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return nil
}

// BindFlags registers the conversion-related flags
func (o *Options) BindFlags(fs *flag.FlagSet) {
	fs.BoolVar(&o.StandaloneMode, "ingress-conversion-only", false, "Run as standalone Ingress-to-Gateway converter. Disables all other controllers.")
	fs.BoolVar(&o.Enabled, "enable-ingress-conversion", false, "Enable automatic Ingress to HTTPRoute conversion")
	fs.StringVar(&o.GatewayName, "conversion-gateway-name", "kubelb", "Gateway name for converted HTTPRoutes")
	fs.StringVar(&o.GatewayNamespace, "conversion-gateway-namespace", "", "Gateway namespace (empty = same namespace as HTTPRoute)")
	fs.StringVar(&o.GatewayClassName, "conversion-gateway-class", "kubelb", "GatewayClass name for created Gateway")
	fs.StringVar(&o.IngressClass, "conversion-ingress-class", "", "Only convert Ingresses with this class (empty = convert all)")
	fs.StringVar(&o.DomainReplace, "conversion-domain-replace", "",
		"Source domain to find in Ingress hostnames. Must be used with --conversion-domain-suffix. "+
			"Example: with --conversion-domain-replace=example.com --conversion-domain-suffix=new.io, "+
			"'app.example.com' becomes 'app.new.io'")
	fs.StringVar(&o.DomainSuffix, "conversion-domain-suffix", "",
		"Target domain to replace source domain with. Must be used with --conversion-domain-replace. "+
			"If either flag is empty, hostnames are not transformed.")
	fs.BoolVar(&o.PropagateExternalDNS, "conversion-propagate-external-dns-annotations", true, "Propagate external-dns annotations to Gateway (target) and HTTPRoute (others)")
	fs.BoolVar(&o.CleanupStale, "conversion-cleanup-stale", true, "Delete orphaned HTTPRoutes when hosts are removed from Ingress")
	fs.Var(&gatewayAnnotationsFlag{target: &o.GatewayAnnotations}, "conversion-gateway-annotations", "Annotations to add to created Gateway (comma-separated key=value pairs, e.g., 'cert-manager.io/cluster-issuer=letsencrypt,external-dns.alpha.kubernetes.io/target=lb.example.com')")
	fs.BoolVar(&o.DisableEnvoyGatewayFeatures, "conversion-disable-envoy-gateway-features", false, "Disable Envoy Gateway policy creation (SecurityPolicy, BackendTrafficPolicy, ClientTrafficPolicy). When enabled, annotations that require policies will generate warnings instead.")
}
