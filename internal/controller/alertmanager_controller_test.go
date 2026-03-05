package controller

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
)

// mockAlertmanagerService is a test double for alertmanager.Service.
type mockAlertmanagerService struct {
	configureCallCount int
	configureTenantID  string
	configureErr       error

	deleteCallCount int
	deleteTenantID  string
	deleteErr       error
}

func (m *mockAlertmanagerService) ConfigureFromSecret(_ context.Context, _ *v1.Secret, tenantID string) error {
	m.configureCallCount++
	m.configureTenantID = tenantID
	return m.configureErr
}

func (m *mockAlertmanagerService) DeleteForTenant(_ context.Context, tenantID string) error {
	m.deleteCallCount++
	m.deleteTenantID = tenantID
	return m.deleteErr
}

var _ = Describe("Alertmanager Controller", func() {
	const (
		secretName      = "test-alertmanager-config"
		secretNamespace = "default"
		testTenant      = "test_tenant"
		timeout         = time.Second * 10
		interval        = time.Millisecond * 250
	)

	var (
		testCtx        context.Context
		secret         *v1.Secret
		grafanaOrg     *observabilityv1alpha1.GrafanaOrganization
		namespacedName types.NamespacedName
		svc            *mockAlertmanagerService
		reconciler     AlertmanagerReconciler
	)

	newSecret := func(withFinalizer bool) *v1.Secret {
		s := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Labels: map[string]string{
					"observability.giantswarm.io/kind": "alertmanager-config",
					tenancy.TenantSelectorLabel:        testTenant,
				},
			},
			Data: map[string][]byte{
				alertmanager.AlertmanagerConfigKey: []byte("route:\n  receiver: noop\nreceivers:\n- name: noop\n"),
			},
		}
		if withFinalizer {
			controllerutil.AddFinalizer(s, alertmanager.AlertmanagerConfigFinalizer)
		}
		return s
	}

	buildReconciler := func() AlertmanagerReconciler {
		return AlertmanagerReconciler{
			client:              k8sClient,
			alertmanagerService: svc,
			tenantRepository:    tenancy.NewTenantRepository(k8sClient),
			finalizerHelper:     NewFinalizerHelper(k8sClient, alertmanager.AlertmanagerConfigFinalizer),
		}
	}

	doReconcile := func() (reconcile.Result, error) {
		return reconciler.Reconcile(testCtx, reconcile.Request{NamespacedName: namespacedName})
	}

	BeforeEach(func() {
		testCtx = context.Background()
		namespacedName = types.NamespacedName{Name: secretName, Namespace: secretNamespace}
		svc = &mockAlertmanagerService{}

		// Create the GrafanaOrganization that exposes testTenant in the TenantRepository.
		grafanaOrg = &observabilityv1alpha1.GrafanaOrganization{
			ObjectMeta: metav1.ObjectMeta{Name: "test-alertmanager-org"},
			Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
				DisplayName: "Test Alertmanager Org",
				RBAC:        &observabilityv1alpha1.RBAC{Admins: []string{"admin"}},
				Tenants:     []observabilityv1alpha1.TenantID{observabilityv1alpha1.TenantID(testTenant)},
			},
		}
		err := k8sClient.Create(testCtx, grafanaOrg)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		// Clean up the secret (remove finalizers first if present).
		existing := &v1.Secret{}
		err := k8sClient.Get(testCtx, namespacedName, existing)
		if err == nil {
			existing.Finalizers = []string{}
			_ = k8sClient.Update(testCtx, existing)
			_ = k8sClient.Delete(testCtx, existing)
		}

		// Clean up the GrafanaOrganization.
		if grafanaOrg != nil {
			err := k8sClient.Delete(testCtx, grafanaOrg)
			if err != nil && !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	Context("Create path", func() {
		It("should add a finalizer on the first reconcile and not call ConfigureFromSecret", func() {
			secret = newSecret(false)
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())
			reconciler = buildReconciler()

			result, err := doReconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			// Finalizer must be present after the first reconcile.
			updated := &v1.Secret{}
			Expect(k8sClient.Get(testCtx, namespacedName, updated)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(updated, alertmanager.AlertmanagerConfigFinalizer)).To(BeTrue())

			// ConfigureFromSecret must NOT have been called — we returned early.
			Expect(svc.configureCallCount).To(Equal(0))
		})

		It("should call ConfigureFromSecret when finalizer is already present", func() {
			secret = newSecret(true)
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())
			reconciler = buildReconciler()

			result, err := doReconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(svc.configureCallCount).To(Equal(1))
			Expect(svc.configureTenantID).To(Equal(testTenant))
		})

		It("should skip ConfigureFromSecret when tenant is not in the tenant list", func() {
			// Delete the GrafanaOrg so the tenant list is empty.
			Expect(k8sClient.Delete(testCtx, grafanaOrg)).To(Succeed())
			grafanaOrg = nil

			secret = newSecret(true)
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())
			reconciler = buildReconciler()

			_, err := doReconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.configureCallCount).To(Equal(0))
		})

		It("should return the error from ConfigureFromSecret", func() {
			svc.configureErr = errors.New("mimir unreachable")
			secret = newSecret(true)
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())
			reconciler = buildReconciler()

			_, err := doReconcile()
			Expect(err).To(MatchError(ContainSubstring("mimir unreachable")))
		})
	})

	Context("Delete path", func() {
		It("should call DeleteForTenant and remove the finalizer on success", func() {
			secret = newSecret(true)
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())
			reconciler = buildReconciler()

			// Trigger deletion — sets DeletionTimestamp, object stays alive because of finalizer.
			Expect(k8sClient.Delete(testCtx, secret)).To(Succeed())

			result, err := doReconcile()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			Expect(svc.deleteCallCount).To(Equal(1))
			Expect(svc.deleteTenantID).To(Equal(testTenant))
			Expect(svc.configureCallCount).To(Equal(0))

			// After the finalizer is removed the API server GCs the object.
			// Either it is already gone or the finalizer is absent.
			updated := &v1.Secret{}
			getErr := k8sClient.Get(testCtx, namespacedName, updated)
			if getErr == nil {
				Expect(controllerutil.ContainsFinalizer(updated, alertmanager.AlertmanagerConfigFinalizer)).To(BeFalse())
			} else {
				Expect(apierrors.IsNotFound(getErr)).To(BeTrue(), "unexpected error: %v", getErr)
			}
		})

		It("should keep the finalizer when DeleteForTenant returns an error", func() {
			svc.deleteErr = errors.New("mimir unavailable")
			secret = newSecret(true)
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())
			reconciler = buildReconciler()

			Expect(k8sClient.Delete(testCtx, secret)).To(Succeed())

			_, err := doReconcile()
			Expect(err).To(MatchError(ContainSubstring("mimir unavailable")))

			// Finalizer must remain because cleanup failed.
			updated := &v1.Secret{}
			Expect(k8sClient.Get(testCtx, namespacedName, updated)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(updated, alertmanager.AlertmanagerConfigFinalizer)).To(BeTrue())
		})

		It("should remove the finalizer even when the tenant label is missing", func() {
			// Secret has finalizer but no tenant label.
			secret = &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: secretNamespace,
					Labels: map[string]string{
						"observability.giantswarm.io/kind": "alertmanager-config",
					},
					Finalizers: []string{alertmanager.AlertmanagerConfigFinalizer},
				},
				Data: map[string][]byte{
					alertmanager.AlertmanagerConfigKey: []byte("route:\n  receiver: noop\nreceivers:\n- name: noop\n"),
				},
			}
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())
			reconciler = buildReconciler()

			Expect(k8sClient.Delete(testCtx, secret)).To(Succeed())

			_, err := doReconcile()
			Expect(err).NotTo(HaveOccurred())

			// No Mimir call (no tenant to clean up).
			Expect(svc.deleteCallCount).To(Equal(0))

			// After the finalizer is removed the API server GCs the object.
			noLabelUpdated := &v1.Secret{}
			getErr := k8sClient.Get(testCtx, namespacedName, noLabelUpdated)
			if getErr == nil {
				Expect(controllerutil.ContainsFinalizer(noLabelUpdated, alertmanager.AlertmanagerConfigFinalizer)).To(BeFalse())
			} else {
				Expect(apierrors.IsNotFound(getErr)).To(BeTrue(), "unexpected error: %v", getErr)
			}
		})
	})

	Context("Edge cases", func() {
		It("should return nil for a non-existent secret", func() {
			reconciler = buildReconciler()

			_, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "does-not-exist", Namespace: secretNamespace},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.configureCallCount).To(Equal(0))
			Expect(svc.deleteCallCount).To(Equal(0))
		})
	})

	// Retain the original smoke test that was in this file.
	Context("Basic object lifecycle", func() {
		It("should successfully create and delete an Alertmanager config Secret", func() {
			By("Creating a new Secret with Alertmanager configuration")
			secret = &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "smoke-alertmanager-config",
					Namespace: secretNamespace,
					Labels: map[string]string{
						"observability.giantswarm.io/kind": "alertmanager-config",
						tenancy.TenantSelectorLabel:        testTenant,
					},
				},
				Data: map[string][]byte{
					alertmanager.AlertmanagerConfigKey: []byte(`
route:
  receiver: 'web.hook'
receivers:
- name: 'web.hook'
  webhook_configs:
  - url: 'http://127.0.0.1:5001/'
`),
				},
			}
			Expect(k8sClient.Create(testCtx, secret)).To(Succeed())

			By("Checking that the Secret was created")
			smokeName := types.NamespacedName{Name: "smoke-alertmanager-config", Namespace: secretNamespace}
			created := &v1.Secret{}
			Eventually(func() bool {
				return k8sClient.Get(testCtx, smokeName, created) == nil
			}, timeout, interval).Should(BeTrue())

			Expect(created.Data).To(HaveKey(alertmanager.AlertmanagerConfigKey))

			By("Cleaning up the Secret")
			Expect(k8sClient.Delete(testCtx, secret)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(testCtx, smokeName, &v1.Secret{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
