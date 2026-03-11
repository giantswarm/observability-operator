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
	var matchFound *httpRequest

	for i := range records {
		if err := assertExpectationRecord(expectation, records[i]); err == nil {
			matchFound = &records[i]
			break
		}
	}

	if expectation.Negate {
		if matchFound != nil {
			return fmt.Errorf("unexpected matching request found\nURL: %s\nHeaders: %v\nBody: %s",
				matchFound.URL.String(), matchFound.Header, matchFound.BodyData)
		}
		return nil
	}

	if matchFound == nil {
		receivedURLs := make([]string, 0, len(records))
		for _, r := range records {
			receivedURLs = append(receivedURLs, r.URL.String())
		}
		return fmt.Errorf("no request matching URL %q found\nReceived URLs:\n  %s",
			expectation.URL, strings.Join(receivedURLs, "\n  "))
	}

	return nil
}

// assertExpectationRecord checks a single record against an expectation.
// When the URL matches, it continues to validate headers and body so the caller
// receives a precise failure reason rather than a generic URL mismatch.
func assertExpectationRecord(expectation Expectation, record httpRequest) error {
	if !strings.Contains(record.URL.String(), expectation.URL) {
		return fmt.Errorf("URL %q does not contain %q", record.URL.String(), expectation.URL)
	}

	// Validate Headers
	for k, v := range expectation.Headers {
		if got := record.Header.Get(k); got != v {
			return fmt.Errorf("header %q: expected %q, got %q", k, v, got)
		}
	}

	// Validate body data.
	// Body data format for receiver can be found at https://github.com/grafana/prometheus-alertmanager/tree/main/notify
	for _, part := range expectation.BodyParts {
		if !strings.Contains(string(record.BodyData), part) {
			return fmt.Errorf("body does not contain %q\nFull body: %s", part, record.BodyData)
		}
	}

	return nil
}
