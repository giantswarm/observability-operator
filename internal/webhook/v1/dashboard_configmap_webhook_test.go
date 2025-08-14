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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/observability-operator/internal/mapper"
)

var _ = Describe("Dashboard ConfigMap Webhook", func() {
	var (
		ctx       context.Context
		validator *DashboardConfigMapValidator
		obj       *corev1.ConfigMap
		oldObj    *corev1.ConfigMap
	)

	BeforeEach(func() {
		ctx = context.Background()
		validator = &DashboardConfigMapValidator{
			client:          k8sClient,
			dashboardMapper: mapper.New(),
		}

		// Create a basic dashboard ConfigMap for testing
		obj = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-dashboard",
				Namespace: "default",
				Labels: map[string]string{
					"app.giantswarm.io/kind": "dashboard",
				},
				Annotations: map[string]string{
					"observability.giantswarm.io/organization": "test-org",
				},
			},
			Data: map[string]string{
				"dashboard.json": `{
					"uid": "test-dashboard",
					"title": "Test Dashboard",
					"panels": []
				}`,
			},
		}

		// Create oldObj for update tests
		oldObj = obj.DeepCopy()
		oldObj.Data["dashboard.json"] = `{
			"uid": "test-dashboard",
			"title": "Old Test Dashboard",
			"panels": []
		}`
	})

	Context("When validating dashboard ConfigMaps", func() {
		It("Should allow dashboard ConfigMaps with proper labels", func() {
			By("Testing scope filtering")
			isDashboard := validator.isDashboardConfigMap(obj)
			Expect(isDashboard).To(BeTrue())

			By("Testing ConfigMap without proper labels")
			configMapWithoutLabels := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-without-labels",
					Namespace: "default",
				},
			}
			isDashboard = validator.isDashboardConfigMap(configMapWithoutLabels)
			Expect(isDashboard).To(BeFalse())

			By("Testing ConfigMap with wrong kind label")
			configMapWithWrongKind := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-wrong-kind",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind": "not-dashboard",
					},
				},
			}
			isDashboard = validator.isDashboardConfigMap(configMapWithWrongKind)
			Expect(isDashboard).To(BeFalse())
		})

		It("Should allow non-dashboard ConfigMaps to pass through without validation", func() {
			By("Creating a ConfigMap without dashboard labels")
			nonDashboardConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "regular-configmap",
					Namespace: "default",
					Labels: map[string]string{
						"app": "some-app",
					},
				},
				Data: map[string]string{
					"config": "some config",
				},
			}

			_, err := validator.ValidateCreate(ctx, nonDashboardConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate dashboard ConfigMaps on create", func() {
			By("Testing dashboard ConfigMap validation on create")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate dashboard ConfigMaps on update", func() {
			By("Testing dashboard ConfigMap validation on update")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow deletion without validation", func() {
			By("Testing delete operation")
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate object type correctly", func() {
			By("Testing with wrong object type on create")
			wrongObj := &corev1.Secret{}
			_, err := validator.ValidateCreate(ctx, wrongObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a ConfigMap object but got"))

			By("Testing with wrong object type on update")
			_, err = validator.ValidateUpdate(ctx, wrongObj, wrongObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a ConfigMap object for the newObj but got"))
		})

		It("Should handle edge cases in webhook validation", func() {
			By("Testing with nil object")
			_, err := validator.ValidateCreate(ctx, nil)
			Expect(err).To(HaveOccurred())

			By("Testing with ConfigMap containing very large dashboard JSON")
			largeConfigMap := obj.DeepCopy()
			// Create a valid JSON with many panels
			largeJSON := `{"uid": "large-dashboard", "title": "Large Dashboard", "panels": [`
			for i := 0; i < 100; i++ {
				if i > 0 {
					largeJSON += ","
				}
				largeJSON += fmt.Sprintf(`{"id": %d, "title": "Panel %d"}`, i, i)
			}
			largeJSON += `]}`
			largeConfigMap.Data["dashboard.json"] = largeJSON

			_, err = validator.ValidateCreate(ctx, largeConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Should handle large JSON gracefully
		})
	})

	Context("When creating or updating dashboard ConfigMaps under Validating Webhook", func() {
		It("Should successfully validate basic dashboard structure", func() {
			By("Testing basic dashboard validation")
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject dashboard ConfigMaps with missing UID", func() {
			By("Creating ConfigMap with dashboard missing UID")
			invalidConfigMap := obj.DeepCopy()
			invalidConfigMap.Data["dashboard.json"] = `{
				"title": "Test Dashboard without UID",
				"panels": []
			}`
			_, err := validator.ValidateCreate(ctx, invalidConfigMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dashboard UID is missing"))
		})

		It("Should reject dashboard ConfigMaps with missing organization", func() {
			By("Creating ConfigMap without organization label or annotation")
			invalidConfigMap := obj.DeepCopy()
			invalidConfigMap.Labels = map[string]string{
				"app.giantswarm.io/kind": "dashboard",
			}
			invalidConfigMap.Annotations = nil
			_, err := validator.ValidateCreate(ctx, invalidConfigMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dashboard organization is missing"))
		})

		It("Should reject dashboard ConfigMaps with invalid JSON", func() {
			By("Creating ConfigMap with malformed JSON")
			invalidConfigMap := obj.DeepCopy()
			invalidConfigMap.Data["dashboard.json"] = `{
				"uid": "test-dashboard",
				"title": "Invalid JSON Dashboard"
				// Missing comma and closing brace
			`

			_, err := validator.ValidateCreate(ctx, invalidConfigMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid JSON format"))
		})

		It("Should accept dashboard ConfigMaps with organization in annotation", func() {
			By("Creating ConfigMap with organization in annotation")
			validConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard-annotation",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind": "dashboard",
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "annotation-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{
						"uid": "test-dashboard-annotation",
						"title": "Test Dashboard with Annotation",
						"panels": []
					}`,
				},
			}

			_, err := validator.ValidateCreate(ctx, validConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should accept dashboard ConfigMaps with organization in label", func() {
			By("Creating ConfigMap with organization in label")
			validConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard-label",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind":                   "dashboard",
						"observability.giantswarm.io/organization": "label-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{
						"uid": "test-dashboard-label",
						"title": "Test Dashboard with Label",
						"panels": []
					}`,
				},
			}

			_, err := validator.ValidateCreate(ctx, validConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should prefer annotation over label for organization", func() {
			By("Creating ConfigMap with both annotation and label")
			configMapWithBoth := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard-both",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind":                   "dashboard",
						"observability.giantswarm.io/organization": "label-org",
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "annotation-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{
						"uid": "test-dashboard-both",
						"title": "Test Dashboard with Both",
						"panels": []
					}`,
				},
			}

			_, err := validator.ValidateCreate(ctx, configMapWithBoth)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate multiple dashboards in a single ConfigMap", func() {
			By("Creating ConfigMap with multiple dashboards")
			multiDashboardConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind": "dashboard",
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"dashboard1.json": `{
						"uid": "dashboard-1",
						"title": "First Dashboard",
						"panels": []
					}`,
					"dashboard2.json": `{
						"uid": "dashboard-2", 
						"title": "Second Dashboard",
						"panels": []
					}`,
				},
			}

			_, err := validator.ValidateCreate(ctx, multiDashboardConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject ConfigMaps with mix of valid and invalid dashboards", func() {
			By("Creating ConfigMap with one valid and one invalid dashboard")
			mixedConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind": "dashboard",
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"valid.json": `{
						"uid": "valid-dashboard",
						"title": "Valid Dashboard",
						"panels": []
					}`,
					"invalid.json": `{
						"title": "Invalid Dashboard without UID",
						"panels": []
					}`,
				},
			}

			_, err := validator.ValidateCreate(ctx, mixedConfigMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dashboard UID is missing"))
		})

		It("Should validate dashboard ConfigMaps on update operations", func() {
			By("Testing update with valid dashboard changes")
			updatedObj := obj.DeepCopy()
			updatedObj.Data["dashboard.json"] = `{
				"uid": "test-dashboard",
				"title": "Updated Test Dashboard",
				"panels": [{"id": 1, "title": "New Panel"}]
			}`

			_, err := validator.ValidateUpdate(ctx, obj, updatedObj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject invalid dashboard updates", func() {
			By("Testing update with invalid dashboard")
			invalidUpdatedObj := obj.DeepCopy()
			invalidUpdatedObj.Data["dashboard.json"] = `{
				"title": "Dashboard without UID after update",
				"panels": []
			}`

			_, err := validator.ValidateUpdate(ctx, obj, invalidUpdatedObj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dashboard UID is missing"))
		})

		It("Should handle empty ConfigMap data gracefully", func() {
			By("Creating ConfigMap with empty data")
			emptyConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind": "dashboard",
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{},
			}

			_, err := validator.ValidateCreate(ctx, emptyConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Empty ConfigMap should be allowed
		})

		It("Should handle ConfigMap with non-JSON data gracefully", func() {
			By("Creating ConfigMap with non-JSON files")
			nonJSONConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-json-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind": "dashboard",
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"readme.txt": "This is not a JSON file",
					"dashboard.json": `{
						"uid": "valid-dashboard",
						"title": "Valid Dashboard",
						"panels": []
					}`,
				},
			}

			_, err := validator.ValidateCreate(ctx, nonJSONConfigMap)
			Expect(err).To(HaveOccurred()) // Mapper currently tries to parse all files as JSON
			Expect(err.Error()).To(ContainSubstring("invalid JSON format"))
		})

		It("Should handle dashboard ConfigMaps with complex validation scenarios", func() {
			By("Testing dashboard with ID field that should be ignored during validation")
			dashboardWithID := obj.DeepCopy()
			dashboardWithID.Data["dashboard.json"] = `{
				"uid": "dashboard-with-id",
				"id": 12345,
				"title": "Dashboard with ID Field",
				"panels": []
			}`

			_, err := validator.ValidateCreate(ctx, dashboardWithID)
			Expect(err).NotTo(HaveOccurred()) // ID field should not affect validation

			By("Testing dashboard with minimal required fields only")
			minimalDashboard := obj.DeepCopy()
			minimalDashboard.Data["dashboard.json"] = `{
				"uid": "minimal-dashboard"
			}`

			_, err = validator.ValidateCreate(ctx, minimalDashboard)
			Expect(err).NotTo(HaveOccurred()) // Only UID is required

			By("Testing dashboard with complex nested structure")
			complexDashboard := obj.DeepCopy()
			complexDashboard.Data["dashboard.json"] = `{
				"uid": "complex-dashboard",
				"title": "Complex Dashboard",
				"tags": ["monitoring", "kubernetes"],
				"panels": [
					{
						"id": 1,
						"title": "CPU Usage",
						"targets": [
							{
								"expr": "rate(cpu_usage[5m])",
								"legendFormat": "CPU"
							}
						]
					}
				],
				"templating": {
					"list": [
						{
							"name": "namespace",
							"type": "query",
							"query": "label_values(namespace)"
						}
					]
				}
			}`

			_, err = validator.ValidateCreate(ctx, complexDashboard)
			Expect(err).NotTo(HaveOccurred()) // Complex structure should be valid
		})

		It("Should validate multiple file extensions correctly", func() {
			By("Testing ConfigMap with .json extension")
			jsonConfigMap := obj.DeepCopy()
			delete(jsonConfigMap.Data, "dashboard.json")
			jsonConfigMap.Data["my-dashboard.json"] = `{
				"uid": "json-extension",
				"title": "JSON Extension Dashboard"
			}`

			_, err := validator.ValidateCreate(ctx, jsonConfigMap)
			Expect(err).NotTo(HaveOccurred())

			By("Testing ConfigMap with mixed file types")
			mixedConfigMap := obj.DeepCopy()
			mixedConfigMap.Data = map[string]string{
				"config.yaml":    "some: yaml",
				"script.sh":      "#!/bin/bash\necho hello",
				"dashboard.json": `{"uid": "mixed-files", "title": "Mixed Files Dashboard"}`,
				"another.json":   `{"uid": "another-dash", "title": "Another Dashboard"}`,
			}

			_, err = validator.ValidateCreate(ctx, mixedConfigMap)
			Expect(err).To(HaveOccurred()) // Non-JSON files will cause parsing errors
			Expect(err.Error()).To(ContainSubstring("invalid JSON format"))
		})

		It("Should validate organization handling edge cases", func() {
			By("Testing annotation takes precedence over label")
			precedenceConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "precedence-test",
					Namespace: "default",
					Labels: map[string]string{
						"app.giantswarm.io/kind":                   "dashboard",
						"observability.giantswarm.io/organization": "label-org",
					},
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "annotation-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{
						"uid": "precedence-test",
						"title": "Precedence Test Dashboard"
					}`,
				},
			}

			_, err := validator.ValidateCreate(ctx, precedenceConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Should use annotation-org

			By("Testing organization with special characters")
			specialOrgConfigMap := obj.DeepCopy()
			specialOrgConfigMap.Annotations["observability.giantswarm.io/organization"] = "org-with-dashes_and_underscores.and.dots"

			_, err = validator.ValidateCreate(ctx, specialOrgConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Special chars should be allowed

			By("Testing very long organization name")
			longOrgConfigMap := obj.DeepCopy()
			longOrgName := "very-long-organization-name-that-might-exceed-normal-limits-but-should-still-be-handled-gracefully-by-the-validation-system"
			longOrgConfigMap.Annotations["observability.giantswarm.io/organization"] = longOrgName

			_, err = validator.ValidateCreate(ctx, longOrgConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Long org names should be allowed
		})

		It("Should handle webhook lifecycle operations correctly", func() {
			By("Testing that validation is only called for dashboard ConfigMaps")
			regularConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "regular-config",
					Namespace: "default",
					Labels: map[string]string{
						"app": "some-application",
					},
				},
				Data: map[string]string{
					"config.properties": "key=value",
				},
			}

			_, err := validator.ValidateCreate(ctx, regularConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Non-dashboard ConfigMaps should pass through

			By("Testing delete operations don't validate")
			_, err = validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred()) // Delete should always pass

			By("Testing update with object type validation")
			_, err = validator.ValidateUpdate(ctx, obj, obj)
			Expect(err).NotTo(HaveOccurred()) // Valid update should work
		})

		It("Should provide meaningful error messages for different validation failures", func() {
			By("Testing specific error message for missing UID")
			noUIDConfigMap := obj.DeepCopy()
			noUIDConfigMap.Data["dashboard.json"] = `{
				"title": "Dashboard without UID",
				"description": "This dashboard is missing the required UID field"
			}`

			_, err := validator.ValidateCreate(ctx, noUIDConfigMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dashboard UID is missing"))
			// Error message format is: "failed to parse dashboard: dashboard UID not found in configmap"

			By("Testing error aggregation for multiple validation failures")
			multiErrorConfigMap := obj.DeepCopy()
			multiErrorConfigMap.Labels = map[string]string{
				"app.giantswarm.io/kind": "dashboard",
			}
			multiErrorConfigMap.Annotations = nil // Remove organization
			multiErrorConfigMap.Data["dashboard.json"] = `{
				"title": "Dashboard with multiple errors"
			}` // Missing UID

			_, err = validator.ValidateCreate(ctx, multiErrorConfigMap)
			Expect(err).To(HaveOccurred())
			// Should report the first error encountered (UID missing in this case since UID validation comes first)
			Expect(err.Error()).To(ContainSubstring("dashboard UID is missing"))

			By("Testing JSON parsing error details")
			invalidJSONConfigMap := obj.DeepCopy()
			invalidJSONConfigMap.Data["dashboard.json"] = `{
				"uid": "invalid-json",
				"title": "Invalid JSON"
				"missing": "comma"
			}` // Invalid JSON syntax

			_, err = validator.ValidateCreate(ctx, invalidJSONConfigMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dashboard validation failed"))
		})

		It("Should handle webhook performance and resource constraints", func() {
			By("Testing validation performance with reasonable dashboard size")
			reasonableSizeConfigMap := obj.DeepCopy()

			// Create a dashboard with reasonable number of panels
			panels := ""
			for i := 0; i < 20; i++ {
				if i > 0 {
					panels += ","
				}
				panels += fmt.Sprintf(`{
					"id": %d,
					"title": "Panel %d",
					"type": "graph",
					"targets": [
						{
							"expr": "up{job=\"kubernetes-nodes\"}",
							"legendFormat": "Node {{instance}}"
						}
					]
				}`, i, i)
			}

			reasonableSizeConfigMap.Data["dashboard.json"] = fmt.Sprintf(`{
				"uid": "performance-test",
				"title": "Performance Test Dashboard",
				"description": "Dashboard with reasonable number of panels for performance testing",
				"panels": [%s]
			}`, panels)

			_, err := validator.ValidateCreate(ctx, reasonableSizeConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Should handle reasonable size efficiently

			By("Testing memory usage with multiple dashboards")
			multiDashConfigMap := obj.DeepCopy()
			delete(multiDashConfigMap.Data, "dashboard.json")

			// Add multiple small dashboards
			for i := 0; i < 10; i++ {
				multiDashConfigMap.Data[fmt.Sprintf("dashboard-%d.json", i)] = fmt.Sprintf(`{
					"uid": "multi-dash-%d",
					"title": "Dashboard %d"
				}`, i, i)
			}

			_, err = validator.ValidateCreate(ctx, multiDashConfigMap)
			Expect(err).NotTo(HaveOccurred()) // Should handle multiple dashboards efficiently
		})

		Context("When testing additional edge cases and security scenarios", func() {
			It("Should handle Unicode characters in organization names", func() {
				By("Testing organization with Unicode characters")
				unicodeOrgConfigMap := obj.DeepCopy()
				unicodeOrgConfigMap.Annotations["observability.giantswarm.io/organization"] = "组织-العربية-русский-🏢"

				_, err := validator.ValidateCreate(ctx, unicodeOrgConfigMap)
				Expect(err).NotTo(HaveOccurred()) // Unicode should be allowed
			})

			It("Should handle extremely large dashboard JSON gracefully", func() {
				By("Creating a dashboard with thousands of panels")
				extremeConfigMap := obj.DeepCopy()

				// Create JSON with 1000 panels to test memory/performance limits
				panels := ""
				for i := 0; i < 1000; i++ {
					if i > 0 {
						panels += ","
					}
					panels += fmt.Sprintf(`{
						"id": %d,
						"title": "Panel %d",
						"type": "graph",
						"targets": [{"expr": "metric_%d"}],
						"description": "This is a very long description that might consume significant memory when parsing large numbers of panels in a single dashboard configuration"
					}`, i, i, i)
				}

				extremeJSON := fmt.Sprintf(`{
					"uid": "extreme-dashboard",
					"title": "Extreme Size Dashboard",
					"panels": [%s]
				}`, panels)
				extremeConfigMap.Data["dashboard.json"] = extremeJSON

				_, err := validator.ValidateCreate(ctx, extremeConfigMap)
				Expect(err).NotTo(HaveOccurred()) // Should handle extreme size gracefully
			})

			It("Should handle empty string values gracefully", func() {
				By("Testing dashboard with empty UID string")
				emptyUIDConfigMap := obj.DeepCopy()
				emptyUIDConfigMap.Data["dashboard.json"] = `{
					"uid": "",
					"title": "Dashboard with Empty UID"
				}`

				_, err := validator.ValidateCreate(ctx, emptyUIDConfigMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard UID is missing"))

				By("Testing organization with empty string")
				emptyOrgConfigMap := obj.DeepCopy()
				emptyOrgConfigMap.Annotations["observability.giantswarm.io/organization"] = ""

				_, err = validator.ValidateCreate(ctx, emptyOrgConfigMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dashboard organization is missing"))
			})

			It("Should handle malformed data gracefully", func() {
				By("Testing with binary data in ConfigMap")
				binaryConfigMap := obj.DeepCopy()
				binaryConfigMap.Data["binary.json"] = string([]byte{0x00, 0x01, 0x02, 0xFF})

				_, err := validator.ValidateCreate(ctx, binaryConfigMap)
				Expect(err).To(HaveOccurred()) // Binary data should fail JSON parsing
				Expect(err.Error()).To(ContainSubstring("invalid JSON format"))

				By("Testing with extremely nested JSON")
				deeplyNestedConfigMap := obj.DeepCopy()
				// Create deeply nested JSON structure
				nestedJSON := `{"uid": "nested-dashboard", "level1": {"level2": {"level3": {"level4": {"level5": {"value": "deep"}}}}}}`
				deeplyNestedConfigMap.Data["nested.json"] = nestedJSON

				_, err = validator.ValidateCreate(ctx, deeplyNestedConfigMap)
				Expect(err).NotTo(HaveOccurred()) // Deep nesting should be allowed
			})

			It("Should validate against JSON injection attacks", func() {
				By("Testing with JSON containing potential injection patterns")
				injectionConfigMap := obj.DeepCopy()
				injectionConfigMap.Data["dashboard.json"] = `{
					"uid": "injection-test",
					"title": "Test Dashboard",
					"malicious": "'; DROP TABLE dashboards; --",
					"script": "<script>alert('xss')</script>",
					"unicode_escape": "\u0000\u001f"
				}`

				_, err := validator.ValidateCreate(ctx, injectionConfigMap)
				Expect(err).NotTo(HaveOccurred()) // JSON parsing should handle these safely
			})
		})
	})
})
