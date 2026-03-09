package basic

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/giantswarm/apptest-framework/v3/pkg/state"
	"github.com/giantswarm/apptest-framework/v3/pkg/suite"
	"github.com/giantswarm/clustertest/v3/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	isUpgrade   = false
	mcClusterNS = "org-giantswarm"
	testOrgName = "test-e2e-org"

	pollInterval = 5 * time.Second
	pollTimeout  = 5 * time.Minute
)

var grafanaOrgGVK = schema.GroupVersionKind{
	Group:   "observability.giantswarm.io",
	Version: "v1alpha2",
	Kind:    "GrafanaOrganization",
}

func TestConfig(t *testing.T) {
	suite.New().
		WithIsUpgrade(isUpgrade).
		WithValuesFile("./values.yaml").
		Tests(func() {
			Describe("observability-operator", Ordered, func() {
				var mcClusterName string

				BeforeAll(func() {
					mcClient := state.GetFramework().MC()
					clusterList := &clusterv1.ClusterList{}
					err := mcClient.List(state.GetContext(), clusterList, client.InNamespace(mcClusterNS))
					Expect(err).NotTo(HaveOccurred())
					Expect(clusterList.Items).NotTo(BeEmpty(), "no Cluster CR found in namespace %s", mcClusterNS)
					mcClusterName = clusterList.Items[0].Name
					logger.Log("Using MC cluster: %s/%s", mcClusterNS, mcClusterName)
				})

				Context("Alloy configuration", func() {
					for _, cmSuffix := range []string{"monitoring-config", "logging-config", "events-logger-config"} {
						cmSuffix := cmSuffix
						It(fmt.Sprintf("should have ConfigMap %s", cmSuffix), func() {
							mcClient := state.GetFramework().MC()
							name := fmt.Sprintf("%s-%s", mcClusterName, cmSuffix)
							Eventually(func() error {
								var cm corev1.ConfigMap
								return mcClient.Get(state.GetContext(), types.NamespacedName{
									Namespace: mcClusterNS,
									Name:      name,
								}, &cm)
							}).WithPolling(pollInterval).WithTimeout(pollTimeout).Should(Succeed(),
								"ConfigMap %s/%s not found", mcClusterNS, name)
						})
					}
				})

				Context("Loki/Mimir/Tempo auth secrets", func() {
					for _, authType := range []string{"metrics", "logs", "traces"} {
						authType := authType
						It(fmt.Sprintf("should have per-cluster %s auth secret with password and htpasswd keys", authType), func() {
							mcClient := state.GetFramework().MC()
							secretName := fmt.Sprintf("%s-observability-%s-auth", mcClusterName, authType)
							Eventually(func() error {
								var secret corev1.Secret
								if err := mcClient.Get(state.GetContext(), types.NamespacedName{
									Namespace: mcClusterNS,
									Name:      secretName,
								}, &secret); err != nil {
									return err
								}
								if _, ok := secret.Data["password"]; !ok {
									return fmt.Errorf("secret %s missing 'password' key", secretName)
								}
								if _, ok := secret.Data["htpasswd"]; !ok {
									return fmt.Errorf("secret %s missing 'htpasswd' key", secretName)
								}
								return nil
							}).WithPolling(pollInterval).WithTimeout(pollTimeout).Should(Succeed(),
								"per-cluster auth secret %s/%s not ready", mcClusterNS, secretName)
						})
					}

					for _, gw := range []struct{ ns, name string }{
						{"mimir", "mimir-gateway-ingress-auth"},
						{"loki", "loki-gateway-ingress-auth"},
						{"tempo", "tempo-gateway-ingress-auth"},
					} {
						gw := gw
						It(fmt.Sprintf("should have gateway secret %s/%s", gw.ns, gw.name), func() {
							mcClient := state.GetFramework().MC()
							Eventually(func() error {
								var secret corev1.Secret
								return mcClient.Get(state.GetContext(), types.NamespacedName{
									Namespace: gw.ns,
									Name:      gw.name,
								}, &secret)
							}).WithPolling(pollInterval).WithTimeout(pollTimeout).Should(Succeed(),
								"gateway secret %s/%s not found", gw.ns, gw.name)
						})
					}
				})

				Context("GrafanaOrganization lifecycle", func() {
					AfterAll(func() {
						// Best-effort cleanup in case a test failed before deletion
						mcClient := state.GetFramework().MC()
						org := &unstructured.Unstructured{}
						org.SetGroupVersionKind(grafanaOrgGVK)
						org.SetName(testOrgName)
						_ = mcClient.Delete(state.GetContext(), org)
					})

					It("should reconcile a new GrafanaOrganization and populate status.orgID", func() {
						mcClient := state.GetFramework().MC()
						org := &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "observability.giantswarm.io/v1alpha2",
								"kind":       "GrafanaOrganization",
								"metadata": map[string]interface{}{
									"name": testOrgName,
								},
								"spec": map[string]interface{}{
									"displayName": "Test E2E Organization",
									"rbac": map[string]interface{}{
										"admins":  []interface{}{"giantswarm"},
										"editors": []interface{}{},
										"viewers": []interface{}{},
									},
									"tenants": []interface{}{
										map[string]interface{}{
											"name":  "giantswarm",
											"types": []interface{}{"data"},
										},
									},
								},
							},
						}
						Expect(mcClient.Create(state.GetContext(), org)).To(Succeed())

						Eventually(func() (int64, error) {
							result := &unstructured.Unstructured{}
							result.SetGroupVersionKind(grafanaOrgGVK)
							if err := mcClient.Get(state.GetContext(), types.NamespacedName{Name: testOrgName}, result); err != nil {
								return 0, err
							}
							orgID, _, _ := unstructured.NestedInt64(result.Object, "status", "orgID")
							if orgID == 0 {
								return 0, fmt.Errorf("status.orgID is not set yet")
							}
							return orgID, nil
						}).WithPolling(pollInterval).WithTimeout(pollTimeout).Should(BeNumerically(">", int64(0)),
							"GrafanaOrganization %s: status.orgID never became non-zero", testOrgName)
					})

					It("should populate status.dataSources", func() {
						mcClient := state.GetFramework().MC()
						Eventually(func() (int, error) {
							result := &unstructured.Unstructured{}
							result.SetGroupVersionKind(grafanaOrgGVK)
							if err := mcClient.Get(state.GetContext(), types.NamespacedName{Name: testOrgName}, result); err != nil {
								return 0, err
							}
							ds, _, _ := unstructured.NestedSlice(result.Object, "status", "dataSources")
							return len(ds), nil
						}).WithPolling(pollInterval).WithTimeout(pollTimeout).Should(BeNumerically(">", 0),
							"GrafanaOrganization %s: status.dataSources is empty", testOrgName)
					})

					It("should be fully removed after deletion", func() {
						mcClient := state.GetFramework().MC()
						org := &unstructured.Unstructured{}
						org.SetGroupVersionKind(grafanaOrgGVK)
						org.SetName(testOrgName)
						Expect(mcClient.Delete(state.GetContext(), org)).To(Succeed())

						Eventually(func() bool {
							result := &unstructured.Unstructured{}
							result.SetGroupVersionKind(grafanaOrgGVK)
							err := mcClient.Get(state.GetContext(), types.NamespacedName{Name: testOrgName}, result)
							return apierrors.IsNotFound(err)
						}).WithPolling(pollInterval).WithTimeout(pollTimeout).Should(BeTrue(),
							"GrafanaOrganization %s was not removed (finalizer may be stuck)", testOrgName)
					})
				})
			})
		}).
		Run(t, "Observability Operator basic tests")
}
