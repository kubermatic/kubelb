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

// Package annotations provides NGINX Ingress annotation handling for Gateway API conversion.
//
// Reference: https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/
package annotations

// Handled annotations - these are processed by the converter and either:
// - Converted to HTTPRoute filters
// - Generate policy suggestions (warnings with recommended Gateway API config)
// - Generate not-supported warnings
const (
	// Redirects - converted to RequestRedirect filters
	SSLRedirect           = "nginx.ingress.kubernetes.io/ssl-redirect"
	ForceSSLRedirect      = "nginx.ingress.kubernetes.io/force-ssl-redirect"
	PermanentRedirect     = "nginx.ingress.kubernetes.io/permanent-redirect"
	PermanentRedirectCode = "nginx.ingress.kubernetes.io/permanent-redirect-code"
	TemporalRedirect      = "nginx.ingress.kubernetes.io/temporal-redirect"

	// URL rewriting - converted to URLRewrite filters
	RewriteTarget = "nginx.ingress.kubernetes.io/rewrite-target"
	AppRoot       = "nginx.ingress.kubernetes.io/app-root"

	// Headers - converted to header modifier filters
	CustomHeaders   = "nginx.ingress.kubernetes.io/custom-headers"
	ProxySetHeaders = "nginx.ingress.kubernetes.io/proxy-set-headers"

	// Timeouts - generate BackendTrafficPolicy suggestions
	ProxyConnectTimeout = "nginx.ingress.kubernetes.io/proxy-connect-timeout"
	ProxySendTimeout    = "nginx.ingress.kubernetes.io/proxy-send-timeout"
	ProxyReadTimeout    = "nginx.ingress.kubernetes.io/proxy-read-timeout"

	// CORS - generate SecurityPolicy suggestions
	EnableCORS           = "nginx.ingress.kubernetes.io/enable-cors"
	CORSAllowOrigin      = "nginx.ingress.kubernetes.io/cors-allow-origin"
	CORSAllowMethods     = "nginx.ingress.kubernetes.io/cors-allow-methods"
	CORSAllowHeaders     = "nginx.ingress.kubernetes.io/cors-allow-headers"
	CORSExposeHeaders    = "nginx.ingress.kubernetes.io/cors-expose-headers"
	CORSAllowCredentials = "nginx.ingress.kubernetes.io/cors-allow-credentials"
	CORSMaxAge           = "nginx.ingress.kubernetes.io/cors-max-age"

	// Rate limiting - generate BackendTrafficPolicy suggestions
	LimitRPS         = "nginx.ingress.kubernetes.io/limit-rps"
	LimitConnections = "nginx.ingress.kubernetes.io/limit-connections"

	// IP access control - generate SecurityPolicy suggestions
	WhitelistSourceRange = "nginx.ingress.kubernetes.io/whitelist-source-range"
	DenylistSourceRange  = "nginx.ingress.kubernetes.io/denylist-source-range"

	// Request size - generate ClientTrafficPolicy suggestions
	ProxyBodySize = "nginx.ingress.kubernetes.io/proxy-body-size"

	// Session affinity - generate BackendTrafficPolicy suggestions
	Affinity              = "nginx.ingress.kubernetes.io/affinity"
	SessionCookieName     = "nginx.ingress.kubernetes.io/session-cookie-name"
	SessionCookiePath     = "nginx.ingress.kubernetes.io/session-cookie-path"
	SessionCookieExpires  = "nginx.ingress.kubernetes.io/session-cookie-expires"
	SessionCookieSameSite = "nginx.ingress.kubernetes.io/session-cookie-samesite"

	// Authentication - generate SecurityPolicy suggestions
	AuthType   = "nginx.ingress.kubernetes.io/auth-type"
	AuthURL    = "nginx.ingress.kubernetes.io/auth-url"
	AuthSecret = "nginx.ingress.kubernetes.io/auth-secret"
	AuthRealm  = "nginx.ingress.kubernetes.io/auth-realm"

	// Backend protocol - generate GRPCRoute or BackendTLSPolicy suggestions
	BackendProtocol = "nginx.ingress.kubernetes.io/backend-protocol"

	// Canary - generate weighted HTTPRoute suggestions
	Canary              = "nginx.ingress.kubernetes.io/canary"
	CanaryWeight        = "nginx.ingress.kubernetes.io/canary-weight"
	CanaryWeightTotal   = "nginx.ingress.kubernetes.io/canary-weight-total"
	CanaryByHeader      = "nginx.ingress.kubernetes.io/canary-by-header"
	CanaryByHeaderValue = "nginx.ingress.kubernetes.io/canary-by-header-value"
	CanaryByCookie      = "nginx.ingress.kubernetes.io/canary-by-cookie"

	// Not supported - generate warnings
	ServerSnippet        = "nginx.ingress.kubernetes.io/server-snippet"
	ConfigurationSnippet = "nginx.ingress.kubernetes.io/configuration-snippet"
	StreamSnippet        = "nginx.ingress.kubernetes.io/stream-snippet"
	EnableModSecurity    = "nginx.ingress.kubernetes.io/enable-modsecurity"
	EnableOWASPCoreRules = "nginx.ingress.kubernetes.io/enable-owasp-core-rules"
	UpstreamHashBy       = "nginx.ingress.kubernetes.io/upstream-hash-by"
)

// Headers - additional header annotations
const (
	UpstreamVhost     = "nginx.ingress.kubernetes.io/upstream-vhost"
	XForwardedPrefix  = "nginx.ingress.kubernetes.io/x-forwarded-prefix"
	ProxyRedirectFrom = "nginx.ingress.kubernetes.io/proxy-redirect-from"
	ProxyRedirectTo   = "nginx.ingress.kubernetes.io/proxy-redirect-to"
	PreserveHost      = "nginx.ingress.kubernetes.io/preserve-host"
)

// TLS/SSL annotations - now handled with warnings
const (
	SSLPassthrough     = "nginx.ingress.kubernetes.io/ssl-passthrough"
	ProxySSLSecret     = "nginx.ingress.kubernetes.io/proxy-ssl-secret"
	ProxySSLVerify     = "nginx.ingress.kubernetes.io/proxy-ssl-verify"
	ProxySSLName       = "nginx.ingress.kubernetes.io/proxy-ssl-name"
	ProxySSLServerName = "nginx.ingress.kubernetes.io/proxy-ssl-server-name"
)

// Rate limiting - now handled
const (
	LimitRPM = "nginx.ingress.kubernetes.io/limit-rpm"
)

// Regex annotation - now handled with standalone warning
const (
	UseRegex = "nginx.ingress.kubernetes.io/use-regex"
)

// Unhandled annotations - defined for reference but not processed by the converter.
// These are either rarely used or have no Gateway API equivalent.
const (
	LimitBurstMultiplier = "nginx.ingress.kubernetes.io/limit-burst-multiplier"
	LimitWhitelist       = "nginx.ingress.kubernetes.io/limit-whitelist"
	ClientBodyBufferSize = "nginx.ingress.kubernetes.io/client-body-buffer-size"
	AffinityMode         = "nginx.ingress.kubernetes.io/affinity-mode"
	AuthSecretType       = "nginx.ingress.kubernetes.io/auth-secret-type"
	ProxyBuffering       = "nginx.ingress.kubernetes.io/proxy-buffering"
	ProxyHTTPVersion     = "nginx.ingress.kubernetes.io/proxy-http-version"
	ProxyNextUpstream    = "nginx.ingress.kubernetes.io/proxy-next-upstream"
	ServiceUpstream      = "nginx.ingress.kubernetes.io/service-upstream"
	LoadBalance          = "nginx.ingress.kubernetes.io/load-balance"
	SSLCiphers           = "nginx.ingress.kubernetes.io/ssl-ciphers"
)
