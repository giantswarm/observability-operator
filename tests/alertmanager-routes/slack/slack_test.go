package main

import (
	"testing"
	"time"

	helper "github.com/giantswarm/observability-operator/tests/alertmanager-routes"
)

// TestTeamsSlackRouting tests Slack routing for multiple teams in a single test
func TestTeamsSlackRouting(t *testing.T) {
	testCases := []helper.TestCase{
		// Atlas team - page alert
		{
			Alert: helper.Alert{
				Name: "TestSlackAtlasPageAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "atlas",
					"severity":     "page",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-atlas-test","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackAtlasPageAlert - Team atlas"`},
				},
			},
		},
		// Atlas team - notify alert
		{
			Alert: helper.Alert{
				Name: "TestSlackAtlasNotifyAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "atlas",
					"severity":     "notify",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-atlas-test","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackAtlasNotifyAlert - Team atlas"`},
				},
			},
		},
		// Atlas team - Inhibition (should NOT go to Slack)
		{
			Alert: helper.Alert{
				Name: "Inhibition",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "atlas",
					"severity":     "notify",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`Inhibition`},
					Negate:    true,
				},
			},
		},
		// Atlas team - Heartbeat (should NOT go to Slack)
		{
			Alert: helper.Alert{
				Name: "Heartbeat",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "atlas",
					"severity":     "notify",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`Heartbeat`},
					Negate:    true,
				},
			},
		},
		// Phoenix team - silent sloth alert
		{
			Alert: helper.Alert{
				Name: "TestSlackPhoenixSlothAlert",
				Labels: map[string]string{
					"cluster_id":     "test-cluster",
					"installation":   "test-installation",
					"pipeline":       "stable",
					"provider":       "aws",
					"team":           "phoenix",
					"sloth_severity": "page",
					"silence":        "true",
					"status":         "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-phoenix-test","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackPhoenixSlothAlert - Team phoenix"`},
				},
			},
		},
		// Shield team - page alert
		{
			Alert: helper.Alert{
				Name: "TestSlackShieldPageAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "shield",
					"severity":     "page",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-shield","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackShieldPageAlert - Team shield"`},
				},
			},
		},
		// Shield team - notify alert
		{
			Alert: helper.Alert{
				Name: "TestSlackShieldNotifyAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "shield",
					"severity":     "notify",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-shield","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackShieldNotifyAlert - Team shield"`},
				},
			},
		},
		// Rocket team - page alert
		{
			Alert: helper.Alert{
				Name: "TestSlackRocketPageAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "rocket",
					"severity":     "page",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-rocket-test","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackRocketPageAlert - Team rocket"`},
				},
			},
		},
		// Rocket team - notify alert
		{
			Alert: helper.Alert{
				Name: "TestSlackRocketNotifyAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "rocket",
					"severity":     "notify",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-rocket-test","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackRocketNotifyAlert - Team rocket"`},
				},
			},
		},
		// Honeybadger team - page alert
		{
			Alert: helper.Alert{
				Name: "TestSlackHoneybadgerPageAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "honeybadger",
					"severity":     "page",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-honeybadger","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackHoneybadgerPageAlert - Team honeybadger"`},
				},
			},
		},
		// Honeybadger team - notify alert
		{
			Alert: helper.Alert{
				Name: "TestSlackHoneybadgerNotifyAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "honeybadger",
					"severity":     "notify",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-honeybadger","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackHoneybadgerNotifyAlert - Team honeybadger"`},
				},
			},
		},
		// Tenet team - notify alert
		{
			Alert: helper.Alert{
				Name: "TestSlackTenetNotifyAlert",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "tenet",
					"severity":     "notify",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#alert-tenet","username":"Alertmanager","attachments":[{"title":"FIRING[1] TestSlackTenetNotifyAlert - Team tenet"`},
				},
			},
		},
		// Falco alert
		{
			Alert: helper.Alert{
				Name: "Falco",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "security",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#noise-falco","username":"Alertmanager","attachments":[{"title":"FIRING[1] Falco - Team security"`},
				},
			},
		},
		// Falco alert with suffix
		{
			Alert: helper.Alert{
				Name: "FalcoAnything",
				Labels: map[string]string{
					"cluster_id":   "test-cluster",
					"installation": "test-installation",
					"pipeline":     "stable",
					"provider":     "aws",
					"team":         "security",
					"status":       "firing",
				},
			},
			Expectations: []helper.Expectation{
				{
					URL:       "https://slack.com/api/chat.postMessage",
					BodyParts: []string{`{"channel":"#noise-falco","username":"Alertmanager","attachments":[{"title":"FIRING[1] FalcoAnything - Team security"`},
				},
			},
		},
	}

	helper.RunAlertmanagerIntegrationTest(t, testCases, 30*time.Second)
}
