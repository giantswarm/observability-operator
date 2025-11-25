//go:build integration

package main

import (
	"testing"
	"time"

	helper "github.com/giantswarm/observability-operator/tests/alertmanager-routes"
)

// TestPipelineStableTestingRouting tests multiple routing scenarios including PagerDuty, and blackhole routes
func TestPipelineStableTestingRouting(t *testing.T) {
	testCases := []helper.TestCase{
		// Page alert - should go to PagerDuty
		// TODO: cannot test this due to time_intervals
		//{
		//	Alert: helper.Alert{
		//		Name: "TestStableTestingPageAlert",
		//		Labels: map[string]string{
		//			"cluster_id":   "test-cluster",
		//			"installation": "test-installation",
		//			"pipeline":     "stable-testing",
		//			"provider":     "aws",
		//			"severity":     "page",
		//			"status":       "firing",
		//			"team":         "foo",
		//		},
		//	},
		//	Expectations: []helper.Expectation{
		//		{
		//			URL:       "https://events.eu.pagerduty.com/v2/enqueue",
		//			BodyParts: []string{`"routing_key":"foo-pagerduty-token"`, `"alertname":"TestStableTestingPageAlert"`},
		//		},
		//	},
		//},
		// Page alert with all_pipelines=true - should go to PagerDuty
		{
			Alert: helper.Alert{
				Name: "TestStableTestingAllPipelineAlert",
				Labels: map[string]string{
					"all_pipelines": "true",
					"cluster_id":    "test-cluster",
					"installation":  "test-installation",
					"pipeline":      "stable-testing",
					"provider":      "aws",
					"severity":      "page",
					"status":        "firing",
					"team":          "foo",
				},
			},
			Expectations: []helper.Expectation{
				// PagerDuty expectation for all_pipelines=true
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`"routing_key":"foo-pagerduty-token"`, `"alertname":"TestStableTestingAllPipelineAlert"`, `"all_pipelines":"true"`},
				},
			},
		},
		// Workload cluster alert - should be dropped (blackhole)
		{
			Alert: helper.Alert{
				Name: "TestStableTestingWCAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"cluster_type": "workload_cluster",
					"installation": "test-installation",
					"pipeline":     "stable-testing",
					"provider":     "aws",
					"severity":     "page",
					"status":       "firing",
					"team":         "foo",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`TestStableTestingWCAlert`},
					Negate:    true,
				},
			},
		},
		// Test cluster alert - should be dropped (blackhole)
		{
			Alert: helper.Alert{
				Name: "TestStableTestingTestClusterAlert",
				Labels: map[string]string{
					"cluster_id":   "t-anything",
					"installation": "test-installation",
					"pipeline":     "stable-testing",
					"provider":     "aws",
					"severity":     "page",
					"status":       "firing",
					"team":         "foo",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`TestStableTestingTestClusterAlert`},
					Negate:    true,
				},
			},
		},
		// ClusterUnhealthyPhase alert - should be dropped
		{
			Alert: helper.Alert{
				Name: "ClusterUnhealthyPhase",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable-testing",
					"provider":     "aws",
					"severity":     "page",
					"status":       "firing",
					"team":         "foo",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`ClusterUnhealthyPhase`},
					Negate:    true,
				},
			},
		},
		// WorkloadClusterApp alert - should be dropped
		{
			Alert: helper.Alert{
				Name: "WorkloadClusterApp",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable-testing",
					"provider":     "aws",
					"severity":     "page",
					"status":       "firing",
					"team":         "foo",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`WorkloadClusterApp`},
					Negate:    true,
				},
			},
		},
		// ManagementClusterAppFailed for org namespace - should be dropped
		{
			Alert: helper.Alert{
				Name: "ManagementClusterAppFailed",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"namespace":    "org-foo",
					"pipeline":     "stable-testing",
					"provider":     "aws",
					"severity":     "page",
					"status":       "firing",
					"team":         "foo",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`ManagementClusterAppFailed`},
					Negate:    true,
				},
			},
		},
	}

	helper.RunAlertmanagerIntegrationTest(t, testCases, 30*time.Second)
}
