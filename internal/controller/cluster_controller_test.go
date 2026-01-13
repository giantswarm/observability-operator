package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/bundle"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/agent/collectors/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

var _ = Describe("Cluster Controller", func() {
	Context("When reconciling a CAPI Cluster resource", func() {
		const (
			clusterName = "test-cluster"
			timeout     = time.Second * 10
			interval    = time.Millisecond * 250
		)

		var (
			ctx              context.Context
			cluster          *clusterv1.Cluster
			reconciler       *ClusterMonitoringReconciler
			namespaceName    types.NamespacedName
			clusterNamespace string
		)

		BeforeEach(func() {
			ctx = context.Background()

			// Generate unique namespace name for each test
			clusterNamespace = fmt.Sprintf("test-ns-%d", time.Now().UnixNano())

			namespaceName = types.NamespacedName{
				Name:      clusterName,
				Namespace: clusterNamespace,
			}

			// Create test namespace if it doesn't exist
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			// Create test CAPI Cluster
			cluster = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
						Kind:       "AWSCluster",
						Name:       "test-aws-cluster",
					},
					ControlPlaneRef: &corev1.ObjectReference{
						APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
						Kind:       "KubeadmControlPlane",
						Name:       "test-control-plane",
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// Setup reconciler with actual services instead of mocks
			organizationRepository := organization.NewNamespaceRepository(k8sClient)

			bundleService := bundle.NewBundleConfigurationService(k8sClient, config.Config{
				Monitoring: config.MonitoringConfig{
					Enabled: true,
				},
			})

			mimirAuthManager := auth.NewAuthManager(
				k8sClient,
				auth.NewConfig(
					auth.AuthTypeMetrics,
					"mimir",
					"mimir-gateway-ingress-auth",
					"mimir-gateway-httproute-auth",
				),
			)

			alloyMetricsService := metrics.Service{
				Config: config.Config{
					Cluster: config.ClusterConfig{
						Name:     "management-cluster",
						Pipeline: "testing",
						Region:   "eu-west-1",
						Customer: "giantswarm",
					},
					Monitoring: config.MonitoringConfig{
						Enabled: true,
					},
				},
				ConfigurationRepository: agent.NewConfigurationRepository(k8sClient),
				OrganizationRepository:  organizationRepository,
				TenantRepository:        tenancy.NewTenantRepository(k8sClient),
				AuthManager:             mimirAuthManager,
			}

			// Create auth managers map for the reconciler
			authManagers := map[auth.AuthType]authManagerEntry{
				auth.AuthTypeMetrics: {
					authManager: mimirAuthManager,
					isEnabled: func(c *clusterv1.Cluster) bool {
						return config.Config{Monitoring: config.MonitoringConfig{Enabled: true}}.Monitoring.IsMonitoringEnabled(c)
					},
				},
			}

			reconciler = &ClusterMonitoringReconciler{
				Client: k8sClient,
				Config: config.Config{
					Cluster: config.ClusterConfig{
						Name:     "management-cluster",
						Pipeline: "testing",
						Region:   "eu-west-1",
						Customer: "giantswarm",
					},
					Monitoring: config.MonitoringConfig{
						Enabled: true,
					},
				},
				BundleConfigurationService: bundleService,
				AlloyMetricsService:        alloyMetricsService,
				authManagers:               authManagers,
				finalizerHelper:            NewFinalizerHelper(k8sClient, monitoring.MonitoringFinalizer),
			}
		})

		AfterEach(func() {
			// Clean up the cluster
			if cluster != nil {
				err := k8sClient.Delete(ctx, cluster)
				if err != nil && !apierrors.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// Clean up the namespace
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespace}}
			err := k8sClient.Delete(ctx, ns)
			if err != nil && !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should successfully reconcile a Cluster resource", func() {
			By("Reconciling the created resource")

			// Test the reconcile function
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespaceName,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("Checking the cluster still exists after reconciliation")
			Eventually(func() error {
				found := &clusterv1.Cluster{}
				return k8sClient.Get(ctx, namespaceName, found)
			}, timeout, interval).Should(Succeed())
		})

		It("should handle cluster deletion with finalizers", func() {
			By("Adding finalizer to the cluster")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, namespaceName, cluster); err != nil {
					return err
				}
				cluster.Finalizers = append(cluster.Finalizers, monitoring.MonitoringFinalizer)
				return k8sClient.Update(ctx, cluster)
			}, timeout, interval).Should(Succeed())

			By("Deleting the cluster")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())

			By("Reconciling during deletion")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespaceName,
			})

			// Should handle deletion gracefully with real services
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should handle non-existent cluster resources", func() {
			By("Reconciling a non-existent resource")

			nonExistentName := types.NamespacedName{
				Name:      "non-existent-cluster",
				Namespace: clusterNamespace,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nonExistentName,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})
})
