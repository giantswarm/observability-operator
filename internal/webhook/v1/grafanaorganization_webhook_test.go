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

package v1

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

var _ = Describe("GrafanaOrganization Webhook", func() {
	var (
		ctx       context.Context
		obj       *observabilityv1alpha1.GrafanaOrganization
		oldObj    *observabilityv1alpha1.GrafanaOrganization
		validator GrafanaOrganizationValidator
	)

	BeforeEach(func() {
		ctx = context.Background()
		obj = &observabilityv1alpha1.GrafanaOrganization{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-org",
			},
			Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
				DisplayName: "Test Organization",
				Tenants:     []observabilityv1alpha1.TenantID{"test-tenant"},
			},
		}
		oldObj = &observabilityv1alpha1.GrafanaOrganization{}
		validator = GrafanaOrganizationValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// Cleanup if needed
	})

	Context("When creating or updating GrafanaOrganization under Validating Webhook", func() {
		It("Should reject empty tenant IDs", func() {
			By("Testing empty tenant ID")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{""}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenant ID cannot be empty"))
		})

		It("Should reject duplicate tenant IDs", func() {
			By("Testing duplicate tenant IDs")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"tenant1", "tenant2", "tenant1"}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate tenant ID \"tenant1\" found"))
		})

		It("Should reject tenant IDs with leading/trailing whitespace", func() {
			By("Testing tenant ID with leading whitespace")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{" tenant-with-space"}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("contains leading or trailing whitespace"))

			By("Testing tenant ID with trailing whitespace")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"tenant-with-space "}
			_, err = validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("contains leading or trailing whitespace"))
		})

		It("Should allow valid tenant IDs with various allowed characters", func() {
			By("Testing alphanumeric characters")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"tenant123", "TENANT456", "MixedCase789"}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("Testing special characters")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"tenant-with-dash", "tenant_with_underscore", "tenant.with.dots"}
			_, err = validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("Testing more special characters")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"tenant!exclamation", "tenant*asterisk", "tenant'quote", "tenant(paren)", "tenant)paren"}
			_, err = validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject forbidden tenant ID values", func() {
			By("Testing single dot")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"."}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenant ID \".\" is not allowed"))

			By("Testing double dot")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{".."}
			_, err = validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenant ID \"..\" is not allowed"))

			By("Testing mimir cluster reserved name")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"__mimir_cluster"}
			_, err = validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenant ID \"__mimir_cluster\" is not allowed"))
		})

		It("Should allow valid tenant IDs mixed with forbidden ones and reject appropriately", func() {
			By("Testing a mix where one is forbidden")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"valid-tenant", "."}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenant ID \".\" is not allowed"))
		})

		It("Should validate on updates as well as creates", func() {
			By("Testing update with valid tenant IDs")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{"valid-tenant-update"}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())

			By("Testing update with forbidden tenant ID")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{".."}
			_, err = validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenant ID \"..\" is not allowed"))
		})

		It("Should allow deletion without validation", func() {
			By("Testing delete operation")
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should handle edge cases gracefully", func() {
			By("Testing empty tenant list")
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("Testing long valid tenant ID (up to 150 chars)")
			longTenantStr := strings.Repeat("a", 150) // 150 chars total
			longTenant := observabilityv1alpha1.TenantID(longTenantStr)
			obj.Spec.Tenants = []observabilityv1alpha1.TenantID{longTenant}
			_, err = validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When testing TenantID pattern validation (simulating kubebuilder validation)", func() {
		// Helper function to simulate the kubebuilder pattern validation
		isValidTenantIDPattern := func(tenantID string) bool {
			if len(tenantID) == 0 || len(tenantID) > 150 {
				return false
			}

			for _, r := range tenantID {
				// Alphanumeric characters
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
					continue
				}
				// Special characters allowed by our pattern: ^[a-zA-Z0-9!._*'()-]+$
				switch r {
				case '!', '.', '_', '*', '\'', '(', ')', '-':
					continue
				default:
					return false
				}
			}
			return true
		}

		It("Should allow all valid TenantID patterns according to Grafana Mimir spec", func() {
			validTenantIDs := []struct {
				id          string
				description string
			}{
				{"tenant123", "lowercase alphanumeric"},
				{"TENANT123", "uppercase alphanumeric"},
				{"TeNaNt123", "mixed case alphanumeric"},
				{"tenant-with-dash", "contains dash character"},
				{"tenant_with_underscore", "contains underscore character"},
				{"tenant.with.dots", "contains dot character"},
				{"tenant!exclamation", "contains exclamation character"},
				{"tenant*asterisk", "contains asterisk character"},
				{"tenant'quote", "contains single quote character"},
				{"tenant(with)parentheses", "contains parentheses characters"},
				{"tenant!-_.*'()", "contains all allowed special characters"},
				{"giantswarm", "example from kubebuilder annotation"},
				{"my-tenant-123", "typical tenant name with dash and numbers"},
				{"PROD_ENVIRONMENT", "uppercase with underscore"},
				{"dev.staging.test", "dots for environment separation"},
				{strings.Repeat("a", 150), "150 characters long (max allowed)"},
			}

			for _, tc := range validTenantIDs {
				By("Testing valid tenant ID: " + tc.description)
				Expect(isValidTenantIDPattern(tc.id)).To(BeTrue(),
					"TenantID %q should be valid for %s", tc.id, tc.description)

				// Also test with the webhook validator (should pass webhook validation too)
				obj.Spec.Tenants = []observabilityv1alpha1.TenantID{observabilityv1alpha1.TenantID(tc.id)}
				_, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred(),
					"TenantID %q should pass webhook validation for %s", tc.id, tc.description)
			}
		})

		It("Should reject invalid TenantID patterns that would fail kubebuilder validation", func() {
			invalidTenantIDs := []struct {
				id          string
				description string
			}{
				{"tenant/with/slash", "contains forward slash (not allowed)"},
				{"tenant\\with\\backslash", "contains backward slash (not allowed)"},
				{"tenant with space", "contains space (not allowed)"},
				{"tenant@symbol", "contains @ symbol (not allowed)"},
				{"tenant#hash", "contains # symbol (not allowed)"},
				{"tenant$dollar", "contains $ symbol (not allowed)"},
				{"tenant%percent", "contains % symbol (not allowed)"},
				{"tenant^caret", "contains ^ symbol (not allowed)"},
				{"tenant&ampersand", "contains & symbol (not allowed)"},
				{"tenant=equals", "contains = symbol (not allowed)"},
				{"tenant+plus", "contains + symbol (not allowed)"},
				{"tenant[bracket]", "contains square brackets (not allowed)"},
				{"tenant{brace}", "contains curly braces (not allowed)"},
				{"tenant|pipe", "contains pipe symbol (not allowed)"},
				{"tenant:colon", "contains colon (not allowed)"},
				{"tenant;semicolon", "contains semicolon (not allowed)"},
				{"tenant\"quote", "contains double quote (not allowed)"},
				{"tenant<less>", "contains angle brackets (not allowed)"},
				{"tenant?question", "contains question mark (not allowed)"},
				{"tenant,comma", "contains comma (not allowed)"},
				{"", "empty string (below minimum length)"},
				{strings.Repeat("a", 151), "151 characters long (exceeds max length)"},
			}

			for _, tc := range invalidTenantIDs {
				By("Testing invalid tenant ID: " + tc.description)
				Expect(isValidTenantIDPattern(tc.id)).To(BeFalse(),
					"TenantID %q should be invalid for %s", tc.id, tc.description)
			}
		})

		It("Should correctly validate length constraints", func() {
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
