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

package validation

import (
	"strings"
	"testing"
)

func TestValidateSelector(t *testing.T) {
	tests := []struct {
		name        string
		selector    string
		shouldError bool
	}{
		{
			name:     "stream selector only",
			selector: `{scrape_job="audit-logs"}`,
		},
		{
			name:     "several matchers",
			selector: `{cluster_id="wc01", scrape_job="audit-logs"}`,
		},
		{
			name:     "line filter",
			selector: `{scrape_job="audit-logs"} |= "delete"`,
		},
		{
			name:     "regex line filter",
			selector: `{scrape_job="audit-logs"} |~ "de.*te"`,
		},
		{
			name:     "parse and filter",
			selector: `{scrape_job="audit-logs"} | json | verb="delete"`,
		},
		{
			// A keyword scan would reject this; the parser knows it is a line filter.
			name:     "aggregation name inside a line filter",
			selector: `{scrape_job="audit-logs"} |= "rate("`,
		},
		{
			name:        "aggregation",
			selector:    `sum by (verb) (rate({scrape_job="audit-logs"}[5m]))`,
			shouldError: true,
		},
		{
			name:        "range aggregation",
			selector:    `count_over_time({scrape_job="audit-logs"}[1h])`,
			shouldError: true,
		},
		{
			name:        "time range",
			selector:    `{scrape_job="audit-logs"}[5m]`,
			shouldError: true,
		},
		{
			name:        "empty stream selector",
			selector:    `{}`,
			shouldError: true,
		},
		{
			name:        "not logql",
			selector:    "audit logs please",
			shouldError: true,
		},
		{
			name:        "empty",
			selector:    "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSelector(tt.selector)
			if tt.shouldError && err == nil {
				t.Errorf("ValidateSelector(%q) succeeded, expected an error", tt.selector)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ValidateSelector(%q) failed: %v", tt.selector, err)
			}
			// The message is the only thing the customer sees, so it has to name the
			// subset rather than just echo the parser.
			if err != nil && !strings.Contains(err.Error(), SupportedSelectorSubset) {
				t.Errorf("ValidateSelector(%q) error does not describe the supported subset: %v", tt.selector, err)
			}
		})
	}
}
