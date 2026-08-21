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
		name string
		// wantErr is the fragment the message must contain; empty means the selector is
		// accepted. Asserting the fragment rather than "some error" is the point: every
		// rejection has to explain its own reason.
		selector string
		wantErr  string
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
			name:     "exact match alongside a regex matcher",
			selector: `{scrape_job="audit-logs", cluster_id=~"wc.*"}`,
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
			name:     "negative line filters",
			selector: `{scrape_job="audit-logs"} != "get" !~ "wat.*"`,
		},
		{
			// Line filters reach Alloy verbatim, so anything the parser accepts works.
			name:     "or line filter",
			selector: `{scrape_job="audit-logs"} |= "delete" or "create"`,
		},
		{
			name:     "ip line filter",
			selector: `{scrape_job="audit-logs"} |= ip("1.2.3.4")`,
		},
		{
			// A keyword scan would reject this; the parser knows it is a line filter.
			name:     "aggregation name inside a line filter",
			selector: `{scrape_job="audit-logs"} |= "rate("`,
		},
		{
			name:     "json parser alone",
			selector: `{scrape_job="audit-logs"} | json`,
		},
		{
			name:     "parse and filter",
			selector: `{scrape_job="audit-logs"} | json | verb="delete"`,
		},
		{
			// The four string comparisons pin stringMatcher's type switch: Loki picks a
			// different LabelFilterer implementation per operator.
			name:     "not-equal label filter",
			selector: `{scrape_job="audit-logs"} | json | verb!="get"`,
		},
		{
			name:     "regex label filter",
			selector: `{scrape_job="audit-logs"} | json | user=~"admin.*"`,
		},
		{
			name:     "negative regex label filter",
			selector: `{scrape_job="audit-logs"} | json | user!~"system:.*"`,
		},
		{
			name:     "chained label filters",
			selector: `{scrape_job="audit-logs"} | json | verb="delete" | user=~"admin.*"`,
		},
		{
			name:     "label filter without a parser",
			selector: `{scrape_job="audit-logs"} | cluster_id="wc01"`,
		},
		{
			// Not an exact match, but it does narrow: the renderer handles it, and Loki's
			// validation is satisfied because `.+` cannot match empty.
			name:     "scoped regex matcher",
			selector: `{namespace=~"prod-.*"}`,
		},
		{
			name:     "match everything with a label",
			selector: `{job=~".+"}`,
		},
		{
			// The empty-value matcher is a filter, not a matcher; `job=~".+"` is what
			// satisfies Loki's validation.
			name:     "empty value equality alongside a regex",
			selector: `{job=~".+", scrape_job=""}`,
		},

		// Not a log selector at all.
		{
			name:     "aggregation",
			selector: `sum by (verb) (rate({scrape_job="audit-logs"}[5m]))`,
			wantErr:  "aggregations are not supported",
		},
		{
			name:     "range aggregation",
			selector: `count_over_time({scrape_job="audit-logs"}[1h])`,
			wantErr:  "aggregations are not supported",
		},
		{
			name:     "scalar literal",
			selector: `1`,
			wantErr:  "returns a value rather than log lines",
		},
		{
			name:     "vector",
			selector: `vector(0)`,
			wantErr:  "returns a value rather than log lines",
		},
		{
			name:     "time range",
			selector: `{scrape_job="audit-logs"}[5m]`,
			wantErr:  "time ranges are not supported",
		},
		{
			name:     "not logql",
			selector: "audit logs please",
			wantErr:  "not a valid LogQL expression",
		},
		{
			name:     "empty",
			selector: "",
			wantErr:  "not a valid LogQL expression",
		},

		// Stream selectors that name no stream at all. Loki's own validation rejects
		// these: it requires one matcher that cannot match empty. `{job=~".+"}` clears
		// that bar, so how much a selector narrows beyond it is the customer's call.
		{
			name:     "empty stream selector",
			selector: `{}`,
			wantErr:  "not a valid LogQL expression",
		},
		{
			name:     "match-everything regex",
			selector: `{job=~".*"}`,
			wantErr:  "not a valid LogQL expression",
		},
		{
			// A negative matcher matches streams missing the label entirely, so Loki
			// counts it as a filter rather than a matcher.
			name:     "negative matcher only",
			selector: `{job!="x"}`,
			wantErr:  "not a valid LogQL expression",
		},
		{
			name:     "reserved label in the stream selector",
			selector: `{__name__="x", job="y"}`,
			wantErr:  `the label "__name__" is reserved`,
		},

		// Parsers the renderer cannot translate.
		{
			name:     "logfmt parser",
			selector: `{scrape_job="audit-logs"} | logfmt | verb="delete"`,
			wantErr:  `the stage "| logfmt" is not supported`,
		},
		{
			name:     "unpack parser",
			selector: `{scrape_job="audit-logs"} | unpack`,
			wantErr:  `the parser "| unpack" is not supported`,
		},
		{
			name:     "pattern parser",
			selector: `{scrape_job="audit-logs"} | pattern "<_> <msg>"`,
			wantErr:  "is not supported",
		},
		{
			name:     "regexp parser",
			selector: `{scrape_job="audit-logs"} | regexp "(?P<verb>\\w+)"`,
			wantErr:  "is not supported",
		},
		{
			name:     "json with parser parameters",
			selector: `{scrape_job="audit-logs"} | json verb="fields.verb"`,
			wantErr:  "is not supported",
		},

		// Stages the renderer cannot translate.
		{
			name:     "line format",
			selector: `{scrape_job="audit-logs"} | line_format "{{ .verb }}"`,
			wantErr:  "is not supported",
		},
		{
			name:     "label format",
			selector: `{scrape_job="audit-logs"} | json | label_format v=verb`,
			wantErr:  "is not supported",
		},
		{
			name:     "drop labels",
			selector: `{scrape_job="audit-logs"} | json | drop verb`,
			wantErr:  "is not supported",
		},
		{
			name:     "keep labels",
			selector: `{scrape_job="audit-logs"} | json | keep verb`,
			wantErr:  "is not supported",
		},
		{
			name:     "decolorize",
			selector: `{scrape_job="audit-logs"} | decolorize`,
			wantErr:  `the stage "| decolorize" is not supported`,
		},

		// Label filters with no negated stream-selector spelling.
		{
			name:     "duration label filter",
			selector: `{scrape_job="audit-logs"} | json | duration > 10s`,
			wantErr:  "is not supported",
		},
		{
			name:     "numeric label filter",
			selector: `{scrape_job="audit-logs"} | json | status_code >= 400`,
			wantErr:  "is not supported",
		},
		{
			name:     "bytes label filter",
			selector: `{scrape_job="audit-logs"} | json | size > 1KB`,
			wantErr:  "is not supported",
		},
		{
			name:     "ip label filter",
			selector: `{scrape_job="audit-logs"} | json | remote_addr = ip("1.2.3.4")`,
			wantErr:  "is not supported",
		},
		{
			name:     "or label filter",
			selector: `{scrape_job="audit-logs"} | json | verb="delete" or verb="create"`,
			wantErr:  "is not supported",
		},
		{
			name:     "and label filter",
			selector: `{scrape_job="audit-logs"} | json | verb="delete" and user="x"`,
			wantErr:  "is not supported",
		},
		{
			name:     "reserved label in a label filter",
			selector: `{scrape_job="audit-logs"} | json | __error__=""`,
			wantErr:  `the label "__error__" is reserved`,
		},

		// Parses, but cannot be built into a pipeline.
		{
			name:     "uncompilable line filter regex",
			selector: `{scrape_job="audit-logs"} |~ "("`,
			wantErr:  "not a usable log selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSelector(tt.selector)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("ValidateSelector(%q) failed: %v", tt.selector, err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("ValidateSelector(%q) succeeded, expected an error about %q", tt.selector, tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("ValidateSelector(%q) error does not mention %q: %v", tt.selector, tt.wantErr, err)
			}
			// The message is the only thing the customer sees, so it has to name the
			// subset rather than just echo the parser.
			if err != nil && !strings.Contains(err.Error(), SupportedSelectorSubset) {
				t.Errorf("ValidateSelector(%q) error does not describe the supported subset: %v", tt.selector, err)
			}
		})
	}
}
