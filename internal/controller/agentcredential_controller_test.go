package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/credential"
)

var _ = Describe("AgentCredential Controller", func() {
	const (
		gatewayNamespace    = "mimir-test"
		ingressSecretName   = "mimir-gateway-ingress-auth"
		httprouteSecretName = "mimir-gateway-httproute-auth"
		timeout             = time.Second * 10
		interval            = time.Millisecond * 250
	)

	var (
		ctx        context.Context
		ns         string
		reconciler *AgentCredentialReconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		ns = fmt.Sprintf("ac-test-ns-%d", time.Now().UnixNano())

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())

		// Gateway secrets live in their own namespace.
		err := k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		gatewayConfigs := credential.GatewayConfigs{
			observabilityv1alpha1.CredentialBackendMetrics: credential.NewGatewayConfig(gatewayNamespace, ingressSecretName, httprouteSecretName),
			observabilityv1alpha1.CredentialBackendLogs:    credential.NewGatewayConfig(gatewayNamespace, "loki-ingress", "loki-httproute"),
			observabilityv1alpha1.CredentialBackendTraces:  credential.NewGatewayConfig(gatewayNamespace, "tempo-ingress", "tempo-httproute"),
		}

		reconciler = &AgentCredentialReconciler{
			Client:          k8sClient,
			Renderer:        credential.NewRenderer(k8sClient),
			Aggregator:      credential.NewAggregator(k8sClient, gatewayConfigs),
			finalizerHelper: NewFinalizerHelper(k8sClient, observabilityv1alpha1.AgentCredentialFinalizer),
		}
	})

	AfterEach(func() {
		// Best-effort cleanup of the per-test namespace.
		_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	})

	reconcileTwice := func(name string) {
		// First reconcile adds the finalizer; second reconcile renders+aggregates.
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
		Expect(err).NotTo(HaveOccurred())
	}

	It("renders a Secret with htpasswd, status, and aggregates the gateway secret", func() {
		cred := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "ac-1", Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend:   observabilityv1alpha1.CredentialBackendMetrics,
				AgentName: "agent-1",
			},
		}
		Expect(k8sClient.Create(ctx, cred)).To(Succeed())

		reconcileTwice("ac-1")

		By("rendering the per-credential Secret")
		secret := &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-1"}, secret)
		}, timeout, interval).Should(Succeed())

		Expect(string(secret.Data[credential.SecretKeyUsername])).To(Equal("agent-1"))
		Expect(secret.Data[credential.SecretKeyPassword]).NotTo(BeEmpty())
		Expect(string(secret.Data[credential.SecretKeyHtpasswd])).To(HavePrefix("agent-1:{SHA}"))

		By("setting Ready and GatewaySynced conditions and the SecretRef")
		updated := &observabilityv1alpha1.AgentCredential{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-1"}, updated)).To(Succeed())
		Expect(updated.Status.SecretRef).NotTo(BeNil())
		Expect(updated.Status.SecretRef.Name).To(Equal("ac-1"))
		Expect(getConditionStatus(updated, observabilityv1alpha1.AgentCredentialConditionReady)).To(Equal(metav1.ConditionTrue))
		Expect(getConditionStatus(updated, observabilityv1alpha1.AgentCredentialConditionGatewaySynced)).To(Equal(metav1.ConditionTrue))

		By("aggregating the entry into both gateway secrets")
		ingress := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: ingressSecretName}, ingress)).To(Succeed())
		Expect(string(ingress.Data[credential.IngressDataKey])).To(HavePrefix("agent-1:{SHA}"))

		httproute := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute)).To(Succeed())
		Expect(string(httproute.Data[credential.HTTPRouteDataKey])).To(HavePrefix("agent-1:{SHA}"))
	})

	It("preserves the password across reconciles", func() {
		cred := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "ac-2", Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend:   observabilityv1alpha1.CredentialBackendLogs,
				AgentName: "agent-2",
			},
		}
		Expect(k8sClient.Create(ctx, cred)).To(Succeed())
		reconcileTwice("ac-2")

		first := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-2"}, first)).To(Succeed())
		firstPassword := first.Data[credential.SecretKeyPassword]

		// Reconcile again — the password must be preserved.
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "ac-2"}})
		Expect(err).NotTo(HaveOccurred())

		second := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-2"}, second)).To(Succeed())
		Expect(second.Data[credential.SecretKeyPassword]).To(Equal(firstPassword))
	})

	It("respects spec.secretName when rendering the Secret", func() {
		cred := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "ac-3", Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend:    observabilityv1alpha1.CredentialBackendMetrics,
				AgentName:  "agent-3",
				SecretName: "ac-3-renamed-secret",
			},
		}
		Expect(k8sClient.Create(ctx, cred)).To(Succeed())
		reconcileTwice("ac-3")

		// CR-named secret should not exist.
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-3"}, &corev1.Secret{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-3-renamed-secret"}, secret)).To(Succeed())
		Expect(string(secret.Data[credential.SecretKeyUsername])).To(Equal("agent-3"))

		updated := &observabilityv1alpha1.AgentCredential{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-3"}, updated)).To(Succeed())
		Expect(updated.Status.SecretRef.Name).To(Equal("ac-3-renamed-secret"))
	})

	It("removes the entry from the gateway secret on deletion", func() {
		// Create two credentials so the gateway secret has more than one entry.
		credKeep := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "keep", Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend:   observabilityv1alpha1.CredentialBackendTraces,
				AgentName: "keep-agent",
			},
		}
		credDel := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "del", Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend:   observabilityv1alpha1.CredentialBackendTraces,
				AgentName: "del-agent",
			},
		}
		Expect(k8sClient.Create(ctx, credKeep)).To(Succeed())
		Expect(k8sClient.Create(ctx, credDel)).To(Succeed())
		reconcileTwice("keep")
		reconcileTwice("del")

		// Both entries are present in the gateway secret.
		ingress := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: "tempo-ingress"}, ingress)).To(Succeed())
		Expect(strings.Count(string(ingress.Data[credential.IngressDataKey]), "\n")).To(Equal(1))

		// Delete one credential — the controller's finalizer must rewrite the
		// gateway secret without the deleted entry.
		Expect(k8sClient.Delete(ctx, credDel)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "del"}})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: "tempo-ingress"}, ingress)).To(Succeed())
		got := string(ingress.Data[credential.IngressDataKey])
		Expect(got).To(HavePrefix("keep-agent:{SHA}"))
		Expect(got).NotTo(ContainSubstring("del-agent"))

		// The deleted CR is gone.
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "del"}, &observabilityv1alpha1.AgentCredential{})
			return apierrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
	})
})

func getConditionStatus(cred *observabilityv1alpha1.AgentCredential, condType string) metav1.ConditionStatus {
	for _, c := range cred.Status.Conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return metav1.ConditionUnknown
}
