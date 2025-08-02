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

package kubelb

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"

	kubelbv1alpha1 "k8c.io/kubelb/api/ce/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/internal/tunnel"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	TunnelControllerName = "tunnel-controller"
	TokenLength          = 32
)

// TunnelReconciler reconciles a Tunnel Object
type TunnelReconciler struct {
	ctrlruntimeclient.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	Namespace          string
	EnvoyProxyTopology EnvoyProxyTopology
	DisableGatewayAPI  bool
}

// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tunnels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tunnels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubelb.k8c.io,resources=tunnels/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName)

	log.Info("Reconciling")

	tunnel := &kubelbv1alpha1.Tunnel{}
	if err := r.Get(ctx, req.NamespacedName, tunnel); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Before proceeding further we need to make sure that the resource is reconcilable.
	tenant, config, err := GetTenantAndConfig(ctx, r.Client, r.Namespace, RemoveTenantPrefix(tunnel.Namespace))
	if err != nil {
		log.Error(err, "unable to fetch Tenant and Config, cannot proceed")
		return reconcile.Result{}, err
	}

	// Check if tunneling is disabled in the config
	if config.Spec.Tunnel.Disable {
		log.Info("Tunneling is disabled in config, skipping reconciliation")
		if err := r.updateStatus(ctx, tunnel, kubelbv1alpha1.TunnelPhaseFailed, "Tunneling is disabled in configuration"); err != nil {
			log.Error(err, "failed to update status")
		}
		return reconcile.Result{}, nil
	}

	// Check if connection manager URL is configured
	if config.Spec.Tunnel.ConnectionManagerURL == "" {
		log.Error(nil, "Connection manager URL not configured")
		if err := r.updateStatus(ctx, tunnel, kubelbv1alpha1.TunnelPhaseFailed, "Connection manager URL not configured"); err != nil {
			log.Error(err, "failed to update status")
		}
		return reconcile.Result{}, fmt.Errorf("connection manager URL not configured in Config resource")
	}

	// Resource is marked for deletion
	if tunnel.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(tunnel, CleanupFinalizer) {
			return r.cleanup(ctx, log, tunnel)
		}
		// Finalizer doesn't exist so clean up is already done
		return reconcile.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(tunnel, CleanupFinalizer) {
		if ok := controllerutil.AddFinalizer(tunnel, CleanupFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer for the Tunnel")
			return ctrl.Result{Requeue: true}, nil
		}
		if err := r.Update(ctx, tunnel); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	result, err := r.reconcile(ctx, log, tunnel, config, tenant)
	if err != nil {
		log.Error(err, "reconciling failed")
		if statusErr := r.updateStatus(ctx, tunnel, kubelbv1alpha1.TunnelPhaseFailed, err.Error()); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		r.Recorder.Eventf(tunnel, corev1.EventTypeWarning, "ReconcileFailed", "Failed to reconcile tunnel: %v", err)
	}

	return result, err
}

func (r *TunnelReconciler) reconcile(ctx context.Context, log logr.Logger, tunnelObj *kubelbv1alpha1.Tunnel, config *kubelbv1alpha1.Config, tenant *kubelbv1alpha1.Tenant) (ctrl.Result, error) {
	log.V(1).Info("Starting reconciliation")

	// Update status to pending if not already set
	if tunnelObj.Status.Phase == "" {
		if err := r.updateStatus(ctx, tunnelObj, kubelbv1alpha1.TunnelPhasePending, "Provisioning tunnel"); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
		}
	}

	// Create or update SyncSecret with token
	if err := r.ensureTunnelAuth(ctx, log, tunnelObj); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure tunnel authentication: %w", err)
	}

	// Get annotations from tenant and config
	annotations := GetAnnotations(tenant, config)

	// Reconcile the tunnel using the business logic
	tunnelReconciler := tunnel.NewReconciler(r.Client, r.Scheme, r.Recorder, r.DisableGatewayAPI)
	routeRef, err := tunnelReconciler.Reconcile(ctx, log, tunnelObj, config, tenant, annotations)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile tunnel: %w", err)
	}

	// Update routeRef and persist it immediately
	oldRouteRef := tunnelObj.Status.Resources.RouteRef
	tunnelObj.Status.Resources.RouteRef = routeRef

	// Only update status if routeRef actually changed
	if !equality.Semantic.DeepEqual(oldRouteRef, routeRef) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestTunnel := &kubelbv1alpha1.Tunnel{}
			if err := r.Get(ctx, ctrlruntimeclient.ObjectKey{Name: tunnelObj.Name, Namespace: tunnelObj.Namespace}, latestTunnel); err != nil {
				return err
			}
			latestTunnel.Status.Resources.RouteRef = routeRef
			return r.Status().Update(ctx, latestTunnel)
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update tunnel status: %w", err)
		}
	}

	// Perform health checks before marking as ready
	dnsReady, endpointReady, tlsReady := r.updateHealthConditions(log, tunnelObj)

	// If health checks fail, requeue to try again in 5 seconds
	if !dnsReady || !endpointReady || !tlsReady {
		log.V(1).Info("Health checks not ready, requeuing", "dnsReady", dnsReady, "endpointReady", endpointReady, "tlsReady", tlsReady)
		if err := r.updateStatus(ctx, tunnelObj, kubelbv1alpha1.TunnelPhasePending, "Waiting for DNS, endpoint, and TLS to be ready"); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil // Requeue after 5 seconds
	}

	if err := r.updateStatus(ctx, tunnelObj, kubelbv1alpha1.TunnelPhaseReady, "Tunnel is ready"); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}
	r.Recorder.Eventf(tunnelObj, corev1.EventTypeNormal, "TunnelReady", "Tunnel is ready at %s", tunnelObj.Status.URL)
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) cleanup(ctx context.Context, log logr.Logger, tunnelObj *kubelbv1alpha1.Tunnel) (ctrl.Result, error) {
	log.Info("Cleaning up tunnel")

	if err := r.updateStatus(ctx, tunnelObj, kubelbv1alpha1.TunnelPhaseTerminating, "Cleaning up tunnel"); err != nil {
		log.Error(err, "failed to update status during cleanup")
	}
	tunnelReconciler := tunnel.NewReconciler(r.Client, r.Scheme, r.Recorder, r.DisableGatewayAPI)
	if err := tunnelReconciler.Cleanup(ctx, tunnelObj); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to cleanup tunnel: %w", err)
	}

	controllerutil.RemoveFinalizer(tunnelObj, CleanupFinalizer)
	if err := r.Update(ctx, tunnelObj); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	r.Recorder.Eventf(tunnelObj, corev1.EventTypeNormal, "TunnelDeleted", "Tunnel has been deleted")

	return reconcile.Result{}, nil
}

func (r *TunnelReconciler) updateStatus(ctx context.Context, tunnel *kubelbv1alpha1.Tunnel, phase kubelbv1alpha1.TunnelPhase, message string) error {
	tunnel.Status.Phase = phase

	// Update conditions - only update the phase condition, preserve others
	condition := metav1.Condition{
		Type:               string(phase),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             string(phase),
		Message:            message,
	}

	// Replace or add the phase condition without affecting other conditions
	found := false
	for i, c := range tunnel.Status.Conditions {
		if c.Type == condition.Type {
			tunnel.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		tunnel.Status.Conditions = append(tunnel.Status.Conditions, condition)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestTunnel := &kubelbv1alpha1.Tunnel{}
		if err := r.Get(ctx, ctrlruntimeclient.ObjectKey{Name: tunnel.Name, Namespace: tunnel.Namespace}, latestTunnel); err != nil {
			return err
		}
		latestTunnel.Status.Phase = tunnel.Status.Phase
		latestTunnel.Status.Conditions = tunnel.Status.Conditions
		return r.Status().Update(ctx, latestTunnel)
	})
}

// generateToken generates a cryptographically secure random token
func generateToken() (string, error) {
	// Create a byte slice to hold the random data
	tokenBytes := make([]byte, TokenLength)

	// Read random bytes
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	token := base64.StdEncoding.EncodeToString(tokenBytes)
	return token, nil
}

// ensureTunnelAuth creates or updates the SyncSecret with token for the tunnel
func (r *TunnelReconciler) ensureTunnelAuth(ctx context.Context, log logr.Logger, tunnel *kubelbv1alpha1.Tunnel) error {
	syncSecretName := fmt.Sprintf("tunnel-auth-%s", tunnel.Name)

	// Check if SyncSecret already exists
	existingSyncSecret := &kubelbv1alpha1.SyncSecret{}
	err := r.Get(ctx, ctrlruntimeclient.ObjectKey{
		Namespace: tunnel.Namespace,
		Name:      syncSecretName,
	}, existingSyncSecret)

	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get existing SyncSecret: %w", err)
	}

	var needsUpdate bool
	var syncSecret *kubelbv1alpha1.SyncSecret

	if kerrors.IsNotFound(err) {
		// Create new SyncSecret
		syncSecret = &kubelbv1alpha1.SyncSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      syncSecretName,
				Namespace: tunnel.Namespace,
				Labels: map[string]string{
					"kubelb.k8c.io/tunnel":          tunnel.Name,
					"kubelb.k8c.io/tunnel-hostname": tunnel.Status.Hostname,
				},
			},
			Data: make(map[string][]byte),
		}
		needsUpdate = true
		log.V(2).Info("Creating new SyncSecret for tunnel")
	} else {
		syncSecret = existingSyncSecret.DeepCopy()

		if syncSecret.Labels == nil {
			syncSecret.Labels = make(map[string]string)
		}
		syncSecret.Labels["kubelb.k8c.io/tunnel"] = tunnel.Name
		syncSecret.Labels["kubelb.k8c.io/tunnel-hostname"] = tunnel.Status.Hostname
		log.V(2).Info("Using existing SyncSecret for tunnel")
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(tunnel, syncSecret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Check if token needs generation/renewal
	if needsUpdate || r.needsTokenRenewal(syncSecret) {
		if err := r.generateAndStoreToken(syncSecret); err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}
		needsUpdate = true
		log.V(2).Info("Generated new token for tunnel")
	}

	// Create or update the SyncSecret
	if kerrors.IsNotFound(err) {
		if err := r.Create(ctx, syncSecret); err != nil {
			return fmt.Errorf("failed to create SyncSecret: %w", err)
		}
		log.Info("Created SyncSecret for tunnel", "syncSecret", syncSecretName)
	} else if needsUpdate || !equality.Semantic.DeepEqual(syncSecret.Labels, existingSyncSecret.Labels) ||
		!equality.Semantic.DeepEqual(syncSecret.Data, existingSyncSecret.Data) {
		if err := r.Update(ctx, syncSecret); err != nil {
			return fmt.Errorf("failed to update SyncSecret: %w", err)
		}
		log.Info("Updated SyncSecret for tunnel", "syncSecret", syncSecretName)
	}

	return nil
}

// generateAndStoreToken generates a new token and stores it in the SyncSecret
func (r *TunnelReconciler) generateAndStoreToken(syncSecret *kubelbv1alpha1.SyncSecret) error {
	// Generate token (24 hours validity)
	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	syncSecret.Data["token"] = []byte(token)
	tokenExpiry := time.Now().Add(24 * time.Hour)
	syncSecret.Data["tokenExpiry"] = []byte(tokenExpiry.Format(time.RFC3339))

	return nil
}

// needsTokenRenewal checks if the token needs renewal (1 hour before expiry)
func (r *TunnelReconciler) needsTokenRenewal(syncSecret *kubelbv1alpha1.SyncSecret) bool {
	expiryData, exists := syncSecret.Data["tokenExpiry"]
	if !exists {
		return true
	}

	expiry, err := time.Parse(time.RFC3339, string(expiryData))
	if err != nil {
		return true
	}

	// Renew 1 hour before expiry
	return time.Until(expiry) < 1*time.Hour
}

// updateHealthConditions performs DNS, endpoint, and TLS health checks and updates conditions
func (r *TunnelReconciler) updateHealthConditions(log logr.Logger, tunnel *kubelbv1alpha1.Tunnel) (bool, bool, bool) {
	if tunnel.Status.Hostname == "" {
		return false, false, false
	}

	dnsReady := r.checkDNSResolution(tunnel.Status.Hostname)
	r.updateCondition(tunnel, "DNSReady", dnsReady, "DNS resolution check")

	endpointReady := r.checkEndpointHealth(tunnel.Status.URL)
	r.updateCondition(tunnel, "EndpointReady", endpointReady, "Endpoint health check")

	tlsReady := r.checkTLSHealth(tunnel.Status.URL)
	r.updateCondition(tunnel, "TLSReady", tlsReady, "TLS certificate check")

	log.V(1).Info("Updated health conditions", "dnsReady", dnsReady, "endpointReady", endpointReady, "tlsReady", tlsReady)
	return dnsReady, endpointReady, tlsReady
}

// checkDNSResolution checks if the hostname resolves to an IP address
func (r *TunnelReconciler) checkDNSResolution(hostname string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		r.Log.V(2).Info("DNS resolution failed", "hostname", hostname, "error", err)
		return false
	}
	r.Log.V(3).Info("DNS resolution successful", "hostname", hostname, "ips", ips)
	return true
}

// checkEndpointHealth checks if the tunnel endpoint is responding
func (r *TunnelReconciler) checkEndpointHealth(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create HTTP client with timeout and TLS config
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip cert verification for health check
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		r.Log.V(2).Info("Endpoint health check failed - request creation", "url", url, "error", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		r.Log.V(2).Info("Endpoint health check failed - request execution", "url", url, "error", err)
		return false
	}
	defer resp.Body.Close()

	// Consider any response (even 404) as healthy since it means the endpoint is responding
	healthy := resp.StatusCode < 600
	if healthy {
		r.Log.V(3).Info("Endpoint health check successful", "url", url, "statusCode", resp.StatusCode)
	} else {
		r.Log.V(2).Info("Endpoint health check failed - bad status code", "url", url, "statusCode", resp.StatusCode)
	}
	return healthy
}

// checkTLSHealth checks if the TLS endpoint has a working TLS connection
func (r *TunnelReconciler) checkTLSHealth(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create HTTP client with timeout - skip cert verification to handle self-signed certs
	// We're checking if TLS handshake works, not certificate validity
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip cert verification to handle self-signed certificates
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		r.Log.V(2).Info("TLS health check failed - request creation", "url", url, "error", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		// TLS handshake or connection errors will cause this to fail
		r.Log.V(2).Info("TLS health check failed - request execution", "url", url, "error", err)
		return false
	}
	defer resp.Body.Close()

	// If we get here, TLS handshake succeeded (even with self-signed cert)
	// Any HTTP response code means TLS is working
	r.Log.V(3).Info("TLS health check successful", "url", url, "statusCode", resp.StatusCode)
	return true
}

// updateCondition updates a specific condition in the tunnel status
func (r *TunnelReconciler) updateCondition(tunnel *kubelbv1alpha1.Tunnel, conditionType string, status bool, message string) {
	conditionStatus := metav1.ConditionFalse
	reason := "Failed"
	if status {
		conditionStatus = metav1.ConditionTrue
		reason = "Ready"
	}

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Replace or add the condition
	found := false
	for i, c := range tunnel.Status.Conditions {
		if c.Type == condition.Type {
			// Only update if status changed to avoid unnecessary updates
			if c.Status != condition.Status {
				tunnel.Status.Conditions[i] = condition
			}
			found = true
			break
		}
	}
	if !found {
		tunnel.Status.Conditions = append(tunnel.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubelbv1alpha1.Tunnel{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
