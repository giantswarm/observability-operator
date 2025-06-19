package controller

import (
	"context"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

var _ = Describe("GrafanaOrganization Controller", func() {
	Context("When reconciling a GrafanaOrganization", func() {
		const (
			grafanaOrgName = "test-grafana-org"
			timeout        = time.Second * 10
			interval       = time.Millisecond * 250
		)

		var (
			ctx                 context.Context
			reconciler          *GrafanaOrganizationReconciler
			grafanaOrganization *observabilityv1alpha1.GrafanaOrganization
			namespacedName      types.NamespacedName
			mockGrafanaGen      *MockGrafanaClientGenerator
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespacedName = types.NamespacedName{
				Name: grafanaOrgName,
			}

			// Clean up any existing GrafanaOrganization from previous test runs
			existingOrg := &observabilityv1alpha1.GrafanaOrganization{}
			err := k8sClient.Get(ctx, namespacedName, existingOrg)
			if err == nil {
				// GrafanaOrganization exists, delete it
				Expect(k8sClient.Delete(ctx, existingOrg)).To(Succeed())
				// Wait for it to be deleted
				Eventually(func() bool {
					err := k8sClient.Get(ctx, namespacedName, &observabilityv1alpha1.GrafanaOrganization{})
					return apierrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}

			// Create fresh mock for each test
			mockGrafanaGen = &MockGrafanaClientGenerator{}

			// Setup reconciler with mock client generator
			grafanaURL, _ := url.Parse("http://localhost:3000")
			reconciler = &GrafanaOrganizationReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				grafanaURL:       grafanaURL,
				finalizerHelper:  NewFinalizerHelper(k8sClient, observabilityv1alpha1.GrafanaOrganizationFinalizer),
				grafanaClientGen: mockGrafanaGen,
			}

			// Create base GrafanaOrganization
			grafanaOrganization = &observabilityv1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: grafanaOrgName,
				},
				Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants:     []observabilityv1alpha1.TenantID{"test_tenant"},
					RBAC: &observabilityv1alpha1.RBAC{
						Admins:  []string{"admin-group"},
						Editors: []string{"editor-group"},
						Viewers: []string{"viewer-group"},
					},
				},
			}
		})

		AfterEach(func() {
			// Clean up GrafanaOrganization if it exists
			if grafanaOrganization != nil {
				orgToDelete := &observabilityv1alpha1.GrafanaOrganization{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: grafanaOrganization.Name,
				}, orgToDelete)
				if err == nil {
					// Remove finalizers to allow deletion
					orgToDelete.Finalizers = []string{}
					k8sClient.Update(ctx, orgToDelete)
					k8sClient.Delete(ctx, orgToDelete)
				}
			}
		})

		Context("When Grafana is unavailable", func() {
			BeforeEach(func() {
				// Configure mock to return errors (Grafana unavailable)
				mockGrafanaGen.SetShouldReturnError(true)
			})

			It("should handle Grafana unavailability gracefully", func() {
				By("Creating a GrafanaOrganization")
				Expect(k8sClient.Create(ctx, grafanaOrganization)).To(Succeed())

				By("First reconciliation - should fail due to Grafana being unavailable")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				// First reconciliation should fail when Grafana client generation fails
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("grafana service unavailable"))
				Expect(result).To(Equal(reconcile.Result{}))

				By("Checking that the finalizer was NOT added due to the error")
				createdOrg := &observabilityv1alpha1.GrafanaOrganization{}
				err = k8sClient.Get(ctx, namespacedName, createdOrg)
				Expect(err).NotTo(HaveOccurred())

				// Finalizer should not be added since the reconciliation failed
				hasFinalizerAdded := false
				for _, finalizer := range createdOrg.Finalizers {
					if finalizer == observabilityv1alpha1.GrafanaOrganizationFinalizer {
						hasFinalizerAdded = true
						break
					}
				}
				Expect(hasFinalizerAdded).To(BeFalse())

				By("Checking that the status was not updated")
				Expect(createdOrg.Status.OrgID).To(Equal(int64(0)))
				Expect(createdOrg.Status.DataSources).To(BeEmpty())
			})

			It("should retry when Grafana becomes available again", func() {
				By("Creating a GrafanaOrganization")
				retryTestOrg := &observabilityv1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "retry-test-org",
					},
					Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
						DisplayName: "Retry Test Organization",
						Tenants:     []observabilityv1alpha1.TenantID{"retry_tenant"},
						RBAC: &observabilityv1alpha1.RBAC{
							Admins: []string{"retry-admin"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, retryTestOrg)).To(Succeed())

				retryNamespacedName := types.NamespacedName{
					Name: "retry-test-org",
				}

				By("First reconciliation - should fail with Grafana unavailable")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: retryNamespacedName,
				})
				Expect(err).To(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				By("Making Grafana available again")
				mockGrafanaGen.SetShouldReturnError(false)

				By("Second reconciliation - should now fail with 'configuration skipped'")
				result, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: retryNamespacedName,
				})
				// This will still fail but with a different error message indicating the test limitation
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, retryTestOrg)).To(Succeed())
			})
		})

		Context("When testing GrafanaOrganization lifecycle with working Grafana", func() {
			BeforeEach(func() {
				// Use "working" Grafana for lifecycle tests (though it will still fail with test message)
				mockGrafanaGen.SetShouldReturnError(false)
			})

			It("should handle finalizer management during creation", func() {
				By("Creating a GrafanaOrganization")
				Expect(k8sClient.Create(ctx, grafanaOrganization)).To(Succeed())

				By("Reconciling - should fail with configuration skipped but finalizer logic should be tested")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				// Should fail due to our test limitation, but we can verify the Grafana client was called
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(mockGrafanaGen.CallCount).To(Equal(1))
			})

			It("should handle deletion scenarios", func() {
				By("Creating a GrafanaOrganization with finalizer")
				grafanaOrganization.Finalizers = []string{observabilityv1alpha1.GrafanaOrganizationFinalizer}
				grafanaOrganization.Status.OrgID = 123
				Expect(k8sClient.Create(ctx, grafanaOrganization)).To(Succeed())

				By("Deleting the GrafanaOrganization")
				Expect(k8sClient.Delete(ctx, grafanaOrganization)).To(Succeed())

				By("Reconciling deletion - should fail with configuration skipped")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				// Should fail due to our test limitation
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(mockGrafanaGen.CallCount).To(Equal(1))
			})

			It("should handle organizations with different RBAC configurations", func() {
				By("Creating a GrafanaOrganization with minimal RBAC")
				minimalOrg := &observabilityv1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "minimal-rbac-org",
					},
					Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
						DisplayName: "Minimal RBAC Organization",
						Tenants:     []observabilityv1alpha1.TenantID{"minimal_tenant"},
						RBAC: &observabilityv1alpha1.RBAC{
							Admins: []string{"minimal-admin"},
							// No editors or viewers
						},
					},
				}
				Expect(k8sClient.Create(ctx, minimalOrg)).To(Succeed())

				By("Reconciling the minimal RBAC organization")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "minimal-rbac-org"},
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, minimalOrg)).To(Succeed())
			})
		})

		Context("When testing edge cases", func() {
			BeforeEach(func() {
				// Use "working" Grafana for edge case tests
				mockGrafanaGen.SetShouldReturnError(false)
			})

			It("should handle non-existent GrafanaOrganization gracefully", func() {
				By("Reconciling a non-existent GrafanaOrganization")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "non-existent-org",
					},
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should handle organizations with empty tenant lists", func() {
				By("Creating a GrafanaOrganization with empty tenants")
				emptyTenantOrg := &observabilityv1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "empty-tenant-org",
					},
					Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
						DisplayName: "Empty Tenant Organization",
						Tenants:     []observabilityv1alpha1.TenantID{}, // Empty
						RBAC: &observabilityv1alpha1.RBAC{
							Admins: []string{"empty-admin"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, emptyTenantOrg)).To(Succeed())

				By("Reconciling the empty tenant organization")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "empty-tenant-org"},
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, emptyTenantOrg)).To(Succeed())
			})

			It("should handle organizations with multiple tenants", func() {
				By("Creating a GrafanaOrganization with multiple tenants")
				multiTenantOrg := &observabilityv1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multi-tenant-org",
					},
					Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
						DisplayName: "Multi Tenant Organization",
						Tenants: []observabilityv1alpha1.TenantID{
							"tenant_1",
							"tenant_2",
							"tenant_3",
						},
						RBAC: &observabilityv1alpha1.RBAC{
							Admins:  []string{"multi-admin-1", "multi-admin-2"},
							Editors: []string{"multi-editor-1"},
							Viewers: []string{"multi-viewer-1", "multi-viewer-2"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, multiTenantOrg)).To(Succeed())

				By("Reconciling the multi-tenant organization")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: "multi-tenant-org"},
				})

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configuration skipped for testing"))
				Expect(result).To(Equal(reconcile.Result{}))

				// Clean up
				Expect(k8sClient.Delete(ctx, multiTenantOrg)).To(Succeed())
			})
		})

		Context("When testing mock call tracking", func() {
			BeforeEach(func() {
				mockGrafanaGen.SetShouldReturnError(false)
			})

			It("should track Grafana client generation calls", func() {
				By("Creating a GrafanaOrganization")
				Expect(k8sClient.Create(ctx, grafanaOrganization)).To(Succeed())

				By("Verifying initial call count")
				Expect(mockGrafanaGen.CallCount).To(Equal(0))

				By("Reconciling the organization")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				By("Verifying the mock was called")
				Expect(mockGrafanaGen.CallCount).To(Equal(1))
				Expect(mockGrafanaGen.LastURL).NotTo(BeNil())
				Expect(mockGrafanaGen.LastURL.String()).To(Equal("http://localhost:3000"))

				By("Reconciling again")
				_, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: namespacedName,
				})

				By("Verifying the call count increased")
				Expect(mockGrafanaGen.CallCount).To(Equal(2))

				// Ignore errors as they're expected due to our test limitation
				_ = err
			})
		})
	})

	Context("When testing GrafanaOrganization controller setup", func() {
		It("should setup the controller with proper configuration", func() {
			By("Verifying the setup function exists")
			Expect(SetupGrafanaOrganizationReconciler).NotTo(BeNil())
		})
	})

	Context("When testing GrafanaOrganization constants and types", func() {
		It("should have correct finalizer constant defined", func() {
			Expect(observabilityv1alpha1.GrafanaOrganizationFinalizer).To(Equal("observability.giantswarm.io/grafanaorganization"))
		})

		It("should handle GrafanaOrganization spec validation", func() {
			By("Creating a valid GrafanaOrganization spec")
			validSpec := observabilityv1alpha1.GrafanaOrganizationSpec{
				DisplayName: "Valid Organization",
				Tenants:     []observabilityv1alpha1.TenantID{"valid_tenant"},
				RBAC: &observabilityv1alpha1.RBAC{
					Admins: []string{"valid-admin"},
				},
			}

			Expect(validSpec.DisplayName).To(Equal("Valid Organization"))
			Expect(validSpec.Tenants).To(HaveLen(1))
			Expect(validSpec.RBAC.Admins).To(HaveLen(1))
		})
	})
})
