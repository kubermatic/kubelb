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

package versioninfo

import "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

var (
	// GitVersion is the git tag. Set via ldflags.
	GitVersion = "dev"
	// GitCommit is the git commit hash. Set via ldflags.
	GitCommit = "unknown"
	// BuildDate is the build date. Set via ldflags.
	BuildDate = "unknown"
	// Edition is the KubeLB edition. Hardcoded to "CE".
	Edition = "CE"
)

// GetVersion returns the version information.
func GetVersion() v1alpha1.Version {
	return v1alpha1.Version{
		GitVersion: GitVersion,
		GitCommit:  GitCommit,
		BuildDate:  BuildDate,
		Edition:    Edition,
	}
}
