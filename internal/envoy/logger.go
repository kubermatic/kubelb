/*
Copyright 2020 The KubeLB Authors.

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

package envoy

import (
	"fmt"

	envoylog "github.com/envoyproxy/go-control-plane/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// xdsLog is shared with the xDS stream callbacks in server.go.
var xdsLog = log.Log.WithName("envoy-xds")

// Logger routes go-control-plane's logging through logr. go-control-plane calls
// it from inside stream callbacks, where the std log package's global stderr
// mutex becomes a contention point during reconnect storms.
//
// Debug gates the verbose levels; warnings and errors are always emitted.
type Logger struct {
	Debug bool
}

var _ envoylog.Logger = Logger{}

func (logger Logger) Debugf(format string, args ...interface{}) {
	if logger.Debug {
		xdsLog.V(2).Info(fmt.Sprintf(format, args...))
	}
}

func (logger Logger) Infof(format string, args ...interface{}) {
	if logger.Debug {
		xdsLog.V(1).Info(fmt.Sprintf(format, args...))
	}
}

func (logger Logger) Warnf(format string, args ...interface{}) {
	xdsLog.Info(fmt.Sprintf(format, args...))
}

func (logger Logger) Errorf(format string, args ...interface{}) {
	xdsLog.Error(nil, fmt.Sprintf(format, args...))
}
