/*
Copyright 2025.

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

package v1alpha2

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
)

// GrafanaOrganizationWebhookSpecs returns the Describe blocks for testing the GrafanaOrganization webhook
var _ = Describe("GrafanaOrganization V1Alpha2 Validation", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("When testing full validation pipeline (CRD + Webhook)", func() {
		It("Should reject tenant IDs with invalid patterns (CRD validation)", func() {
			testCases := []struct {
				name     string
				tenantID string
				reason   string
			}{
				{"hyphens", "tenant-with-hyphens", "hyphens not allowed in Alloy identifiers"},
				{"dots", "tenant.with.dots", "dots not allowed in Alloy identifiers"},
				{"spaces", "tenant with space", "spaces not allowed"},
				{"starts_with_number", "123tenant", "must start with letter or underscore"},
				{"special_chars", "tenant@symbol", "special characters not allowed"},
			}

			for _, tc := range testCases {
				By("Testing " + tc.name + ": " + tc.reason)
				grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org-invalid-" + tc.name,
					},
					Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
						DisplayName: "Test Organization",
						Tenants: []observabilityv1alpha2.TenantConfig{
							{
								Name:  observabilityv1alpha2.TenantID(tc.tenantID),
								Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
							},
						},
						RBAC: &observabilityv1alpha2.RBAC{
							Admins: []string{"admin-org"},
						},
					},
				}

				err := k8sClient.Create(ctx, grafanaOrg)
				Expect(err).To(HaveOccurred(), "Tenant ID %q should be rejected by CRD validation", tc.tenantID)
				Expect(err.Error()).To(ContainSubstring("should match"), "Error should mention pattern validation")
			}
		})

		It("Should reject tenant IDs with invalid lengths (CRD validation)", func() {
			By("Testing empty tenant ID")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-empty",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name:  "",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
						},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("should be at least 1 chars long"))

			By("Testing tenant ID that's too long (151 chars)")
			tooLongTenant := "a" + strings.Repeat("b", 150) // 151 characters total
			grafanaOrg.ObjectMeta.Name = "test-org-too-long"
			grafanaOrg.Spec.Tenants = []observabilityv1alpha2.TenantConfig{
				{
					Name:  observabilityv1alpha2.TenantID(tooLongTenant),
					Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
				},
			}

			err = k8sClient.Create(ctx, grafanaOrg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Too long"))
		})

		It("Should reject forbidden tenant values (Webhook validation)", func() {
			By("Testing __mimir_cluster (passes CRD pattern but forbidden by Mimir)")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-forbidden",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name:  "__mimir_cluster",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
						},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("__mimir_cluster\" is not allowed"))
		})

		It("Should reject duplicate tenant IDs (Webhook validation)", func() {
			By("Testing duplicate tenant IDs")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-duplicates",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name:  "valid_tenant",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
						},
						{
							Name:  "valid_tenant", // Duplicate!
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting},
						},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate tenant ID"))
		})

		It("Should reject tenant configs with empty types (Webhook validation)", func() {
			By("Testing tenant config with empty types array")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-empty-types",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name:  "valid_tenant",
							Types: []observabilityv1alpha2.TenantType{}, // Empty types!
						},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must have at least one type specified"))
		})

		It("Should allow valid tenant configurations", func() {
			testCases := []struct {
				name    string
				tenants []observabilityv1alpha2.TenantConfig
				reason  string
			}{
				{
					"single_data_tenant",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "tenant123", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					"basic data tenant",
				},
				{
					"single_alerting_tenant",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "alert_tenant", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting}},
					},
					"basic alerting tenant",
				},
				{
					"mixed_type_tenant",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "mixed_tenant", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData, observabilityv1alpha2.TenantTypeAlerting}},
					},
					"tenant with both data and alerting types",
				},
				{
					"multiple_tenants",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "prod_data", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
						{Name: "prod_alerts", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting}},
					},
					"multiple unique tenants with different types",
				},
				{
					"underscores_and_long_name",
					[]observabilityv1alpha2.TenantConfig{
						{Name: observabilityv1alpha2.TenantID("a" + strings.Repeat("b", 149)), Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					"150 characters with underscores (max length)",
				},
			}

			for i, tc := range testCases {
				By("Testing " + tc.name + ": " + tc.reason)

				grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("test-org-valid-%s-%d", strings.ReplaceAll(tc.name, "_", "-"), i),
					},
					Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
						DisplayName: "Test Organization",
						Tenants:     tc.tenants,
						RBAC: &observabilityv1alpha2.RBAC{
							Admins: []string{"admin-org"},
						},
					},
				}

				err := k8sClient.Create(ctx, grafanaOrg)
				Expect(err).NotTo(HaveOccurred(), "Valid tenant config %+v should be accepted", tc.tenants)

				// Clean up immediately
				_ = k8sClient.Delete(ctx, grafanaOrg)
			}
		})

		It("Should allow updates with valid changes", func() {
			By("Creating a valid GrafanaOrganization")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-update",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{Name: "initial_tenant", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				_ = k8sClient.Delete(ctx, grafanaOrg)
			}()

			By("Updating with valid tenant changes")
			grafanaOrg.Spec.Tenants = []observabilityv1alpha2.TenantConfig{
				{Name: "updated_tenant", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData, observabilityv1alpha2.TenantTypeAlerting}},
				{Name: "another_tenant", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting}},
			}
			err = k8sClient.Update(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())

			By("Attempting update with invalid tenant (should fail)")
			grafanaOrg.Spec.Tenants = []observabilityv1alpha2.TenantConfig{
				{Name: "__mimir_cluster", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
			}
			err = k8sClient.Update(ctx, grafanaOrg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("__mimir_cluster\" is not allowed"))
		})

		It("Should allow deletion without validation", func() {
			By("Creating and then deleting a GrafanaOrganization")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-delete",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{Name: "delete_test_tenant", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When demonstrating tenant type validation", func() {
		It("Should accept all valid tenant type combinations", func() {
			validTypeCombinations := []struct {
				name  string
				types []observabilityv1alpha2.TenantType
			}{
				{"data_only", []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
				{"alerting_only", []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting}},
				{"both_types", []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData, observabilityv1alpha2.TenantTypeAlerting}},
				{"duplicate_order", []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting, observabilityv1alpha2.TenantTypeData}},
			}

			for i, tc := range validTypeCombinations {
				By("Testing tenant type combination: " + tc.name)
				grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("test-org-types-%s-%d", strings.ReplaceAll(tc.name, "_", "-"), i),
					},
					Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
						DisplayName: "Test Organization",
						Tenants: []observabilityv1alpha2.TenantConfig{
							{Name: "test_tenant", Types: tc.types},
						},
						RBAC: &observabilityv1alpha2.RBAC{
							Admins: []string{"admin-org"},
						},
					},
				}

				err := k8sClient.Create(ctx, grafanaOrg)
				Expect(err).NotTo(HaveOccurred(), "Valid tenant type combination %v should be accepted", tc.types)

				// Clean up immediately
				_ = k8sClient.Delete(ctx, grafanaOrg)
			}
		})
	})
})
