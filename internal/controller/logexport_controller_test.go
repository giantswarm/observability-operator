package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// Ordered because every spec here writes the same two objects, whose names and namespace
// are fixed by management-cluster-bases, and reads every LogExport on the cluster. Ginkgo
// parallelises at spec level, so without this two specs would race over shared state.
var _ = Describe("LogExport Controller", Ordered, func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250

		// Fixtures asserted by more than one spec. Named only because they repeat.
		auditExport = "audit"
		auditBucket = "audit-export"
		s3Region    = "eu-west-2"
	)

	var (
		ctx        context.Context
		ns         string
		reconciler *LogExportReconciler
	)

	// configMapKey and secretKey are fixed by management-cluster-bases, so they are the
	// same for every test.
	configMapKey := types.NamespacedName{Namespace: logExportNamespace, Name: logExportConfigMapName}
	secretKey := types.NamespacedName{Namespace: logExportNamespace, Name: logExportSecretName}

	BeforeEach(func() {
		ctx = context.Background()
		ns = fmt.Sprintf("le-test-ns-%d", time.Now().UnixNano())

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())

		// The exporter's objects live in a fixed namespace shared by every test.
		err := k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: logExportNamespace}})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		reconciler = &LogExportReconciler{
			Client:          k8sClient,
			finalizerHelper: NewFinalizerHelper(k8sClient, observabilityv1alpha1.LogExportFinalizer),
		}
	})

	AfterEach(func() {
		// Every LogExport on the cluster feeds the render, and envtest runs no namespace
		// controller, so deleting the per-test namespace leaves its contents in place.
		// Anything left here would show up in the next spec's render, so remove the
		// resources explicitly, finalizer first.
		list := &observabilityv1alpha1.LogExportList{}
		Expect(k8sClient.List(ctx, list)).To(Succeed())
		for i := range list.Items {
			export := &list.Items[i]
			export.Finalizers = nil
			Expect(client.IgnoreNotFound(k8sClient.Update(ctx, export))).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, export))).To(Succeed())
		}
		Eventually(func() (int, error) {
			remaining := &observabilityv1alpha1.LogExportList{}
			err := k8sClient.List(ctx, remaining)
			return len(remaining.Items), err
		}, timeout, interval).Should(BeZero())

		// The rendered objects are shared across specs too.
		_ = k8sClient.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: logExportNamespace, Name: logExportConfigMapName}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: logExportNamespace, Name: logExportSecretName}})
		_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	})

	// reconcileTwice drives the finalizer early-return: the first call adds the
	// finalizer, the second renders.
	reconcileTwice := func(name string) {
		for range 2 {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	newExport := func(name, selector string, s3 observabilityv1alpha1.S3Destination) *observabilityv1alpha1.LogExport {
		return &observabilityv1alpha1.LogExport{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: observabilityv1alpha1.LogExportSpec{
				Selector: selector,
				Destination: observabilityv1alpha1.LogExportDestination{
					Type: observabilityv1alpha1.LogExportDestinationS3,
					S3:   &s3,
				},
			},
		}
	}

	// deleteAndReconcile removes an export and drives the delete path to completion.
	deleteAndReconcile := func(export *observabilityv1alpha1.LogExport) {
		Expect(k8sClient.Delete(ctx, export)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: export.Name}})
		Expect(err).NotTo(HaveOccurred())
	}

	It("renders the ConfigMap the HelmRelease reads, and no Secret without credentials", func() {
		export := newExport(auditExport, `{scrape_job="audit-logs"}`, observabilityv1alpha1.S3Destination{
			Bucket:  auditBucket,
			Region:  s3Region,
			RoleARN: "arn:aws:iam::123456789012:role/log-archive-writer",
		})
		Expect(k8sClient.Create(ctx, export)).To(Succeed())

		reconcileTwice(auditExport)

		By("writing the values the app is switched on by")
		configMap := &corev1.ConfigMap{}
		Eventually(func() error {
			return k8sClient.Get(ctx, configMapKey, configMap)
		}, timeout, interval).Should(Succeed())
		Expect(configMap.Data).To(HaveKey(logExportValuesKey))
		Expect(configMap.Data[logExportValuesKey]).To(ContainSubstring(auditBucket))

		By("owning nothing, so deleting one export cannot garbage-collect the shared object")
		Expect(configMap.OwnerReferences).To(BeEmpty())

		By("leaving the Secret absent, which is what valuesFrom declares as optional")
		Consistently(func() bool {
			err := k8sClient.Get(ctx, secretKey, &corev1.Secret{})
			return apierrors.IsNotFound(err)
		}, time.Second, interval).Should(BeTrue())
	})

	It("writes the credential Secret when an export uses credentialsRef", func() {
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "archive-credentials", Namespace: ns},
			Data: map[string][]byte{
				logExportAccessKeyIDKey:     []byte("AKIAEXAMPLE"),
				logExportSecretAccessKeyKey: []byte("s3cr3t"),
			},
		})).To(Succeed())

		export := newExport(auditExport, `{scrape_job="audit-logs"}`, observabilityv1alpha1.S3Destination{
			Bucket:         auditBucket,
			Region:         s3Region,
			CredentialsRef: &corev1.LocalObjectReference{Name: "archive-credentials"},
		})
		Expect(k8sClient.Create(ctx, export)).To(Succeed())

		reconcileTwice(auditExport)

		secret := &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, secretKey, secret)
		}, timeout, interval).Should(Succeed())
		Expect(secret.Data).To(HaveKey(logExportValuesKey))
		Expect(string(secret.Data[logExportValuesKey])).To(ContainSubstring("AKIAEXAMPLE"))
		Expect(secret.OwnerReferences).To(BeEmpty())
	})

	It("keeps the other exports when one of several is deleted", func() {
		audit := newExport(auditExport, `{scrape_job="audit-logs"}`, observabilityv1alpha1.S3Destination{
			Bucket: auditBucket, Region: s3Region, RoleARN: "arn:aws:iam::123456789012:role/a",
		})
		teleport := newExport("teleport", `{scrape_job="teleport.giantswarm.io"}`, observabilityv1alpha1.S3Destination{
			Bucket: "teleport-archive", Region: s3Region, RoleARN: "arn:aws:iam::123456789012:role/b",
		})
		Expect(k8sClient.Create(ctx, audit)).To(Succeed())
		Expect(k8sClient.Create(ctx, teleport)).To(Succeed())

		reconcileTwice(auditExport)
		reconcileTwice("teleport")

		By("rendering both into the one ConfigMap")
		configMap := &corev1.ConfigMap{}
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, configMapKey, configMap); err != nil {
				return false
			}
			values := configMap.Data[logExportValuesKey]
			return strings.Contains(values, auditBucket) && strings.Contains(values, "teleport-archive")
		}, timeout, interval).Should(BeTrue())

		By("dropping only the deleted one, and keeping the ConfigMap")
		deleteAndReconcile(audit)

		Eventually(func() (string, error) {
			if err := k8sClient.Get(ctx, configMapKey, configMap); err != nil {
				return "", err
			}
			return configMap.Data[logExportValuesKey], nil
		}, timeout, interval).ShouldNot(ContainSubstring(auditBucket))
		Expect(configMap.Data[logExportValuesKey]).To(ContainSubstring("teleport-archive"))
	})

	It("removes both objects when the last export goes, returning the app to inert", func() {
		export := newExport(auditExport, `{scrape_job="audit-logs"}`, observabilityv1alpha1.S3Destination{
			Bucket: auditBucket, Region: s3Region, RoleARN: "arn:aws:iam::123456789012:role/a",
		})
		Expect(k8sClient.Create(ctx, export)).To(Succeed())
		reconcileTwice(auditExport)

		Eventually(func() error {
			return k8sClient.Get(ctx, configMapKey, &corev1.ConfigMap{})
		}, timeout, interval).Should(Succeed())

		deleteAndReconcile(export)

		By("removing the ConfigMap so the HelmRelease falls back to the defaults")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, configMapKey, &corev1.ConfigMap{})
			return apierrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())

		By("removing the finalizer so the resource can go")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: auditExport}, &observabilityv1alpha1.LogExport{})
			return apierrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
	})

	It("refuses two exports that both carry static credentials", func() {
		// Static credentials reach the exporter as process environment, which is
		// per-container, so only one export can carry them.
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
			Data: map[string][]byte{
				logExportAccessKeyIDKey:     []byte("AKIAEXAMPLE"),
				logExportSecretAccessKeyKey: []byte("s3cr3t"),
			},
		})).To(Succeed())

		for _, name := range []string{auditExport, "teleport"} {
			export := newExport(name, fmt.Sprintf(`{scrape_job=%q}`, name), observabilityv1alpha1.S3Destination{
				Bucket:         name + "-export",
				Region:         s3Region,
				CredentialsRef: &corev1.LocalObjectReference{Name: "creds"},
			})
			Expect(k8sClient.Create(ctx, export)).To(Succeed())
		}

		// First reconcile adds the finalizer and succeeds; the render then fails.
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: auditExport}})
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: auditExport}})
		Expect(err).To(MatchError(ContainSubstring("cannot be set per destination")))
	})

	It("reports a missing credentials Secret rather than rendering without it", func() {
		export := newExport(auditExport, `{scrape_job="audit-logs"}`, observabilityv1alpha1.S3Destination{
			Bucket:         auditBucket,
			Region:         s3Region,
			CredentialsRef: &corev1.LocalObjectReference{Name: "does-not-exist"},
		})
		Expect(k8sClient.Create(ctx, export)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: auditExport}})
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: auditExport}})
		Expect(err).To(MatchError(ContainSubstring("failed to get credentials secret")))

		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, configMapKey, &corev1.ConfigMap{}))).To(BeTrue())
	})
})
