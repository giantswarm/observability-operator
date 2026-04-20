/*
Copyright 2026.

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

package v1alpha1

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

var _ = Describe("AgentCredential Validation", func() {
	var (
		ctx context.Context
		ns  string
	)

	BeforeEach(func() {
		ctx = context.Background()
		ns = fmt.Sprintf("ac-webhook-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
	})

	newAC := func(name, agentName string, backend observabilityv1alpha1.CredentialBackend) *observabilityv1alpha1.AgentCredential {
		return &observabilityv1alpha1.AgentCredential{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: observabilityv1alpha1.AgentCredentialSpec{
				Backend:   backend,
				AgentName: agentName,
			},
		}
	}

	Context("Create (CRD-level validation)", func() {
		It("accepts a valid AgentCredential", func() {
			ac := newAC("valid-ac", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)
			Expect(k8sClient.Create(ctx, ac)).To(Succeed())
		})

		It("rejects an agentName containing ':'", func() {
			ac := newAC("bad-agent", "bad:agent", observabilityv1alpha1.CredentialBackendMetrics)
			err := k8sClient.Create(ctx, ac)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("should match"))
		})

		It("rejects an unknown backend", func() {
			ac := &observabilityv1alpha1.AgentCredential{
				ObjectMeta: metav1.ObjectMeta{Name: "bad-backend", Namespace: ns},
				Spec: observabilityv1alpha1.AgentCredentialSpec{
					Backend:   observabilityv1alpha1.CredentialBackend("invalid"),
					AgentName: "agent-a",
				},
			}
			err := k8sClient.Create(ctx, ac)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("supported values"))
		})

		It("rejects a secretName that isn't a DNS-1123 subdomain", func() {
			ac := newAC("bad-secret-name", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)
			ac.Spec.SecretName = "Invalid_Secret_Name"
			err := k8sClient.Create(ctx, ac)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("should match"))
		})

		It("rejects an empty agentName", func() {
			ac := newAC("empty-agent", "", observabilityv1alpha1.CredentialBackendMetrics)
			err := k8sClient.Create(ctx, ac)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Create (webhook-level uniqueness)", func() {
		It("rejects a duplicate (backend, agentName) in a different namespace", func() {
			first := newAC("first", "shared-agent", observabilityv1alpha1.CredentialBackendMetrics)
			Expect(k8sClient.Create(ctx, first)).To(Succeed())

			otherNs := fmt.Sprintf("%s-other", ns)
			Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNs}})).To(Succeed())

			dup := &observabilityv1alpha1.AgentCredential{
				ObjectMeta: metav1.ObjectMeta{Name: "dup", Namespace: otherNs},
				Spec: observabilityv1alpha1.AgentCredentialSpec{
					Backend:   observabilityv1alpha1.CredentialBackendMetrics,
					AgentName: "shared-agent",
				},
			}
			err := k8sClient.Create(ctx, dup)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already used"))
		})

		It("accepts the same agentName across different backends", func() {
			metricsAC := newAC("metrics-ac", "dual-agent", observabilityv1alpha1.CredentialBackendMetrics)
			logsAC := newAC("logs-ac", "dual-agent", observabilityv1alpha1.CredentialBackendLogs)
			Expect(k8sClient.Create(ctx, metricsAC)).To(Succeed())
			Expect(k8sClient.Create(ctx, logsAC)).To(Succeed())
		})
	})

	Context("Update (webhook-level immutability)", func() {
		var ac *observabilityv1alpha1.AgentCredential

		BeforeEach(func() {
			ac = newAC("immutable-ac", "agent-i", observabilityv1alpha1.CredentialBackendMetrics)
			ac.Spec.SecretName = "immutable-ac-secret"
			Expect(k8sClient.Create(ctx, ac)).To(Succeed())
		})

		It("rejects mutating spec.backend", func() {
			ac.Spec.Backend = observabilityv1alpha1.CredentialBackendLogs
			err := k8sClient.Update(ctx, ac)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("backend is immutable"))
		})

		It("rejects mutating spec.agentName", func() {
			ac.Spec.AgentName = "agent-renamed"
			err := k8sClient.Update(ctx, ac)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("agentName is immutable"))
		})

		It("rejects mutating spec.secretName", func() {
			ac.Spec.SecretName = "renamed-secret"
			err := k8sClient.Update(ctx, ac)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("secretName is immutable"))
		})

		It("accepts an update that leaves spec unchanged", func() {
			if ac.Labels == nil {
				ac.Labels = map[string]string{}
			}
			ac.Labels["touched"] = "yes"
			Expect(k8sClient.Update(ctx, ac)).To(Succeed())
		})
	})

	Context("Update (uniqueness skips self)", func() {
		It("accepts updating an AgentCredential in place", func() {
			ac := newAC("self-update", "self-agent", observabilityv1alpha1.CredentialBackendTraces)
			Expect(k8sClient.Create(ctx, ac)).To(Succeed())

			// A no-op metadata change — the uniqueness check must skip the same UID.
			if ac.Annotations == nil {
				ac.Annotations = map[string]string{}
			}
			ac.Annotations["observability.giantswarm.io/note"] = "reconciled"
			Expect(k8sClient.Update(ctx, ac)).To(Succeed())
		})
	})

	AfterEach(func() {
		// Best-effort: drop every AC in the per-test namespace so later tests don't
		// hit uniqueness collisions against stragglers.
		list := &observabilityv1alpha1.AgentCredentialList{}
		if err := k8sClient.List(ctx, list); err == nil {
			for i := range list.Items {
				_ = k8sClient.Delete(ctx, &list.Items[i])
			}
		}
		_ = k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	})
})
