package controller

import (
	"context"
	"crypto/sha1" //nolint:gosec // G505: htpasswd {SHA} is the format under test
	"encoding/base64"
	"fmt"
	"regexp"
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
			observabilityv1alpha1.CredentialBackendMetrics: credential.NewGatewayConfig(gatewayNamespace, httprouteSecretName),
			observabilityv1alpha1.CredentialBackendLogs:    credential.NewGatewayConfig(gatewayNamespace, "loki-httproute"),
			observabilityv1alpha1.CredentialBackendTraces:  credential.NewGatewayConfig(gatewayNamespace, "tempo-httproute"),
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
			ObjectMeta: metav1.ObjectMeta{Name: ac1, Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend: observabilityv1alpha1.CredentialBackendMetrics,
			},
		}
		Expect(k8sClient.Create(ctx, cred)).To(Succeed())

		reconcileTwice(ac1)

		By("rendering the per-credential Secret")
		secret := &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: ac1}, secret)
		}, timeout, interval).Should(Succeed())

		username := ns + "/" + ac1
		Expect(string(secret.Data[credential.SecretKeyUsername])).To(Equal(username))
		Expect(secret.Data[credential.SecretKeyPassword]).NotTo(BeEmpty())
		Expect(string(secret.Data[credential.SecretKeyHtpasswd])).To(HavePrefix(username + ":{SHA}"))

		By("setting Ready and GatewaySynced conditions and the SecretRef")
		updated := &observabilityv1alpha1.AgentCredential{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: ac1}, updated)).To(Succeed())
		Expect(updated.Status.SecretRef).NotTo(BeNil())
		Expect(updated.Status.SecretRef.Name).To(Equal(ac1))
		Expect(getConditionStatus(updated, observabilityv1alpha1.AgentCredentialConditionReady)).To(Equal(metav1.ConditionTrue))
		Expect(getConditionStatus(updated, observabilityv1alpha1.AgentCredentialConditionGatewaySynced)).To(Equal(metav1.ConditionTrue))

		By("aggregating the entry into the HTTPRoute gateway secret")
		// The aggregated htpasswd holds every metrics credential in the cluster, sorted, so
		// with randomized spec order other entries can precede ours: match the line, not the prefix.
		httproute := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute)).To(Succeed())
		Expect(string(httproute.Data[credential.HTTPRouteDataKey])).To(MatchRegexp(htpasswdLine(username)))
	})

	It("gives two credentials with the same agentName distinct usernames", func() {
		otherNs := fmt.Sprintf("ac-test-ns-other-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNs}})).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNs}})
		})

		for _, namespace := range []string{ns, otherNs} {
			cred := &observabilityv1alpha1.AgentCredential{
				ObjectMeta: metav1.ObjectMeta{Name: sharedName, Namespace: namespace},
				Spec: observabilityv1alpha1.AgentCredentialSpec{
					Backend:   observabilityv1alpha1.CredentialBackendMetrics,
					AgentName: "same-agent", //nolint:staticcheck // SA1019: the deprecated field is set on purpose to prove it is ignored.
				},
			}
			Expect(k8sClient.Create(ctx, cred)).To(Succeed())
			for range 2 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: sharedName}})
				Expect(err).NotTo(HaveOccurred())
			}
		}

		httproute := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute)).To(Succeed())
		aggregated := string(httproute.Data[credential.HTTPRouteDataKey])

		for _, namespace := range []string{ns, otherNs} {
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: sharedName}, secret)).To(Succeed())
			username := namespace + "/" + sharedName
			Expect(string(secret.Data[credential.SecretKeyUsername])).To(Equal(username))
			Expect(aggregated).To(MatchRegexp(htpasswdLine(username)))
		}
	})

	It("moves a Secret rendered under the old agentName username to namespace/name, keeping its password", func() {
		// A Secret as rendered by a previous release, keyed on spec.agentName.
		legacy := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: upgradedName, Namespace: ns},
			Data: map[string][]byte{
				credential.SecretKeyUsername: []byte("legacy-agent"),
				credential.SecretKeyPassword: []byte("legacy-password"),
				credential.SecretKeyHtpasswd: []byte("legacy-agent:{SHA}irrelevant"),
			},
		}
		Expect(k8sClient.Create(ctx, legacy)).To(Succeed())

		cred := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: upgradedName, Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend:   observabilityv1alpha1.CredentialBackendMetrics,
				AgentName: "legacy-agent", //nolint:staticcheck // SA1019: the deprecated field is set on purpose, as on credentials created before the upgrade.
			},
		}
		Expect(k8sClient.Create(ctx, cred)).To(Succeed())
		reconcileTwice(upgradedName)

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: upgradedName}, secret)).To(Succeed())
		username := ns + "/" + upgradedName
		Expect(string(secret.Data[credential.SecretKeyUsername])).To(Equal(username))
		Expect(string(secret.Data[credential.SecretKeyPassword])).To(Equal("legacy-password"))
		Expect(string(secret.Data[credential.SecretKeyHtpasswd])).To(Equal(username + ":{SHA}" + shaDigest("legacy-password")))

		httproute := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute)).To(Succeed())
		aggregated := string(httproute.Data[credential.HTTPRouteDataKey])
		Expect(aggregated).To(MatchRegexp(`(?m)^` + regexp.QuoteMeta(username+":{SHA}"+shaDigest("legacy-password")) + `$`))
		By("still accepting the legacy agentName with the same password during the transition")
		Expect(aggregated).To(MatchRegexp(`(?m)^` + regexp.QuoteMeta("legacy-agent:{SHA}"+shaDigest("legacy-password")) + `$`))
	})

	It("preserves the password across reconciles", func() {
		cred := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: ac2, Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend: observabilityv1alpha1.CredentialBackendLogs,
			},
		}
		Expect(k8sClient.Create(ctx, cred)).To(Succeed())
		reconcileTwice(ac2)

		first := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: ac2}, first)).To(Succeed())
		firstPassword := first.Data[credential.SecretKeyPassword]

		// Reconcile again — the password must be preserved.
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: ac2}})
		Expect(err).NotTo(HaveOccurred())

		second := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: ac2}, second)).To(Succeed())
		Expect(second.Data[credential.SecretKeyPassword]).To(Equal(firstPassword))
	})

	It("respects spec.secretName when rendering the Secret", func() {
		cred := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: ac3, Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{ // nolint:gosec // G101: Secret resource name, not a credential
				Backend:    observabilityv1alpha1.CredentialBackendMetrics,
				SecretName: "ac-3-renamed-secret",
			},
		}
		Expect(k8sClient.Create(ctx, cred)).To(Succeed())
		reconcileTwice(ac3)

		// CR-named secret should not exist.
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: ac3}, &corev1.Secret{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "ac-3-renamed-secret"}, secret)).To(Succeed())
		Expect(string(secret.Data[credential.SecretKeyUsername])).To(Equal(ns + "/" + ac3))

		updated := &observabilityv1alpha1.AgentCredential{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: ac3}, updated)).To(Succeed())
		Expect(updated.Status.SecretRef.Name).To(Equal("ac-3-renamed-secret"))
	})

	It("removes the entry from the gateway secret on deletion", func() {
		// Create two credentials so the gateway secret has more than one entry.
		credKeep := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: "keep", Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend: observabilityv1alpha1.CredentialBackendTraces,
			},
		}
		credDel := &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: del, Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend: observabilityv1alpha1.CredentialBackendTraces,
			},
		}
		Expect(k8sClient.Create(ctx, credKeep)).To(Succeed())
		Expect(k8sClient.Create(ctx, credDel)).To(Succeed())
		reconcileTwice("keep")
		reconcileTwice(del)

		// Both entries are present in the gateway secret.
		keepLine := htpasswdLine(ns + "/keep")
		delLine := htpasswdLine(ns + "/" + del)
		httproute := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: "tempo-httproute"}, httproute)).To(Succeed())
		Expect(string(httproute.Data[credential.HTTPRouteDataKey])).To(SatisfyAll(MatchRegexp(keepLine), MatchRegexp(delLine)))

		// Delete one credential — the controller's finalizer must rewrite the
		// gateway secret without the deleted entry.
		Expect(k8sClient.Delete(ctx, credDel)).To(Succeed())
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: del}})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: gatewayNamespace, Name: "tempo-httproute"}, httproute)).To(Succeed())
		got := string(httproute.Data[credential.HTTPRouteDataKey])
		Expect(got).To(MatchRegexp(keepLine))
		Expect(got).NotTo(MatchRegexp(delLine))

		// The deleted CR is gone.
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: del}, &observabilityv1alpha1.AgentCredential{})
			return apierrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
	})
})

const (
	sharedName   = "shared"
	upgradedName = "upgraded"
)

// shaDigest returns the htpasswd {SHA} digest of password, computed
// independently of the operator's own password package.
func shaDigest(password string) string {
	sum := sha1.Sum([]byte(password)) //nolint:gosec // G401: htpasswd {SHA} is the format under test
	return base64.StdEncoding.EncodeToString(sum[:])
}

// htpasswdLine matches the htpasswd line of the given username in an
// aggregated gateway htpasswd, which is shared by every spec and sorted.
func htpasswdLine(username string) string {
	return `(?m)^` + regexp.QuoteMeta(username) + `:\{SHA\}\S+$`
}

func getConditionStatus(cred *observabilityv1alpha1.AgentCredential, condType string) metav1.ConditionStatus {
	for _, c := range cred.Status.Conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return metav1.ConditionUnknown
}
