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

package webhook

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
)

var _ = Describe("GrafanaOrganization v1alpha2 Validation", func() {
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
						Name: "test-org-" + tc.name,
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
				Expect(err).To(HaveOccurred(), "Should reject invalid tenant pattern: "+tc.reason)
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
							Name:  "valid_tenant", // Duplicate
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

		It("Should validate tenant types correctly", func() {
			By("Testing valid tenant types")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-valid-types",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name:  "data_tenant",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
						},
						{
							Name:  "alerting_tenant",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting},
						},
						{
							Name:  "both_tenant",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData, observabilityv1alpha2.TenantTypeAlerting},
						},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())

			// Clean up
			defer func() {
				_ = k8sClient.Delete(ctx, grafanaOrg)
			}()
		})

		It("Should allow valid tenant IDs", func() {
			testCases := []struct {
				name    string
				tenants []observabilityv1alpha2.TenantConfig
				reason  string
			}{
				{
					"single_tenant",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "tenant123", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					"basic alphanumeric",
				},
				{
					"underscores",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "tenant_with_underscore", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					"underscores allowed",
				},
				{
					"starts_with_underscore",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "_starts_with_underscore", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					"can start with underscore",
				},
				{
					"mixed_case",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "MixedCase123", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					"mixed case allowed",
				},
				{
					"long_name",
					[]observabilityv1alpha2.TenantConfig{
						{Name: observabilityv1alpha2.TenantID("a" + strings.Repeat("b", 149)), Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
					},
					"150 characters (max length)",
				},
				{
					"multiple_valid",
					[]observabilityv1alpha2.TenantConfig{
						{Name: "prod_env", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData}},
						{Name: "staging_env", Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting}},
					},
					"multiple unique tenants",
				},
			}

			for i, tc := range testCases {
				By(fmt.Sprintf("Testing %s: %s", tc.name, tc.reason))
				grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("test-org-valid-%d", i),
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
				Expect(err).NotTo(HaveOccurred(), "Should allow valid tenant: "+tc.reason)

				// Clean up immediately
				_ = k8sClient.Delete(ctx, grafanaOrg)
			}
		})

		It("Should allow updates with valid changes", func() {
			By("Creating a valid GrafanaOrganization v1alpha2")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-update",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name:  "initial_tenant",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
						},
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
				{
					Name:  "updated_tenant",
					Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
				},
				{
					Name:  "another_tenant",
					Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeAlerting},
				},
			}
			err = k8sClient.Update(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())

			By("Attempting update with invalid tenant (should fail)")
			grafanaOrg.Spec.Tenants = []observabilityv1alpha2.TenantConfig{
				{
					Name:  "__mimir_cluster",
					Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
				},
			}
			err = k8sClient.Update(ctx, grafanaOrg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("__mimir_cluster\" is not allowed"))
		})

		It("Should allow deletion without validation", func() {
			By("Creating and then deleting a GrafanaOrganization v1alpha2")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-delete",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name:  "delete_test_tenant",
							Types: []observabilityv1alpha2.TenantType{observabilityv1alpha2.TenantTypeData},
						},
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

		It("Should handle default tenant types correctly", func() {
			By("Creating tenant without explicit types (should default to data)")
			grafanaOrg := &observabilityv1alpha2.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org-default-types",
				},
				Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants: []observabilityv1alpha2.TenantConfig{
						{
							Name: "default_tenant",
							// Types omitted, should default to ["data"]
						},
					},
					RBAC: &observabilityv1alpha2.RBAC{
						Admins: []string{"admin-org"},
					},
				},
			}

			err := k8sClient.Create(ctx, grafanaOrg)
			Expect(err).NotTo(HaveOccurred())

			// Clean up
			defer func() {
				_ = k8sClient.Delete(ctx, grafanaOrg)
			}()
		})
	})

	Context("When demonstrating what CRD pattern validation covers", func() {
		// Helper function to simulate the kubebuilder pattern validation for Alloy-compatible names
		isValidTenantIDPattern := func(tenantID string) bool {
			if len(tenantID) == 0 || len(tenantID) > 150 {
				return false
			}

			// Must start with a letter or underscore (following Alloy identifier rules)
			if len(tenantID) > 0 {
				first := rune(tenantID[0])
				if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
					return false
				}
			}

			for _, r := range tenantID {
				// Only alphanumeric characters and underscore allowed (no hyphens in Alloy)
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
					return false
				}
			}
			return true
		}

		It("Should show which patterns are valid according to CRD validation", func() {
			validTenantIDs := []struct {
				id          string
				description string
			}{
				{"tenant123", "lowercase alphanumeric"},
				{"TENANT123", "uppercase alphanumeric"},
				{"TeNaNt123", "mixed case alphanumeric"},
				{"tenant_with_underscore", "contains underscore character"},
				{"_starts_with_underscore", "starts with underscore"},
				{"giantswarm", "example from kubebuilder annotation"},
				{"my_tenant_123", "typical tenant name with underscore and numbers"},
				{"PROD_ENVIRONMENT", "uppercase with underscore"},
				{"dev_staging_test", "underscores for environment separation"},
				{"a" + strings.Repeat("b", 149), "150 characters long (max allowed)"},
			}

			for _, tc := range validTenantIDs {
				By(fmt.Sprintf("Testing valid pattern: %s (%s)", tc.id, tc.description))
				Expect(isValidTenantIDPattern(tc.id)).To(BeTrue(), "Should be valid: "+tc.description)
			}
		})

		It("Should show which patterns are invalid according to CRD validation", func() {
			invalidTenantIDs := []struct {
				id          string
				description string
			}{
				{"1tenant", "starts with number (invalid Alloy identifier)"},
				{"123abc", "starts with number (invalid Alloy identifier)"},
				{"tenant-with-dash", "contains dash (invalid Alloy identifier)"},
				{"tenant.with.dots", "contains dots (invalid Alloy identifier)"},
				{"tenant with space", "contains space (not allowed)"},
				{"tenant@symbol", "contains @ symbol (not allowed)"},
				{"tenant!exclamation", "contains special characters (not allowed)"},
				{"", "empty string (below minimum length)"},
				{strings.Repeat("a", 151), "151 characters long (exceeds max length)"},
			}

			for _, tc := range invalidTenantIDs {
				By(fmt.Sprintf("Testing invalid pattern: %s (%s)", tc.id, tc.description))
				Expect(isValidTenantIDPattern(tc.id)).To(BeFalse(), "Should be invalid: "+tc.description)
			}
		})

		It("Should validate length constraints", func() {
			By("Testing minimum length boundary")
			Expect(isValidTenantIDPattern("a")).To(BeTrue(), "Single character should be valid")

			By("Testing maximum length boundary")
			maxLengthTenant := strings.Repeat("a", 150)
			Expect(isValidTenantIDPattern(maxLengthTenant)).To(BeTrue(), "150 character tenant should be valid")

			exceedsMaxTenant := strings.Repeat("a", 151)
			Expect(isValidTenantIDPattern(exceedsMaxTenant)).To(BeFalse(), "151 character tenant should be invalid")
		})
	})
})
