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
	"errors"
	"strings"
	"testing"
)

// Fragments asserted by more than one case. Named only because they repeat; the
// one-off fragments below stay inline so each case still reads on its own.
const (
	errInvalidLogQL = "not a valid LogQL expression"
	errUnsupported  = "is not supported"
)

func TestValidateSelector(t *testing.T) {
	tests := []struct {
		name string
		// wantErr is the fragment the message must contain; empty means the selector is
		// accepted. Asserting the fragment rather than "some error" is the point: every
		// rejection has to explain its own reason.
		selector string
		wantErr  string
		// wantCode is the code the error must carry. Codes get quoted in tickets and
		// documented one by one, so pin them: renumbering one is a breaking change.
		wantCode error
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
			name:     "or line filter",
			selector: `{scrape_job="audit-logs"} |= "delete" or "create"`,
			wantErr:  "uses `or`",
			wantCode: ErrCodeLineFilterOr,
		},
		{
			name:     "ip line filter",
			selector: `{scrape_job="audit-logs"} |= ip("1.2.3.4")`,
			wantErr:  "uses ip()",
			wantCode: ErrCodeLineFilterOp,
		},
		{
			// Accepted: on a negative operator Loki flattens `or` into an AND-chain of
			// nots, so each term negates on its own. Only the positive operators make it a
			// real alternation.
			name:     "or on a negative line filter",
			selector: `{scrape_job="audit-logs"} != "delete" or "create"`,
		},
		{
			name:     "or later in a chain",
			selector: `{scrape_job="audit-logs"} |= "a" |= "b" or "c"`,
			wantErr:  "uses `or`",
			wantCode: ErrCodeLineFilterOr,
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
			// The four string comparisons pin StringMatcher's type switch: Loki picks a
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
			wantCode: ErrCodeAggregation,
		},
		{
			name:     "range aggregation",
			selector: `count_over_time({scrape_job="audit-logs"}[1h])`,
			wantErr:  "aggregations are not supported",
			wantCode: ErrCodeAggregation,
		},
		{
			name:     "scalar literal",
			selector: `1`,
			wantErr:  "returns a value rather than log lines",
			wantCode: ErrCodeNotLogLines,
		},
		{
			name:     "vector",
			selector: `vector(0)`,
			wantErr:  "returns a value rather than log lines",
			wantCode: ErrCodeNotLogLines,
		},
		{
			name:     "time range",
			selector: `{scrape_job="audit-logs"}[5m]`,
			wantErr:  "time ranges are not supported",
			wantCode: ErrCodeTimeRange,
		},
		{
			name:     "not logql",
			selector: "audit logs please",
			wantErr:  errInvalidLogQL,
			wantCode: ErrCodeSyntax,
		},
		{
			name:     "empty",
			selector: "",
			wantErr:  errInvalidLogQL,
			wantCode: ErrCodeSyntax,
		},

		// Stream selectors that name no stream at all. Loki's own validation rejects
		// these: it requires one matcher that cannot match empty. `{job=~".+"}` clears
		// that bar, so how much a selector narrows beyond it is the customer's call.
		{
			name:     "empty stream selector",
			selector: `{}`,
			wantErr:  errInvalidLogQL,
			wantCode: ErrCodeSyntax,
		},
		{
			name:     "match-everything regex",
			selector: `{job=~".*"}`,
			wantErr:  errInvalidLogQL,
			wantCode: ErrCodeSyntax,
		},
		{
			// A negative matcher matches streams missing the label entirely, so Loki
			// counts it as a filter rather than a matcher.
			name:     "negative matcher only",
			selector: `{job!="x"}`,
			wantErr:  errInvalidLogQL,
			wantCode: ErrCodeSyntax,
		},
		{
			name:     "reserved label in the stream selector",
			selector: `{__name__="x", job="y"}`,
			wantErr:  `the label "__name__" is reserved`,
			wantCode: ErrCodeReservedLabel,
		},

		// Parsers the renderer cannot translate.
		{
			name:     "logfmt parser",
			selector: `{scrape_job="audit-logs"} | logfmt | verb="delete"`,
			wantErr:  `the stage "| logfmt" is not supported`,
			wantCode: ErrCodeStage,
		},
		{
			name:     "unpack parser",
			selector: `{scrape_job="audit-logs"} | unpack`,
			wantErr:  `the parser "| unpack" is not supported`,
			wantCode: ErrCodeParser,
		},
		{
			name:     "pattern parser",
			selector: `{scrape_job="audit-logs"} | pattern "<_> <msg>"`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeParser,
		},
		{
			name:     "regexp parser",
			selector: `{scrape_job="audit-logs"} | regexp "(?P<verb>\\w+)"`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeParser,
		},
		{
			name:     "json with parser parameters",
			selector: `{scrape_job="audit-logs"} | json verb="fields.verb"`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeStage,
		},

		// Stages the renderer cannot translate.
		{
			name:     "line format",
			selector: `{scrape_job="audit-logs"} | line_format "{{ .verb }}"`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeStage,
		},
		{
			name:     "label format",
			selector: `{scrape_job="audit-logs"} | json | label_format v=verb`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeStage,
		},
		{
			name:     "drop labels",
			selector: `{scrape_job="audit-logs"} | json | drop verb`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeStage,
		},
		{
			name:     "keep labels",
			selector: `{scrape_job="audit-logs"} | json | keep verb`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeStage,
		},
		{
			name:     "decolorize",
			selector: `{scrape_job="audit-logs"} | decolorize`,
			wantErr:  `the stage "| decolorize" is not supported`,
			wantCode: ErrCodeStage,
		},

		// Label filters with no negated stream-selector spelling.
		{
			name:     "duration label filter",
			selector: `{scrape_job="audit-logs"} | json | duration > 10s`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeLabelFilter,
		},
		{
			name:     "numeric label filter",
			selector: `{scrape_job="audit-logs"} | json | status_code >= 400`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeLabelFilter,
		},
		{
			name:     "bytes label filter",
			selector: `{scrape_job="audit-logs"} | json | size > 1KB`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeLabelFilter,
		},
		{
			name:     "ip label filter",
			selector: `{scrape_job="audit-logs"} | json | remote_addr = ip("1.2.3.4")`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeLabelFilter,
		},
		{
			name:     "or label filter",
			selector: `{scrape_job="audit-logs"} | json | verb="delete" or verb="create"`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeLabelFilter,
		},
		{
			name:     "and label filter",
			selector: `{scrape_job="audit-logs"} | json | verb="delete" and user="x"`,
			wantErr:  errUnsupported,
			wantCode: ErrCodeLabelFilter,
		},
		{
			name:     "reserved label in a label filter",
			selector: `{scrape_job="audit-logs"} | json | __error__=""`,
			wantErr:  `the label "__error__" is reserved`,
			wantCode: ErrCodeReservedLabel,
		},

		// Parses, but cannot be built into a pipeline.
		{
			name:     "uncompilable line filter regex",
			selector: `{scrape_job="audit-logs"} |~ "("`,
			wantErr:  "not a usable log selector",
			wantCode: ErrCodePipelineBuild,
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
			if tt.wantCode != nil && !errors.Is(err, tt.wantCode) {
				t.Errorf("ValidateSelector(%q) error is not %v: %v", tt.selector, tt.wantCode, err)
			}
		})
	}
}

// TestCodesAreDistinct guards against every sentinel collapsing to one value. The table
// above would still pass in that case, because each of its rejections would match
// whatever code it was asserted against.
func TestCodesAreDistinct(t *testing.T) {
	// LOGQL005 is unreachable through ValidateSelector, so it is not listed.
	codes := []error{
		ErrCodeTimeRange, ErrCodeSyntax, ErrCodeNotLogLines, ErrCodeAggregation,
		ErrCodeNotLogSelector, ErrCodePipelineBuild, ErrCodeParser, ErrCodeLabelFilter,
		ErrCodeStage, ErrCodeLineFilterOr, ErrCodeLineFilterOp, ErrCodeReservedLabel,
	}
	for i, a := range codes {
		for j, b := range codes {
			if i != j && errors.Is(a, b) {
				t.Errorf("%v and %v are the same error value", a, b)
			}
		}
	}
}

// TestUnsupportedMessageShape pins the rendered message, which errors.Is cannot: the code
// has to lead, because docs/logexport-selectors.md tells customers to read it off the
// front of the admission error.
func TestUnsupportedMessageShape(t *testing.T) {
	err := ValidateSelector(`{scrape_job="audit-logs"}[5m]`)
	if err == nil {
		t.Fatal("ValidateSelector() accepted a time range")
	}
	if !strings.HasPrefix(err.Error(), ErrCodeTimeRange.Error()+": ") {
		t.Errorf("message does not lead with the code: %v", err)
	}
	if !strings.Contains(err.Error(), SupportedSelectorSubset) {
		t.Errorf("message does not describe the supported subset: %v", err)
	}
}
