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

package kubelb

import (
	"testing"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/event"
)

// The xDS SnapshotCache is keyed by tenant namespace and holds an entry for the
// process lifetime. Tenant deletion must reconcile so that entry is cleared,
// otherwise it survives until the manager restarts.
func TestTenantSpecChangedPredicateTriggersOnDelete(t *testing.T) {
	if !tenantSpecChangedPredicate().Delete(event.DeleteEvent{Object: &kubelbv1alpha1.Tenant{}}) {
		t.Fatal("tenant deletion must enqueue a reconcile so the tenant's snapshot is cleared")
	}
}
