/*
Copyright 2024.

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

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/url"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	grafanaclient "github.com/grafana/grafana-openapi-client-go/client"
)

// GrafanaOrganizationReconciler reconciles a GrafanaOrganization object
type GrafanaOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	GrafanaUrl           = "http://grafana.monitoring.svc.cluster.local:3000"
	Namespace            = "monitoring"
	GrafanaSecretName    = "grafana"
	grafanaTlsSecretName = "grafana-tls"
)

//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations/finalizers,verbs=update

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the GrafanaOrganization closer to the desired state.
// TODO(zirko): Modify the Reconcile function to compare the state specified by
// the GrafanaOrganization object against the actual organization state, and then
// perform operations to make the organization state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *GrafanaOrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling Grafana Organization")
	defer logger.Info("Finished reconciling Grafana Organization")

	grafanaOrganization := &v1alpha1.GrafanaOrganization{}
	err := r.Client.Get(ctx, req.NamespacedName, grafanaOrganization)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	logger.WithValues("grafanaOrganization", grafanaOrganization.Spec.Name)

	// Get grafana admin-password and admin-user
	grafanaAdminAuth, err := getGrafanaAdminAut(ctx, r.Client)
	if err != nil {
		logger.Error(err, "Failed to fetch Grafana admin secret")
	}

	// Generate Grafana client
	grafanaClient, err := generateGrafanaClient(ctx, r.Client, grafanaAdminAuth, grafana)

	// Test connection to Grafana
	_, _, err = grafanaClient.GetHealth(ctx)
	if err != nil {
		logger.Error(err, "Failed to connect to Grafana")
	}

	// Handle deleted grafana organizations
	if !grafanaOrganization.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, grafanaOrganization)
	}

	// Handle non-deleted grafana organizations
	return r.reconcileCreate(ctx, grafanaOrganization)
}

// reconcileCreate creates the grafanaOrganization.
func (r GrafanaOrganizationReconciler) reconcileCreate(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	originalGrafanaOrganization := grafanaOrganization.DeepCopy()
	// If the grafanaOrganization doesn't have our finalizer, add it.
	if controllerutil.AddFinalizer(grafanaOrganization, v1alpha1.grafanaOrganizationFinalizer) {
		logger.Info("Add finalizer to grafana organization")
		// Register the finalizer immediately to avoid orphaning AWS resources on delete
		if err := r.Client.Patch(ctx, grafanaOrganization, client.MergeFrom(originalGrafanaOrganization)); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	//TODO Implement the logic to create the Grafana organization

	return ctrl.Result{}, nil
}

// reconcileDelete deletes the bucket.
func (r GrafanaOrganizationReconciler) reconcileDelete(ctx context.Context, grafanaOrganization *v1alpha1.GrafanaOrganization) error {
	logger := log.FromContext(ctx)

	//TODO Implement the logic to delete the organization from Grafana.

	logger.Info("Remove finalizer from grafana organization")
	// Remove the finalizer.
	originalGrafanaOrganization := grafanaOrganization.DeepCopy()
	controllerutil.RemoveFinalizer(grafanaOrganization, v1alpha1.grafanaOrganizationFinalizer)

	return r.Client.Patch(ctx, grafanaOrganization, client.MergeFrom(originalGrafanaOrganization))
}

func getGrafanaAdminAuth(ctx context.Context, client client.Client) ([]string, error) {
	grafanaAdminSecret := &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: Namespace,
		Name:      GrafanaSecretName,
	}, grafanaAdminSecret)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	adminPassword, _ := grafanaAdminSecret.Data["admin-password"]
	adminUser, _ := grafanaAdminSecret.Data["admin-user"]

	return []string{adminPassword, adminUser}, nil
}

func generateGrafanaClient(ctx context.Context, client client.Client, adminAuth []string) (*genapi.GrafanaHTTPAPI, error) {
	tlsConfig, err := buildTLSConfiguration(ctx, c)
	if err != nil {
		return nil, err
	}

	grafanaUrl, err := url.Parse(GrafanaUrl)
	if err != nil {
		return nil, fmt.Errorf("parsing url for client: %w", err)
	}

	cfg := &grafanaclient.TransportConfig{
		Schemes:  []string{grafanaUrl.Scheme},
		BasePath: "/api",
		Host:     grafanaUrl.Host,
		// We use basic auth to authenticate on grafana.
		BasicAuth: url.UserPassword(adminAuth[1], adminAuth[0]),
		// NumRetries contains the optional number of attempted retries.
		NumRetries: 0,
		TLSConfig:  tlsConfig,
	}

	cl := grafanaclient.NewHTTPClientWithConfig(nil, cfg)

	return cl, nil
}

// build the tls.Config object based on the content of the grafana-tls secret
func buildTLSConfiguration(ctx context.Context, c client.Client) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	secret := &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: Namespace,
		Name:      grafanaTlsSecretName,
	}, grafanaAdminSecret)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if secret.Data == nil {
		return nil, fmt.Errorf("empty credential secret: %v/%v", grafana.Namespace, tlsConfigBlock.CertSecretRef.Name)
	}

	crt, crtPresent := secret.Data["tls.crt"]
	key, keyPresent := secret.Data["tls.key"]

	if (crtPresent && !keyPresent) || (keyPresent && !crtPresent) {
		return nil, fmt.Errorf("invalid secret %v/%v. tls.crt and tls.key needs to be present together when one of them is declared", tlsConfigBlock.CertSecretRef.Namespace, tlsConfigBlock.CertSecretRef.Name)
	} else if crtPresent && keyPresent {
		loadedCrt, err := tls.X509KeyPair(crt, key)
		if err != nil {
			return nil, fmt.Errorf("certificate from secret %v/%v cannot be parsed : %w", grafana.Namespace, tlsConfigBlock.CertSecretRef.Name, err)
		}
		tlsConfig.Certificates = []tls.Certificate{loadedCrt}
	}

	return tlsConfig, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GrafanaOrganization{}).
		Complete(r)
}
