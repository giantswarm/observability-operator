package alertmanagerroutes

import (
	"fmt"
	"strings"
)

// TestCase represents a single alert test case with its expectations
// Note that all test cases are run together, meaning that all alerts are sent
// at once to Alertmanager before validating each TestCase expectations.
type TestCase struct {
	Alert
	// Expectations is the list of expectations to be validated against the received requests
	Expectations []Expectation
}

// Alert represents a single alert to be sent to Alertmanager
type Alert struct {
	// Name of the Alert, should ideally be unique across all test cases
	// The Name is added as "alertname" in the set of labels.
	Name string

	// Labels represents the set of labels representing the alert
	Labels map[string]string
}

// Expectation represents a matcher for an expected HTTP request
type Expectation struct {
	// URL is a partial match for the request URL (including scheme, host, path, and query string)
	URL string
	// Headers is a map of expected headers and their values
	Headers map[string]string
	// BodyParts is a list of strings to be partially matched against the request body. All parts are ANDed
	BodyParts []string
	// Negate indicates whether the expectation is negated (i.e., the request should NOT be present)
	Negate bool
}

// AssertExpectation validates that a given expectation is present in a set of records.
// It does so by matching the Host and RequestURI of the expectation url against the request.
// The expectation.body is partially matched against the request body.
func AssertExpectation(expectation Expectation, records []httpRequest) error {
	var (
		err        error
		matchFound *httpRequest
	)

	for _, r := range records {
		err = assertExpectationRecord(expectation, r)
		if err == nil {
			matchFound = &r
			break
		}
	}

	if expectation.Negate {
		if matchFound != nil {
			return fmt.Errorf("==> unexpected matching request found\nURL: %s\nHeaders: %v\nBody: %s", matchFound.URL.String(), matchFound.Header, matchFound.BodyData)
		}

		// Valid when the condition is negated and no match is found
		return nil
	}

	if matchFound == nil {
		return fmt.Errorf("==> no matching request found")
	}

	// Valid when a match is found
	return nil
}

func assertExpectationRecord(expectation Expectation, record httpRequest) error {
	// Validate URL
	if strings.Contains(record.URL.String(), expectation.URL) {

		// Validate Headers
		for k, v := range expectation.Headers {
			if record.Header.Get(k) != v {
				return fmt.Errorf("==> invalid %q header value, expected %q got %q", k, v, record.Header.Get(k))
			}
		}

		// Validate body data
		// Body data format for receiver can be found at https://github.com/grafana/prometheus-alertmanager/tree/main/notify
		for _, part := range expectation.BodyParts {
			if !strings.Contains(string(record.BodyData), part) {
				return fmt.Errorf("==> invalid request body\n%s", string(record.BodyData))
			}
		}

		// Expectation is valid
		return nil
	}

	return fmt.Errorf("==> invalid URL, expected %q got %q", expectation.URL, record.URL.String())
}
