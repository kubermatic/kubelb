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

package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	gitVersion = "dev"
	gitCommit  = "unknown"
	buildDate  = "unknown"
)

func versionCmd() *cobra.Command {
	var short bool

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Print the version information of the KubeLB CLI`,
		Run: func(_ *cobra.Command, _ []string) {
			if short {
				fmt.Printf("%s\n", gitVersion)
				return
			}
			fmt.Printf("GitVersion: %s\n", gitVersion)
			fmt.Printf("GitCommit:  %s\n", gitCommit)
			fmt.Printf("BuildDate:  %s\n", buildDate)
			fmt.Printf("Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Printf("Compiler:   %s\n", runtime.Compiler)
			fmt.Printf("GoVersion:  %s\n", runtime.Version())
		},
		Example: `kubelb version`,
	}
	versionCmd.Flags().BoolVar(&short, "short", false, "Print only the version in short format")
	return versionCmd
}
