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

package loadbalancer

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	kubelb "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/cli/internal/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DisplayLoadbalancerList(loadbalancers []kubelb.LoadBalancer) {
	if len(loadbalancers) == 0 {
		fmt.Println("No load balancers found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tHOSTNAME\tINGRESS ENDPOINTS\tTYPE\tCLI GENERATED\tAGE")

	for _, lb := range loadbalancers {
		// Get hostname from status
		hostname := constants.NoneValue
		if lb.Status.Hostname != nil && lb.Status.Hostname.Hostname != "" {
			hostname = lb.Status.Hostname.Hostname
		}

		// Get ingress endpoints from status
		ingressEndpoints := getIngressEndpointsSummary(lb.Status.LoadBalancer.Ingress)

		cliGenerated := boolToString(isCLIGenerated(lb))
		age := getAge(lb.CreationTimestamp)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			lb.Name,
			hostname,
			ingressEndpoints,
			string(lb.Spec.Type),
			cliGenerated,
			age,
		)
	}
}

func getIngressEndpointsSummary(ingress []corev1.LoadBalancerIngress) string {
	if len(ingress) == 0 {
		return constants.NoneValue
	}

	var addresses []string
	for _, ing := range ingress {
		if ing.IP != "" {
			addresses = append(addresses, ing.IP)
		} else if ing.Hostname != "" {
			addresses = append(addresses, ing.Hostname)
		}
	}

	if len(addresses) == 0 {
		return constants.NoneValue
	}

	if len(addresses) == 1 {
		return addresses[0]
	}

	return fmt.Sprintf("%s + %d more", addresses[0], len(addresses)-1)
}

func getAge(creationTime metav1.Time) string {
	return time.Since(creationTime.Time).Truncate(time.Second).String()
}

func isCLIGenerated(lb kubelb.LoadBalancer) bool {
	if lb.Annotations == nil {
		return false
	}

	value, exists := lb.Annotations[kubelb.CLIResourceAnnotation]
	if !exists {
		return false
	}
	return value == constants.TrueString
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
