package controller

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/yaml"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
	webhookv1alpha2 "github.com/giantswarm/observability-operator/internal/webhook/v1alpha2"
)

const watchOrgAdmin = "admin"

// clusterGetCounter counts the Cluster reads of each reconcile and reports every Cluster as not found,
// so a reconcile request ends right after it is counted.
type clusterGetCounter struct {
	client.Client

	mu   sync.Mutex
	gets map[types.NamespacedName]int
}

func (c *clusterGetCounter) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*clusterv1.Cluster); !ok {
		return c.Client.Get(ctx, key, obj, opts...)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets[key]++
	return apierrors.NewNotFound(clusterv1.GroupVersion.WithResource("clusters").GroupResource(), key.Name)
}

func (c *clusterGetCounter) count(key types.NamespacedName) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gets[key]
}

// grafanaOrganizationCRD returns the generated GrafanaOrganization CRD as the Helm chart renders it.
// Without the webhook, the chart serves only the storage version and sets no conversion webhook.
func grafanaOrganizationCRD(webhookEnabled bool) *apiextensionsv1.CustomResourceDefinition {
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "crd", "bases", "observability.giantswarm.io_grafanaorganizations.yaml"))
	Expect(err).NotTo(HaveOccurred())

	crd := &apiextensionsv1.CustomResourceDefinition{}
	Expect(yaml.Unmarshal(data, crd)).To(Succeed())

	if !webhookEnabled {
		var versions []apiextensionsv1.CustomResourceDefinitionVersion
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				versions = append(versions, version)
			}
		}
		crd.Spec.Versions = versions
		crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}
	}

	return crd
}

func createV1alpha1GrafanaOrganization(ctx context.Context, c client.Client, name string) error {
	return c.Create(ctx, &observabilityv1alpha1.GrafanaOrganization{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
			DisplayName: name,
			RBAC:        &observabilityv1alpha1.RBAC{Admins: []string{watchOrgAdmin}},
			Tenants:     []observabilityv1alpha1.TenantID{"watch_tenant"},
		},
	})
}

func createV1alpha2GrafanaOrganization(ctx context.Context, c client.Client, name string) error {
	return c.Create(ctx, &observabilityv1alpha2.GrafanaOrganization{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: observabilityv1alpha2.GrafanaOrganizationSpec{
			DisplayName: name,
			RBAC:        &observabilityv1alpha2.RBAC{Admins: []string{watchOrgAdmin}},
			Tenants:     []observabilityv1alpha2.TenantConfig{{Name: "watch_tenant"}},
		},
	})
}

var _ = Describe("Cluster Controller GrafanaOrganization watch", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 250
	)

	DescribeTable("reconciles every Cluster when a GrafanaOrganization is created",
		func(webhookEnabled bool, createGrafanaOrganization func(context.Context, client.Client, string) error) {
			watchCtx, watchCancel := context.WithCancel(ctx)

			By("starting an API server with the GrafanaOrganization CRD as the chart renders it")
			clusterCRD := &apiextensionsv1.CustomResourceDefinition{}
			Expect(k8sClient.Get(watchCtx, client.ObjectKey{Name: clusterCRDName}, clusterCRD)).To(Succeed())
			clusterCRD.ObjectMeta = metav1.ObjectMeta{Name: clusterCRD.Name}
			clusterCRD.Status = apiextensionsv1.CustomResourceDefinitionStatus{}

			watchScheme := runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(watchScheme)).To(Succeed())
			Expect(clusterv1.AddToScheme(watchScheme)).To(Succeed())
			Expect(observabilityv1alpha1.AddToScheme(watchScheme)).To(Succeed())
			Expect(observabilityv1alpha2.AddToScheme(watchScheme)).To(Succeed())

			// envtest points the CRD at the local conversion webhook when its scheme holds both versions.
			envScheme := runtime.NewScheme()
			if webhookEnabled {
				envScheme = watchScheme
			}

			watchEnv := &envtest.Environment{
				Scheme:                envScheme,
				CRDs:                  []*apiextensionsv1.CustomResourceDefinition{clusterCRD, grafanaOrganizationCRD(webhookEnabled)},
				BinaryAssetsDirectory: getKubeBuilderAssets(),
			}
			watchCfg, err := watchEnv.Start()
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(watchEnv.Stop)
			// Stop the manager before the API server.
			DeferCleanup(watchCancel)

			watchClient, err := client.New(watchCfg, client.Options{Scheme: watchScheme})
			Expect(err).NotTo(HaveOccurred())

			By("starting the cluster controller")
			options := ctrl.Options{
				Scheme:  watchScheme,
				Metrics: metricsserver.Options{BindAddress: "0"},
				Controller: ctrlconfig.Controller{
					SkipNameValidation: new(true),
					CacheSyncTimeout:   10 * time.Second,
				},
			}
			if webhookEnabled {
				options.WebhookServer = webhook.NewServer(webhook.Options{
					Host:    watchEnv.WebhookInstallOptions.LocalServingHost,
					Port:    watchEnv.WebhookInstallOptions.LocalServingPort,
					CertDir: watchEnv.WebhookInstallOptions.LocalServingCertDir,
				})
			}
			mgr, err := ctrl.NewManager(watchCfg, options)
			Expect(err).NotTo(HaveOccurred())
			if webhookEnabled {
				Expect(webhookv1alpha2.SetupGrafanaOrganizationWebhookWithManager(mgr)).To(Succeed())
			}

			counter := &clusterGetCounter{Client: mgr.GetClient(), gets: map[types.NamespacedName]int{}}
			Expect((&ClusterReconciler{Client: counter}).SetupWithManager(mgr)).To(Succeed())

			mgrErr := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				mgrErr <- mgr.Start(watchCtx)
			}()
			expectManagerRunning := func() {
				select {
				case err := <-mgrErr:
					StopTrying("the manager stopped").Wrap(err).Now()
				default:
				}
			}

			By("creating a Cluster")
			clusterKey := types.NamespacedName{Name: "watch-cluster", Namespace: "default"}
			Expect(watchClient.Create(watchCtx, &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "infrastructure.cluster.x-k8s.io",
						Kind:     "AWSCluster",
						Name:     "watch-aws-cluster",
					},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "controlplane.cluster.x-k8s.io",
						Kind:     "KubeadmControlPlane",
						Name:     "watch-control-plane",
					},
				},
			})).To(Succeed())

			Eventually(func(g Gomega) {
				expectManagerRunning()
				g.Expect(counter.count(clusterKey)).To(BeNumerically(">=", 1))
			}, timeout, interval).Should(Succeed(), "the cluster controller should start and reconcile the Cluster")
			reconciles := counter.count(clusterKey)

			By("creating a GrafanaOrganization")
			Expect(createGrafanaOrganization(watchCtx, watchClient, "watch-org")).To(Succeed())

			Eventually(func(g Gomega) {
				expectManagerRunning()
				g.Expect(counter.count(clusterKey)).To(BeNumerically(">", reconciles))
			}, timeout, interval).Should(Succeed(), "the GrafanaOrganization should trigger a Cluster reconcile")
		},
		Entry("webhook disabled, GrafanaOrganization created as v1alpha2", false, createV1alpha2GrafanaOrganization),
		Entry("webhook enabled, GrafanaOrganization created as v1alpha2", true, createV1alpha2GrafanaOrganization),
		Entry("webhook enabled, GrafanaOrganization created as v1alpha1", true, createV1alpha1GrafanaOrganization),
	)
})

var _ = Describe("Cluster Controller AgentCredential Secret watch", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 250
	)

	It("reconciles the owning Cluster when its AgentCredential Secret changes", func() {
		watchCtx, watchCancel := context.WithCancel(ctx)

		By("starting an API server with the Cluster and AgentCredential CRDs")
		var crds []*apiextensionsv1.CustomResourceDefinition
		for _, name := range []string{clusterCRDName, "agentcredentials.observability.giantswarm.io"} {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			Expect(k8sClient.Get(watchCtx, client.ObjectKey{Name: name}, crd)).To(Succeed())
			crd.ObjectMeta = metav1.ObjectMeta{Name: crd.Name}
			crd.Status = apiextensionsv1.CustomResourceDefinitionStatus{}
			crds = append(crds, crd)
		}
		crds = append(crds, grafanaOrganizationCRD(false))

		watchScheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(watchScheme)).To(Succeed())
		Expect(clusterv1.AddToScheme(watchScheme)).To(Succeed())
		Expect(observabilityv1alpha1.AddToScheme(watchScheme)).To(Succeed())
		Expect(observabilityv1alpha2.AddToScheme(watchScheme)).To(Succeed())

		watchEnv := &envtest.Environment{
			Scheme:                runtime.NewScheme(),
			CRDs:                  crds,
			BinaryAssetsDirectory: getKubeBuilderAssets(),
		}
		watchCfg, err := watchEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(watchEnv.Stop)
		// Stop the manager before the API server.
		DeferCleanup(watchCancel)

		watchClient, err := client.New(watchCfg, client.Options{Scheme: watchScheme})
		Expect(err).NotTo(HaveOccurred())

		By("starting the cluster controller")
		mgr, err := ctrl.NewManager(watchCfg, ctrl.Options{
			Scheme:  watchScheme,
			Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: ctrlconfig.Controller{
				SkipNameValidation: new(true),
				CacheSyncTimeout:   10 * time.Second,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		counter := &clusterGetCounter{Client: mgr.GetClient(), gets: map[types.NamespacedName]int{}}
		Expect((&ClusterReconciler{Client: counter}).SetupWithManager(mgr)).To(Succeed())

		mgrErr := make(chan error, 1)
		go func() {
			defer GinkgoRecover()
			mgrErr <- mgr.Start(watchCtx)
		}()
		expectManagerRunning := func() {
			select {
			case err := <-mgrErr:
				StopTrying("the manager stopped").Wrap(err).Now()
			default:
			}
		}

		By("creating a Cluster, its AgentCredential and the AgentCredential's Secret")
		clusterKey := types.NamespacedName{Name: "secret-watch-cluster", Namespace: "default"}
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "AWSCluster",
					Name:     "secret-watch-aws-cluster",
				},
				ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "controlplane.cluster.x-k8s.io",
					Kind:     "KubeadmControlPlane",
					Name:     "secret-watch-control-plane",
				},
			},
		}
		Expect(watchClient.Create(watchCtx, cluster)).To(Succeed())

		cred := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "secret-watch-cluster-observability-metrics", Namespace: clusterKey.Namespace},
			Spec:       observabilityv1alpha1.AgentCredentialSpec{Backend: observabilityv1alpha1.CredentialBackendMetrics},
		}
		Expect(controllerutil.SetControllerReference(cluster, cred, watchScheme)).To(Succeed())
		Expect(watchClient.Create(watchCtx, cred)).To(Succeed())

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "secret-watch-cluster-observability-metrics-auth", Namespace: clusterKey.Namespace},
			StringData: map[string]string{"username": "old"},
		}
		Expect(controllerutil.SetControllerReference(cred, secret, watchScheme)).To(Succeed())
		Expect(watchClient.Create(watchCtx, secret)).To(Succeed())

		Eventually(func(g Gomega) {
			expectManagerRunning()
			g.Expect(counter.count(clusterKey)).To(BeNumerically(">=", 1))
		}, timeout, interval).Should(Succeed(), "the cluster controller should start and reconcile the Cluster")
		// Let the reconciles triggered by the creations, including the Secret's, settle.
		Eventually(func() bool {
			before := counter.count(clusterKey)
			time.Sleep(time.Second)
			return counter.count(clusterKey) == before
		}, timeout, interval).Should(BeTrue())
		reconciles := counter.count(clusterKey)

		By("changing the username in the AgentCredential's Secret")
		Expect(watchClient.Get(watchCtx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
		secret.StringData = map[string]string{"username": "default/secret-watch-cluster-observability-metrics"}
		Expect(watchClient.Update(watchCtx, secret)).To(Succeed())

		Eventually(func(g Gomega) {
			expectManagerRunning()
			g.Expect(counter.count(clusterKey)).To(BeNumerically(">", reconciles))
		}, timeout, interval).Should(Succeed(), "the Secret change should trigger a Cluster reconcile")
	})
})
