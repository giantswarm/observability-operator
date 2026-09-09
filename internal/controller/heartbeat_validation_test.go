package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// These specs exercise the Heartbeat CRD's schema against a real API server.
// There is no reconciler yet -- the union is only worth anything if the API
// server enforces it, and CEL rules are easy to write in a way that never fires.
var _ = Describe("Heartbeat CRD validation", func() {
	var (
		ctx context.Context
		ns  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		ns = fmt.Sprintf("hb-test-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	})

	newHeartbeat := func(name string, provider observabilityv1alpha1.HeartbeatProvider) *observabilityv1alpha1.Heartbeat {
		return &observabilityv1alpha1.Heartbeat{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec:       observabilityv1alpha1.HeartbeatSpec{Provider: provider},
		}
	}

	validCronitor := func() *observabilityv1alpha1.CronitorProvider {
		return &observabilityv1alpha1.CronitorProvider{
			CredentialsRef: corev1.LocalObjectReference{Name: "cronitor-credentials"},
			Schedule:       "every 30 minutes",
			GraceSeconds:   1800,
		}
	}

	It("accepts a cronitor provider with its block set", func() {
		hb := newHeartbeat("valid", observabilityv1alpha1.HeartbeatProvider{
			Type:     observabilityv1alpha1.HeartbeatProviderCronitor,
			Cronitor: validCronitor(),
		})
		Expect(k8sClient.Create(ctx, hb)).To(Succeed())
	})

	It("rejects type cronitor with no cronitor block", func() {
		hb := newHeartbeat("missing-block", observabilityv1alpha1.HeartbeatProvider{
			Type: observabilityv1alpha1.HeartbeatProviderCronitor,
		})
		err := k8sClient.Create(ctx, hb)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.provider.cronitor is required when type is 'cronitor'"))
	})

	It("rejects a cronitor block under a different type", func() {
		// While cronitor is the only enum member, the enum rejects this before the
		// "may only be set when type is 'cronitor'" rule is reached. The rule is
		// kept so that adding a second variant does not silently allow a mismatch.
		hb := newHeartbeat("wrong-type", observabilityv1alpha1.HeartbeatProvider{
			Type:     observabilityv1alpha1.HeartbeatProviderType("pingdom"),
			Cronitor: validCronitor(),
		})
		err := k8sClient.Create(ctx, hb)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.provider.type"))
	})

	It("rejects an empty credentialsRef", func() {
		cronitor := validCronitor()
		cronitor.CredentialsRef = corev1.LocalObjectReference{}
		hb := newHeartbeat("no-credentials", observabilityv1alpha1.HeartbeatProvider{
			Type:     observabilityv1alpha1.HeartbeatProviderCronitor,
			Cronitor: cronitor,
		})
		err := k8sClient.Create(ctx, hb)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("credentialsRef.name must not be empty"))
	})

	It("rejects a key that cannot be a ping URL path segment", func() {
		cronitor := validCronitor()
		cronitor.Key = "mimir/gazelle"
		hb := newHeartbeat("bad-key", observabilityv1alpha1.HeartbeatProvider{
			Type:     observabilityv1alpha1.HeartbeatProviderCronitor,
			Cronitor: cronitor,
		})
		err := k8sClient.Create(ctx, hb)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.provider.cronitor.key"))
	})
})
