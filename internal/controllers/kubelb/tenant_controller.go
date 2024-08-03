/*
Copyright 2024 The KubeLB Authors.

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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	tenantresources "k8c.io/kubelb/internal/controllers/kubelb/resources/tenant"
	"k8c.io/reconciler/pkg/reconciling"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	TenantControllerName   = "tenant-controller"
	tenantNamespacePattern = "tenant-%s"
)

const kubeconfigTemplate = `apiVersion: v1
kind: Config
clusters:
- name: kubelb-cluster
  cluster:
    certificate-authority-data: {{ .CACertificate }}
    server: {{ .ServerURL }}
contexts:
- name: default-context
  context:
    cluster: kubelb-cluster
    namespace: {{ .Namespace }}
    user: default-user
current-context: default-context
users:
- name: default-user
  user:
    token: {{ .Token }}
`

// TenantReconciler reconciles an HTTPRoute Object
type TenantReconciler struct {
	ctrlclient.Client

	Config   *rest.Config
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tenants/status,verbs=get;update;patch

func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling Tenant")

	resource := &kubelbv1alpha1.Tenant{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Resource is marked for deletion
	if resource.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
			return r.cleanup(ctx, resource)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(resource, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(resource, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the Tenant")
			return ctrl.Result{Requeue: true}, nil
		}
		if err := r.Update(ctx, resource); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	err := r.reconcile(ctx, log, resource)
	if err != nil {
		log.Error(err, "reconciling failed")
	}

	return reconcile.Result{}, err
}

func (r *TenantReconciler) reconcile(ctx context.Context, _ logr.Logger, tenant *kubelbv1alpha1.Tenant) error {
	ownerReference := metav1.OwnerReference{
		APIVersion: tenant.APIVersion,
		Kind:       tenant.Kind,
		Name:       tenant.Name,
		UID:        tenant.UID,
	}

	namespace := fmt.Sprintf(tenantNamespacePattern, tenant.Name)

	// 1. Create namespace for tenant with owner reference
	nsReconcilers := []reconciling.NamedNamespaceReconcilerFactory{
		tenantresources.NamespaceReconciler(namespace, ownerReference),
	}

	if err := reconciling.ReconcileNamespaces(ctx, nsReconcilers, "", r.Client); err != nil {
		return fmt.Errorf("failed to reconcile namespace: %w", err)
	}

	// 2. Create RBAC
	saReconcilers := []reconciling.NamedServiceAccountReconcilerFactory{
		tenantresources.ServiceAccountReconciler(),
	}

	if err := reconciling.ReconcileServiceAccounts(ctx, saReconcilers, namespace, r.Client); err != nil {
		return fmt.Errorf("failed to reconcile service account: %w", err)
	}

	roleReconcilers := []reconciling.NamedRoleReconcilerFactory{
		tenantresources.RoleReconciler(),
	}

	if err := reconciling.ReconcileRoles(ctx, roleReconcilers, namespace, r.Client); err != nil {
		return fmt.Errorf("failed to reconcile role: %w", err)
	}

	roleBindingReconcilers := []reconciling.NamedRoleBindingReconcilerFactory{
		tenantresources.RoleBindingReconciler(namespace),
	}

	if err := reconciling.ReconcileRoleBindings(ctx, roleBindingReconcilers, namespace, r.Client); err != nil {
		return fmt.Errorf("failed to reconcile role binding: %w", err)
	}

	// 3. Create service account token secret.
	secretReconcilers := []reconciling.NamedSecretReconcilerFactory{
		tenantresources.SecretReconciler(),
	}

	if err := reconciling.ReconcileSecrets(ctx, secretReconcilers, namespace, r.Client); err != nil {
		return fmt.Errorf("failed to reconcile secret: %w", err)
	}

	// 4. Create secret with kubeconfig for the tenant.
	tenantKubeconfig, err := r.generateKubeconfig(ctx, r.Client, namespace)
	if err != nil {
		return fmt.Errorf("failed to generate kubeconfig: %w", err)
	}

	secretReconcilers = []reconciling.NamedSecretReconcilerFactory{
		tenantresources.TenantKubeconfigSecretReconciler(tenantKubeconfig),
	}

	// Create kubeconfig secret in the user cluster namespace.
	if err := reconciling.ReconcileSecrets(ctx, secretReconcilers, namespace, r.Client); err != nil {
		return fmt.Errorf("failed to reconcile kubeLB tenant kubeconfig secret: %w", err)
	}

	return nil
}

func (r *TenantReconciler) generateKubeconfig(ctx context.Context, client ctrlruntimeclient.Client, namespace string) (string, error) {
	secret := corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: tenantresources.ServiceAccountTokenSecretName}, &secret)
	if err != nil {
		return "", fmt.Errorf("failed to get ServiceAccount token Secret: %w", err)
	}

	serverURL := r.Config.Host
	ca := r.Config.TLSClientConfig.CAData
	token := secret.Data[corev1.ServiceAccountTokenKey]

	// Generate kubeconfig.
	data := struct {
		CACertificate string
		Token         string
		Namespace     string
		ServerURL     string
	}{
		CACertificate: base64.StdEncoding.EncodeToString(ca),
		Token:         string(token),
		Namespace:     namespace,
		ServerURL:     serverURL,
	}

	tmpl, err := template.New("kubeconfig").Parse(kubeconfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse kubeconfig template: %w", err)
	}

	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, data); err != nil {
		return "", fmt.Errorf("failed to execute kubeconfig template: %w", err)
	}
	return buf.String(), nil
}

func (r *TenantReconciler) cleanup(ctx context.Context, tenant *kubelbv1alpha1.Tenant) (ctrl.Result, error) {
	namespace := fmt.Sprintf(tenantNamespacePattern, tenant.Name)
	for _, resource := range tenantresources.Deletion(namespace) {
		err := r.Client.Delete(ctx, resource)
		if err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed to ensure kubeLB resources are removed/not present on management cluster: %w", err)
		}
	}

	// Clean up is complete so remove the finalizer.
	controllerutil.RemoveFinalizer(tenant, CleanupFinalizer)
	if err := r.Update(ctx, tenant); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbv1alpha1.Tenant{}).
		Complete(r)
}
