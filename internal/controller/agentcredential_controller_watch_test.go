package controller

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/credential"
)

var _ = Describe("AgentCredential Controller duplicate agent name watch", func() {
	const (
		timeout          = time.Second * 30
		interval         = time.Millisecond * 250
		watchNamespace   = "default"
		watchGatewayNs   = "mimir-watch"
		watchIngress     = "mimir-watch-ingress"
		watchHTTPRoute   = "mimir-watch-httproute"
		watchAgentName   = "shared-agent"
		watchWinnerName  = "dup-a"
		watchLoserName   = "dup-b"
		watchRenamedName = "renamed-agent"
	)

	DescribeTable("promotes the loser once the winner gives up the agent name",
		func(giveUp func(ctx context.Context, c client.Client, winner *observabilityv1alpha1.AgentCredential)) {
			watchCtx, watchCancel := context.WithCancel(ctx)

			By("starting an API server with the AgentCredential CRD")
			watchScheme := runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(watchScheme)).To(Succeed())
			Expect(observabilityv1alpha1.AddToScheme(watchScheme)).To(Succeed())

			watchEnv := &envtest.Environment{
				CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases", "observability.giantswarm.io_agentcredentials.yaml")},
				ErrorIfCRDPathMissing: true,
				BinaryAssetsDirectory: getKubeBuilderAssets(),
			}
			watchCfg, err := watchEnv.Start()
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(watchEnv.Stop)
			// Stop the manager before the API server.
			DeferCleanup(watchCancel)

			watchClient, err := client.New(watchCfg, client.Options{Scheme: watchScheme})
			Expect(err).NotTo(HaveOccurred())
			Expect(watchClient.Create(watchCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: watchGatewayNs}})).To(Succeed())

			By("starting the AgentCredential controller")
			mgr, err := ctrl.NewManager(watchCfg, ctrl.Options{
				Scheme:  watchScheme,
				Metrics: metricsserver.Options{BindAddress: "0"},
				Controller: ctrlconfig.Controller{
					SkipNameValidation: new(true),
					CacheSyncTimeout:   10 * time.Second,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			gatewayConfigs := credential.GatewayConfigs{
				observabilityv1alpha1.CredentialBackendMetrics: credential.NewGatewayConfig(watchGatewayNs, watchIngress, watchHTTPRoute),
			}
			reconciler := &AgentCredentialReconciler{
				Client:          mgr.GetClient(),
				Renderer:        credential.NewRenderer(mgr.GetClient()),
				Aggregator:      credential.NewAggregator(mgr.GetClient(), gatewayConfigs),
				finalizerHelper: NewFinalizerHelper(mgr.GetClient(), observabilityv1alpha1.AgentCredentialFinalizer),
			}
			Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

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

			gatewayEntries := func(g Gomega) []string {
				ingress := &corev1.Secret{}
				g.Expect(watchClient.Get(watchCtx, types.NamespacedName{Namespace: watchGatewayNs, Name: watchIngress}, ingress)).To(Succeed())
				var entries []string
				for line := range strings.SplitSeq(string(ingress.Data[credential.IngressDataKey]), "\n") {
					if strings.HasPrefix(line, watchAgentName+":") {
						entries = append(entries, line)
					}
				}
				return entries
			}
			gatewaySynced := func(g Gomega, name string) *metav1.Condition {
				cred := &observabilityv1alpha1.AgentCredential{}
				g.Expect(watchClient.Get(watchCtx, types.NamespacedName{Namespace: watchNamespace, Name: name}, cred)).To(Succeed())
				cond := meta.FindStatusCondition(cred.Status.Conditions, observabilityv1alpha1.AgentCredentialConditionGatewaySynced)
				g.Expect(cond).NotTo(BeNil())
				return cond
			}
			renderedSecret := func(g Gomega, name string) *corev1.Secret {
				secret := &corev1.Secret{}
				g.Expect(watchClient.Get(watchCtx, types.NamespacedName{Namespace: watchNamespace, Name: name}, secret)).To(Succeed())
				return secret
			}

			By("creating two credentials with the same agent name")
			newCred := func(name string) *observabilityv1alpha1.AgentCredential {
				return &observabilityv1alpha1.AgentCredential{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: watchNamespace},
					Spec: observabilityv1alpha1.AgentCredentialSpec{
						Backend:   observabilityv1alpha1.CredentialBackendMetrics,
						AgentName: watchAgentName,
					},
				}
			}
			// The winner is created first and also wins the name tie-break.
			winner := newCred(watchWinnerName)
			Expect(watchClient.Create(watchCtx, winner)).To(Succeed())
			Expect(watchClient.Create(watchCtx, newCred(watchLoserName))).To(Succeed())

			var loserPassword []byte
			Eventually(func(g Gomega) {
				expectManagerRunning()
				g.Expect(gatewaySynced(g, watchWinnerName).Status).To(Equal(metav1.ConditionTrue))
				g.Expect(gatewaySynced(g, watchLoserName).Reason).To(Equal("DuplicateAgentName"))
				g.Expect(gatewayEntries(g)).To(Equal([]string{string(renderedSecret(g, watchWinnerName).Data[credential.SecretKeyHtpasswd])}))
				loserPassword = renderedSecret(g, watchLoserName).Data[credential.SecretKeyPassword]
			}, timeout, interval).Should(Succeed(), "only the winner's entry should be served")

			By("making the winner give up the agent name")
			Expect(watchClient.Get(watchCtx, client.ObjectKeyFromObject(winner), winner)).To(Succeed())
			giveUp(watchCtx, watchClient, winner)

			Eventually(func(g Gomega) {
				expectManagerRunning()
				g.Expect(gatewaySynced(g, watchLoserName).Status).To(Equal(metav1.ConditionTrue))
				loserSecret := renderedSecret(g, watchLoserName)
				g.Expect(loserSecret.Data[credential.SecretKeyPassword]).To(Equal(loserPassword))
				g.Expect(gatewayEntries(g)).To(Equal([]string{string(loserSecret.Data[credential.SecretKeyHtpasswd])}))
			}, timeout, interval).Should(Succeed(), "the loser should be promoted without its own reconcile trigger")
		},
		Entry("winner deleted", func(ctx context.Context, c client.Client, winner *observabilityv1alpha1.AgentCredential) {
			Expect(c.Delete(ctx, winner)).To(Succeed())
		}),
		Entry("winner renamed", func(ctx context.Context, c client.Client, winner *observabilityv1alpha1.AgentCredential) {
			winner.Spec.AgentName = watchRenamedName
			Expect(c.Update(ctx, winner)).To(Succeed())
		}),
	)
})
