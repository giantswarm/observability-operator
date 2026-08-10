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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/internal/webhook/validation"
)

var _ = Describe("LogExport Validation", func() {
	var (
		ctx       context.Context
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = createNamespace(ctx, "logexport-webhook-ns-")
	})

	// newLE builds a valid LogExport with an s3 destination, overriding the selector.
	newLE := func(name, selector string) *observabilityv1alpha1.LogExport {
		return &observabilityv1alpha1.LogExport{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: observabilityv1alpha1.LogExportSpec{
				Selector: selector,
				Destination: observabilityv1alpha1.LogExportDestination{
					Type: observabilityv1alpha1.LogExportDestinationS3,
					S3: &observabilityv1alpha1.S3Destination{
						Bucket: "audit-export",
						Region: "eu-west-2",
					},
				},
			},
		}
	}

	Context("Create (webhook-level selector validation)", func() {
		It("accepts a stream selector", func() {
			Expect(k8sClient.Create(ctx, newLE("selector-only", `{scrape_job="audit-logs"}`))).To(Succeed())
		})

		It("accepts a selector with line filters", func() {
			Expect(k8sClient.Create(ctx, newLE("line-filter", `{scrape_job="audit-logs"} |= "delete"`))).To(Succeed())
		})

		It("accepts a parse-and-filter expression", func() {
			Expect(k8sClient.Create(ctx, newLE("parse-filter", `{scrape_job="audit-logs"} | json | verb="delete"`))).To(Succeed())
		})

		It("rejects an aggregation", func() {
			err := k8sClient.Create(ctx, newLE("aggregation", `sum by (verb) (rate({scrape_job="audit-logs"}[5m]))`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("aggregations are not supported"))
		})

		It("rejects a time range", func() {
			err := k8sClient.Create(ctx, newLE("time-range", `{scrape_job="audit-logs"}[5m]`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("time ranges are not supported"))
		})

		It("rejects a stage the exporter cannot render", func() {
			err := k8sClient.Create(ctx, newLE("line-format", `{scrape_job="audit-logs"} | line_format "{{ .verb }}"`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`the stage "| line_format`))
		})

		It("rejects a selector that matches every stream", func() {
			err := k8sClient.Create(ctx, newLE("match-all", `{scrape_job=~".+"}`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("needs at least one exact match"))
		})

		It("names the supported subset in the error", func() {
			err := k8sClient.Create(ctx, newLE("bad-syntax", "audit logs please"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(validation.SupportedSelectorSubset))
			Expect(err.Error()).To(ContainSubstring("spec.selector"))
		})

		It("rejects a selector that becomes invalid on update", func() {
			le := newLE("update-invalid", `{scrape_job="audit-logs"}`)
			Expect(k8sClient.Create(ctx, le)).To(Succeed())

			le.Spec.Selector = `count_over_time({scrape_job="audit-logs"}[1h])`
			Expect(k8sClient.Update(ctx, le)).ToNot(Succeed())
		})
	})

	// The CRD's discriminated-union rules shipped in #928 without tests.
	Context("Create (CRD-level validation)", func() {
		It("rejects type s3 without an s3 block", func() {
			le := newLE("s3-missing", `{scrape_job="audit-logs"}`)
			le.Spec.Destination.S3 = nil
			err := k8sClient.Create(ctx, le)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.destination.s3 is required when type is 's3'"))
		})

		It("rejects an s3 block when type is loki", func() {
			le := newLE("loki-with-s3", `{scrape_job="audit-logs"}`)
			le.Spec.Destination.Type = observabilityv1alpha1.LogExportDestinationLoki
			le.Spec.Destination.Loki = &observabilityv1alpha1.LokiDestination{URL: "https://logs.example.com/loki/api/v1/push"}
			err := k8sClient.Create(ctx, le)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.destination.s3 may only be set when type is 's3'"))
		})

		It("rejects an unknown destination type", func() {
			le := newLE("bad-type", `{scrape_job="audit-logs"}`)
			le.Spec.Destination.Type = "gcs"
			le.Spec.Destination.S3 = nil
			err := k8sClient.Create(ctx, le)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("supported values"))
		})

		It("accepts a loki destination", func() {
			le := newLE("loki-ok", `{scrape_job="audit-logs"}`)
			le.Spec.Destination.Type = observabilityv1alpha1.LogExportDestinationLoki
			le.Spec.Destination.S3 = nil
			le.Spec.Destination.Loki = &observabilityv1alpha1.LokiDestination{URL: "https://logs.example.com/loki/api/v1/push"}
			Expect(k8sClient.Create(ctx, le)).To(Succeed())
		})
	})
})
