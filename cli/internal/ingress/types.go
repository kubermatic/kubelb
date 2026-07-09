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

package ingress

import (
	"k8c.io/kubelb/pkg/conversion"
)

// Status represents the conversion state of an Ingress
type Status string

const (
	StatusConverted Status = conversion.ConversionStatusConverted
	StatusPartial   Status = conversion.ConversionStatusPartial
	StatusPending   Status = conversion.ConversionStatusPending
	StatusFailed    Status = conversion.ConversionStatusFailed
	StatusSkipped   Status = conversion.ConversionStatusSkipped
	StatusNew       Status = "new" // Not yet converted
)

// Info holds ingress metadata and conversion status
type Info struct {
	Name       string
	Namespace  string
	Status     Status
	Hosts      []string
	HTTPRoutes []string
	GRPCRoutes []string
	Warnings   []string
	SkipReason string
}

// Summary holds aggregate counts
type Summary struct {
	Total     int
	Converted int
	Partial   int
	Pending   int
	Failed    int
	Skipped   int
	New       int
}
