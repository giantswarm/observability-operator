package alertmanagerroutes

import (
	"net"
	"testing"
	"time"

	"github.com/sanity-io/litter"
)

// RunAlertmanagerIntegrationTest is the main entry point for running Alertmanager routes integration tests.
// It evaluates multiple test cases by sending all alerts at once to Alertmanager, wait for the given waitTime and then verifying the received notifications against the expectations.
// Each TestCase is run as a separate test (using t.Run).
// Various parameters are configured using flags, see other helper test files for flags definitions.
func RunAlertmanagerIntegrationTest(t *testing.T, testCases []TestCase, waitTime time.Duration) {
	// Using the test name as tenant ID
	tenantID := t.Name()

	// Start the HTTP requests receiver
	receiver, err := NewHTTPReceiver(t)
	if err != nil {
		t.Fatalf("failed to create HTTP receiver: %v", err)
	}
	receiver.Start()
	defer receiver.Stop()

	_, receiverPort, err := net.SplitHostPort(receiver.GetAddress())
	if err != nil {
		t.Fatalf("failed to get receiver port: %v", err)
	}

	// Initialize the Alertmanager client
	amClient := NewAlertmanagerClient(t, alertmanagerURL, tenantID, receiverPort)

	// Upload Alertmanager configuration
	err = amClient.Configure()
	if err != nil {
		t.Fatalf("failed to configure Alertmanager: %v", err)
	}
	defer amClient.UnConfigure()

	// Send alerts
	for _, tc := range testCases {
		err = amClient.PostAlerts(tc.Alert)
		if err != nil {
			t.Errorf("failed to send alert: %v", err)
		}
	}

	// Wait for Alertmanager notification requests to be received
	t.Logf("Waiting %s to receive Alertmanager notifications...", waitTime)
	time.Sleep(waitTime)

	// Assert expectation match received Alertmanager notifications
	records := receiver.GetHTTPRequests()

	for _, tc := range testCases {
		t.Run(tc.Alert.Name, func(t *testing.T) {
			if len(tc.Expectations) > 0 && len(records) <= 0 {
				// Fail when we expect notifications but none were received
				t.Fatalf("no Alertmanager notifications received")
			} else if len(tc.Expectations) <= 0 && len(records) > 0 {
				// Fail when we do not expect any notifications but some were received
				t.Fatalf("unexpected Alertmanager notifications received")
			}

			for i, expectation := range tc.Expectations {
				err = AssertExpectation(expectation, records)
				if err != nil {
					t.Errorf("assertion failed for expectation[%d]\n%s\n\n%v\n\n", i, litter.Sdump(expectation), err)
				}
			}
		})
	}
}
