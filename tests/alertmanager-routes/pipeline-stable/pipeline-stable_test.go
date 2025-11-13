//go:build integration

package main

import (
	"testing"
	"time"

	helper "github.com/giantswarm/observability-operator/tests/alertmanager-routes"
)

// TestPipelineStable tests multiple routing scenarios for "pipeline" related routes
// This combines PagerDuty, and Slack routing tests
func TestPipelineStable(t *testing.T) {
	testCases := []helper.TestCase{
		// Page alert - SHOULD go to PagerDuty
		{
			Alert: helper.Alert{
				Name: "TestStablePageAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"severity":     "page",
					"status":       "firing",
					"team":         "foo",
				},
			},
			Expectations: []helper.Expectation{
				// PagerDuty expectation for pipeline=stable
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`"routing_key":"foo-pagerduty-token"`, `"event_action":"trigger","payload":{"summary":"test-installation-test-cluster - TestStablePageAlert","source":"Alertmanager"`},
				},
				// Page alerts SHOULD NOT go to Slack for team foo
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`TestStablePageAlert`},
					Negate:    true,
				},
			},
		},
		// Page alert with all_pipelines=true - SHOULD go to PagerDuty
		{
			Alert: helper.Alert{
				Name: "TestStableAllPipelineAlert",
				Labels: map[string]string{
					"all_pipelines": "true",
					"cluster_id":    "test-cluster",
					"installation":  "test-installation",
					"pipeline":      "stable",
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
					BodyParts: []string{`"routing_key":"foo-pagerduty-token"`, `"event_action":"trigger","payload":{"summary":"test-installation-test-cluster - TestStableAllPipelineAlert","source":"Alertmanager"`},
				},
				// Page alerts SHOULD NOT go to Slack for team foo
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`TestStableAllPipelineAlert`},
					Negate:    true,
				},
			},
		},
		// Atlas page alert - SHOULD NOT go to Slack
		{
			Alert: helper.Alert{
				Name: "TestStablePageAtlasAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"severity":     "page",
					"status":       "firing",
					"team":         "atlas",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`TestStablePageAtlasAlert`},
					Negate:    true,
				},
			},
		},
		// Atlas notify alert - SHOULD go to Slack
		{
			Alert: helper.Alert{
				Name: "TestStableNotifyAtlasAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"severity":     "notify",
					"status":       "firing",
					"team":         "atlas",
				},
			},
			Expectations: []helper.Expectation{
				// Notify alerts SHOULD be delivered to Slack for team Atlas
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-atlas","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestStableNotifyAtlasAlert - Team atlas"`},
				},
				// Notify alerts SHOULD NOT be delivered to PagerDuty
				{
					URL:       "https://events.eu.pagerduty.com/v2/enqueue",
					BodyParts: []string{`TestStableNotifyAtlasAlert`},
					Negate:    true,
				},
			},
		},
	}

	helper.RunAlertmanagerIntegrationTest(t, testCases, 30*time.Second)
}
