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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func docsCmd() *cobra.Command {
	var outputDir string
	var hugoFormat bool

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate markdown documentation for all commands",
		Long: `Generate markdown documentation for all CLI commands and their parameters.
This creates individual markdown files for each command with complete usage information.

Use --hugo flag to generate Hugo-compatible documentation with front matter
for integration with static site generators.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			var prepender func(string) string
			var linkHandler func(string) string

			if hugoFormat {
				prepender = hugoPrepender
				linkHandler = hugoLinkHandler
			} else {
				prepender = empty
				linkHandler = identity
			}

			rootCmd := cmd.Root()
			rootCmd.DisableAutoGenTag = true
			err := doc.GenMarkdownTreeCustom(rootCmd, outputDir, prepender, linkHandler)
			if err != nil {
				return fmt.Errorf("failed to generate documentation: %w", err)
			}

			fmt.Printf("Documentation generated successfully in: %s\n", outputDir)
			if hugoFormat {
				fmt.Println("Hugo format enabled - files include front matter and Hugo-compatible links")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "./docs", "Output directory for generated documentation")
	cmd.Flags().BoolVar(&hugoFormat, "hugo", false, "Generate Hugo-compatible docs with front matter")
	return cmd
}

func identity(s string) string {
	return s
}

func empty(_ string) string {
	return ""
}

// hugoPrepender adds Hugo front matter to generated docs
func hugoPrepender(filename string) string {
	// Extract command name from filename (e.g., "kubelb_tunnel_connect.md" -> "kubelb tunnel connect")
	name := strings.TrimSuffix(filepath.Base(filename), ".md")
	title := strings.ReplaceAll(name, "_", " ")

	// Calculate weight based on command depth (more underscores = higher weight = appears later)
	depth := strings.Count(name, "_")
	weight := 10 + (depth * 20)

	date := time.Now().Format("2006-01-02T00:00:00+00:00")

	return fmt.Sprintf(`+++
title = "%s"
date = %s
weight = %d
+++

`, title, date, weight)
}

// hugoLinkHandler converts links to Hugo-compatible format
// Changes: "kubelb_tunnel.md" -> "../kubelb_tunnel" (relative path, no .md extension)
func hugoLinkHandler(name string) string {
	return "../" + strings.TrimSuffix(name, ".md")
}
