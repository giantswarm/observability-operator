package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/agent/logexporter"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	// The objects alloy-logexporter's HelmRelease reads. All three values are fixed by
	// bases/collections/shared/base/alloy-logexporter.yaml in management-cluster-bases,
	// which declares them as optional valuesFrom entries with the ConfigMap last so it
	// wins, so they are not configurable here.
	logExportNamespace     = "monitoring"
	logExportConfigMapName = "alloy-logexporter-config"
	logExportSecretName    = "alloy-logexporter-secret" //nolint:gosec // G101: an object name, not a credential.

	// logExportValuesKey is the valuesKey both objects are declared with.
	logExportValuesKey = "values"
)

// LogExportReconciler reconciles LogExport objects into the single ConfigMap and Secret
// that switch alloy-logexporter on and configure it.
//
// The rendered values cover every LogExport on the installation at once, so this
// reconciler is installation-scoped rather than per-resource: whichever resource
// triggered a reconcile, the work is the same and the result is the same.
type LogExportReconciler struct {
	client.Client
	finalizerHelper FinalizerHelper
}

// SetupLogExportReconciler wires the reconciler into the manager. The caller gates this
// on --controllers-log-export-enabled.
func SetupLogExportReconciler(mgr manager.Manager, cfg config.Config) error {
	r := &LogExportReconciler{
		Client:          mgr.GetClient(),
		finalizerHelper: NewFinalizerHelper(mgr.GetClient(), observabilityv1alpha1.LogExportFinalizer),
	}

	return r.SetupWithManager(mgr)
}

// SetupWithManager registers the controller with the manager.
//
// The rendered objects carry no owner reference: they are derived from every LogExport, so
// an ownerRef from any single one would make deleting that resource garbage-collect them
// and stop every other export.
func (r *LogExportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named("logexport").
		For(&observabilityv1alpha1.LogExport{}).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build logexport controller: %w", err)
	}
	return nil
}

//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=logexports,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=logexports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=logexports/finalizers,verbs=update

// Reconcile rebuilds the exporter configuration from the current set of LogExports.
func (r *LogExportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("logexport", req.NamespacedName)
	ctx = log.IntoContext(ctx, logger)

	export := &observabilityv1alpha1.LogExport{}
	if err := r.Get(ctx, req.NamespacedName, export); err != nil {
		if apierrors.IsNotFound(err) {
			// Usually the delete path below has already re-rendered without it. A
			// resource can also vanish without that happening -- a force delete, or a
			// finalizer removed by hand -- so re-render rather than assume. Rendering
			// is idempotent, so the ordinary delete just repeats itself here.
			return ctrl.Result{}, r.renderExporterConfiguration(ctx)
		}
		return ctrl.Result{}, fmt.Errorf("failed to get log export: %w", err)
	}

	if !export.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, export)
	}

	return r.reconcileCreate(ctx, export)
}

func (r *LogExportReconciler) reconcileCreate(ctx context.Context, export *observabilityv1alpha1.LogExport) (ctrl.Result, error) {
	added, err := r.finalizerHelper.EnsureAdded(ctx, export)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
	}
	if added {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, r.renderExporterConfiguration(ctx)
}

func (r *LogExportReconciler) reconcileDelete(ctx context.Context, export *observabilityv1alpha1.LogExport) (ctrl.Result, error) {
	// Re-render before dropping the finalizer. This export still exists, but
	// activeExports skips anything with a deletion timestamp, so it falls out of the
	// result while every other export is preserved.
	if err := r.renderExporterConfiguration(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.finalizerHelper.EnsureRemoved(ctx, export); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

// renderExporterConfiguration rebuilds both objects from every LogExport that is not
// being deleted.
func (r *LogExportReconciler) renderExporterConfiguration(ctx context.Context) error {
	exports, err := r.activeExports(ctx)
	if err != nil {
		return err
	}

	// Nothing selected any more: remove both objects so the HelmRelease falls back to
	// alloy-logexporter-defaults and the app returns to zero replicas.
	if len(exports) == 0 {
		if err := r.deleteObject(ctx, logExportConfigMapName, &corev1.ConfigMap{}); err != nil {
			return err
		}
		return r.deleteObject(ctx, logExportSecretName, &corev1.Secret{})
	}

	credentials, err := r.resolveCredentials(ctx, exports)
	if err != nil {
		return err
	}

	values, err := logexporter.RenderValues(exports)
	if err != nil {
		return fmt.Errorf("failed to render alloy-logexporter values: %w", err)
	}

	environment, err := logexporter.SecretEnv(exports, credentials)
	if err != nil {
		return fmt.Errorf("failed to build alloy-logexporter environment: %w", err)
	}

	// The Secret goes first: the ConfigMap is what switches the app on, so writing it
	// last means the credentials are already in place when the exporter starts. The
	// teardown above is the same reasoning reversed -- switch off, then withdraw them.
	if err := r.writeSecret(ctx, environment); err != nil {
		return err
	}

	return r.writeConfigMap(ctx, values)
}

// activeExports lists every LogExport on the installation, skipping those being deleted
// so a removed export stops being rendered immediately.
func (r *LogExportReconciler) activeExports(ctx context.Context) ([]observabilityv1alpha1.LogExport, error) {
	list := &observabilityv1alpha1.LogExportList{}
	if err := r.List(ctx, list); err != nil {
		return nil, fmt.Errorf("failed to list log exports: %w", err)
	}

	active := make([]observabilityv1alpha1.LogExport, 0, len(list.Items))
	for _, export := range list.Items {
		if !export.DeletionTimestamp.IsZero() {
			continue
		}
		active = append(active, export)
	}
	return active, nil
}

// resolveCredentials reads the Secret each S3 destination references. The reference is
// name-only and resolved in the LogExport's own namespace, so a resource can only reach
// credentials a writer in that namespace already has.
func (r *LogExportReconciler) resolveCredentials(ctx context.Context, exports []observabilityv1alpha1.LogExport) (map[client.ObjectKey]logexporter.Credentials, error) {
	credentials := map[client.ObjectKey]logexporter.Credentials{}

	for _, export := range exports {
		s3 := export.Spec.Destination.S3
		if s3 == nil || s3.CredentialsRef == nil {
			continue
		}

		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: export.Namespace, Name: s3.CredentialsRef.Name}
		if err := r.Get(ctx, key, secret); err != nil {
			return nil, fmt.Errorf("failed to get credentials secret %s for log export %s/%s: %w", key, export.Namespace, export.Name, err)
		}

		accessKeyID, ok := secret.Data[logexporter.AccessKeyIDEnv]
		if !ok {
			return nil, fmt.Errorf("credentials secret %s has no %s key", key, logexporter.AccessKeyIDEnv)
		}
		secretAccessKey, ok := secret.Data[logexporter.SecretAccessKeyEnv]
		if !ok {
			return nil, fmt.Errorf("credentials secret %s has no %s key", key, logexporter.SecretAccessKeyEnv)
		}

		// Keyed by the LogExport, not the Secret: that is what SecretEnv looks up.
		credentials[client.ObjectKey{Namespace: export.Namespace, Name: export.Name}] = logexporter.Credentials{
			AccessKeyID:     string(accessKeyID),
			SecretAccessKey: string(secretAccessKey),
		}
	}

	return credentials, nil
}

func (r *LogExportReconciler) writeConfigMap(ctx context.Context, values string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      logExportConfigMapName,
			Namespace: logExportNamespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		configMap.Data = map[string]string{logExportValuesKey: values}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to write configmap %s/%s: %w", logExportNamespace, logExportConfigMapName, err)
	}
	return nil
}

// writeSecret writes the credential environment, or removes the Secret when no export
// carries static credentials. The shared secret template renders nothing for an empty
// environment, and an empty document in the last-precedence valuesFrom entry is not worth
// relying on: absent is what management-cluster-bases declares as optional.
func (r *LogExportReconciler) writeSecret(ctx context.Context, environment map[string]string) error {
	if len(environment) == 0 {
		return r.deleteObject(ctx, logExportSecretName, &corev1.Secret{})
	}

	values, err := common.GenerateSecretData(environment, "")
	if err != nil {
		return fmt.Errorf("failed to generate alloy-logexporter secret data: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      logExportSecretName,
			Namespace: logExportNamespace,
		},
	}

	_, err = ctrl.CreateOrUpdate(ctx, r.Client, secret, func() error {
		secret.Data = map[string][]byte{logExportValuesKey: values}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to write secret %s/%s: %w", logExportNamespace, logExportSecretName, err)
	}
	return nil
}

func (r *LogExportReconciler) deleteObject(ctx context.Context, name string, object client.Object) error {
	object.SetName(name)
	object.SetNamespace(logExportNamespace)

	if err := r.Delete(ctx, object); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete %T %s/%s: %w", object, logExportNamespace, name, err)
	}
	return nil
}
