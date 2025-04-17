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
	"errors"
	"fmt"
	"html/template"
	"net"
	"strconv"
	"time"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/kubelb.k8c.io/v1alpha1"
	tenantresources "k8c.io/kubelb/internal/controllers/kubelb/resources/tenant"
	"k8c.io/reconciler/pkg/reconciling"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	TenantControllerName    = "tenant-controller"
	tenantNamespacePattern  = "tenant-%s"
	configMapName           = "cluster-info"
	kubernetesEndpointsName = "kubernetes"
	securePortName          = "https"
	requeueAfter            = 10 * time.Second
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
	ctrlruntimeclient.Client

	Config   *rest.Config
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete;bind;escalate
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete;bind;escalate
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=routes,verbs=get;list;deletecollection
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=loadbalancers,verbs=get;list;deletecollection
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=syncsecrets,verbs=get;list;deletecollection

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

func (r *TenantReconciler) reconcile(ctx context.Context, log logr.Logger, tenant *kubelbv1alpha1.Tenant) error {
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
	tenantKubeconfig, err := r.generateKubeconfig(ctx, r.Client, log, namespace)
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

func (r *TenantReconciler) generateKubeconfig(ctx context.Context, client ctrlruntimeclient.Client, log logr.Logger, namespace string) (string, error) {
	secret := corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: tenantresources.ServiceAccountTokenSecretName}, &secret)
	if err != nil {
		return "", fmt.Errorf("failed to get ServiceAccount token Secret: %w", err)
	}

	serverURL := r.Config.Host
	conf, err := GetKubeconfig(ctx, client, log)
	if err != nil || conf == nil {
		return "", fmt.Errorf("failed to compute the server URL for kubeconfig: %w", err)
	}
	for key := range conf.Clusters {
		if conf.Clusters[key].Server != "" {
			serverURL = conf.Clusters[key].Server
			break
		}
	}

	ca := secret.Data[corev1.ServiceAccountRootCAKey]
	token := secret.Data[corev1.ServiceAccountTokenKey]

	if len(token) == 0 {
		return "", fmt.Errorf("failed to get ServiceAccount token")
	}
	if len(ca) == 0 {
		return "", fmt.Errorf("failed to get CA certificate")
	}

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

func GetKubeconfig(ctx context.Context, client ctrlruntimeclient.Client, log logr.Logger) (*clientcmdapi.Config, error) {
	cm, err := getKubeconfigFromConfigMap(ctx, client)
	if err != nil {
		log.V(3).Info(fmt.Sprintf("could not get cluster-info kubeconfig from configmap: %v", err))
		log.V(3).Info("falling back to retrieval via endpoint")
		return buildKubeconfigFromEndpoint(ctx, client)
	}
	return cm, nil
}

func getKubeconfigFromConfigMap(ctx context.Context, client ctrlruntimeclient.Client) (*clientcmdapi.Config, error) {
	cm := &corev1.ConfigMap{}
	if err := client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: metav1.NamespacePublic}, cm); err != nil {
		return nil, err
	}

	data, found := cm.Data["kubeconfig"]
	if !found {
		return nil, errors.New("no kubeconfig found in cluster-info configmap")
	}
	return clientcmd.Load([]byte(data))
}

func buildKubeconfigFromEndpoint(ctx context.Context, client ctrlruntimeclient.Client) (*clientcmdapi.Config, error) {
	endpoint := &corev1.Endpoints{}
	if err := client.Get(ctx, types.NamespacedName{Name: kubernetesEndpointsName, Namespace: metav1.NamespaceDefault}, endpoint); err != nil {
		return nil, err
	}

	if len(endpoint.Subsets) == 0 {
		return nil, errors.New("no subsets in the kubernetes endpoints resource")
	}
	subset := endpoint.Subsets[0]

	if len(subset.Addresses) == 0 {
		return nil, errors.New("no addresses in the first subset of the kubernetes endpoints resource")
	}
	address := subset.Addresses[0]

	ip := net.ParseIP(address.IP)
	if ip == nil {
		return nil, errors.New("could not parse ip from ")
	}

	getSecurePort := func(_ corev1.EndpointSubset) *corev1.EndpointPort {
		for _, p := range subset.Ports {
			if p.Name == securePortName {
				return &p
			}
		}
		return nil
	}

	port := getSecurePort(subset)
	if port == nil {
		return nil, errors.New("no secure port in the subset")
	}
	url := fmt.Sprintf("https://%s", net.JoinHostPort(ip.String(), strconv.Itoa(int(port.Port))))

	return &clientcmdapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			"": {
				Server: url,
			},
		},
	}, nil
}

func (r *TenantReconciler) cleanup(ctx context.Context, tenant *kubelbv1alpha1.Tenant) (ctrl.Result, error) {
	namespace := fmt.Sprintf(tenantNamespacePattern, tenant.Name)

	result, err := r.cleanupResources(ctx, namespace)
	if err != nil {
		return result, fmt.Errorf("failed to cleanup resources: %w", err)
	}

	// Delete all resources in the namespace
	for _, resource := range tenantresources.Deletion(namespace) {
		err := r.Delete(ctx, resource)
		if err != nil && !kerrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed to ensure kubeLB resources are removed/not present on management cluster: %w", err)
		}
	}

	controllerutil.RemoveFinalizer(tenant, CleanupFinalizer)
	if err := r.Update(ctx, tenant); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *TenantReconciler) cleanupResources(ctx context.Context, namespace string) (ctrl.Result, error) {
	// Delete all Routes in the namespace
	if err := r.DeleteAllOf(ctx, &kubelbv1alpha1.Route{}, ctrlruntimeclient.InNamespace(namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete Routes: %w", err)
	}

	// Delete all LoadBalancers in the namespace
	if err := r.DeleteAllOf(ctx, &kubelbv1alpha1.LoadBalancer{}, ctrlruntimeclient.InNamespace(namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete LoadBalancers: %w", err)
	}

	// Delete all SyncSecrets in the namespace
	if err := r.DeleteAllOf(ctx, &kubelbv1alpha1.SyncSecret{}, ctrlruntimeclient.InNamespace(namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete SyncSecrets: %w", err)
	}

	// Ensure that routes, loadbalancers, and syncsecrets are removed.
	routes := &kubelbv1alpha1.RouteList{}
	if err := r.List(ctx, routes, ctrlruntimeclient.InNamespace(namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list HTTPRoutes: %w", err)
	}

	if len(routes.Items) > 0 {
		// Requeue until all resources are deleted
		return reconcile.Result{RequeueAfter: requeueAfter}, fmt.Errorf("routes still exist")
	}

	loadbalancers := &kubelbv1alpha1.LoadBalancerList{}
	if err := r.List(ctx, loadbalancers, ctrlruntimeclient.InNamespace(namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list LoadBalancers: %w", err)
	}

	if len(loadbalancers.Items) > 0 {
		// Requeue until all resources are deleted
		return reconcile.Result{RequeueAfter: requeueAfter}, fmt.Errorf("loadbalancers still exist")
	}

	syncsecrets := &kubelbv1alpha1.SyncSecretList{}
	if err := r.List(ctx, syncsecrets, ctrlruntimeclient.InNamespace(namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list SyncSecrets: %w", err)
	}

	if len(syncsecrets.Items) > 0 {
		// Requeue until all resources are deleted
		return reconcile.Result{RequeueAfter: requeueAfter}, fmt.Errorf("syncsecrets still exist")
	}

	return reconcile.Result{}, nil
}

func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(TenantControllerName).
		For(&kubelbv1alpha1.Tenant{}).
		Complete(r)
}
