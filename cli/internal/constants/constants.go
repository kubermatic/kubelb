/*
Copyright 2025 The KubeLB Authors.

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

package constants

import "time"

const (
	// Timeout constants
	DefaultWaitTimeout     = 4 * time.Minute
	DefaultPollInterval    = 3 * time.Second
	ProgressTickerInterval = 500 * time.Millisecond

	// CLI constants
	DefaultIngressClass = "kubelb"
	DefaultRouteDomain  = "kubelb.io"

	// Port constants
	HTTPPort    = 80
	HTTPSPort   = 443
	HTTPAltPort = 8080

	// Protocol constants
	ProtocolHTTP    = "http"
	DefaultProtocol = ProtocolHTTP

	// Kubernetes resource names
	DefaultEndpointName = "default"
	DefaultPortName     = "default"

	// Display constants
	NoneValue  = "<none>"
	TrueString = "true"
)
