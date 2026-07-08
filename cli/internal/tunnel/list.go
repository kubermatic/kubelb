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

package tunnel

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	kubelbce "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/config"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func List(ctx context.Context, k8s client.Client, cfg *config.Config) error {
	if cfg.IsCE() {
		return ErrTunnelNotAvailable
	}

	tunnelList := &kubelbce.TunnelList{}

	if err := k8s.List(ctx, tunnelList, client.InNamespace(cfg.TenantNamespace)); err != nil {
		return fmt.Errorf("failed to list tunnels: %w", err)
	}

	if len(tunnelList.Items) == 0 {
		fmt.Println("No tunnels found.")
		return nil
	}

	// Sort tunnels by creation time (newest first)
	sort.Slice(tunnelList.Items, func(i, j int) bool {
		return tunnelList.Items[i].CreationTimestamp.After(tunnelList.Items[j].CreationTimestamp.Time)
	})

	// Display tunnels in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tHOSTNAME\tSTATUS\tURL\tCREATED")

	for _, tunnel := range tunnelList.Items {
		name := tunnel.Name
		hostname := tunnel.Status.Hostname
		if hostname == "" {
			hostname = "-"
		}

		status := string(tunnel.Status.Phase)
		if status == "" {
			status = "Pending"
		}

		url := tunnel.Status.URL
		if url == "" {
			url = "-"
		}

		created := formatAge(tunnel.CreationTimestamp.Time)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, hostname, status, url, created)
	}

	return w.Flush()
}

// formatAge formats a time duration as a human-readable age string
func formatAge(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd", int(duration.Hours()/24))
}
