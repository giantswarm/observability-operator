package logexporter

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/loki/v3/pkg/logql/syntax"

	"github.com/giantswarm/observability-operator/internal/webhook/validation"
)

// Fragments asserted by more than one case. Named only because they repeat; the one-off
// fragments below stay inline so each case still reads on its own.
const (
	dropAuditJob = `{scrape_job!="audit-logs"}`
	fieldVerb    = "verb"
)

// TestTranslate pins the stages a selector becomes. Selection is a drop of the negated
// selector, so these strings are the whole of what decides which lines leave the
// installation: a wrong negation over-exports rather than failing.
func TestTranslate(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		want     pipeline
	}{
		{
			name:     "single matcher",
			selector: `{scrape_job="audit-logs"}`,
			want:     pipeline{StreamDrops: []string{dropAuditJob}},
		},
		{
			name:     "several matchers drop independently",
			selector: `{cluster_id="wc01", scrape_job="audit-logs"}`,
			want: pipeline{StreamDrops: []string{
				`{cluster_id!="wc01"}`,
				dropAuditJob,
			}},
		},
		{
			name:     "regex matcher",
			selector: `{scrape_job=~"audit.*"}`,
			want:     pipeline{StreamDrops: []string{`{scrape_job!~"audit.*"}`}},
		},
		{
			name:     "negative matchers flip back",
			selector: `{scrape_job="audit-logs", cluster_id!="wc01", namespace!~"kube-.*"}`,
			want: pipeline{StreamDrops: []string{
				dropAuditJob,
				`{cluster_id="wc01"}`,
				`{namespace=~"kube-.*"}`,
			}},
		},
		{
			// Newly reachable: validation no longer requires an exact match, it only
			// requires a matcher that cannot match empty.
			name:     "match-everything-with-a-label",
			selector: `{scrape_job=~".+"}`,
			want:     pipeline{StreamDrops: []string{`{scrape_job!~".+"}`}},
		},
		{
			// A line filter is not a selector on its own, so the drop carries the stream
			// selector too. It is redundant after the matcher drops above, and harmless.
			name:     "line filter",
			selector: `{scrape_job="audit-logs"} |= "delete"`,
			want: pipeline{StreamDrops: []string{
				dropAuditJob,
				`{scrape_job="audit-logs"} != "delete"`,
			}},
		},
		{
			name:     "all four line filter operators",
			selector: `{scrape_job="audit-logs"} |= "a" != "b" |~ "c.*" !~ "d.*"`,
			want: pipeline{StreamDrops: []string{
				dropAuditJob,
				`{scrape_job="audit-logs"} != "a"`,
				`{scrape_job="audit-logs"} |= "b"`,
				`{scrape_job="audit-logs"} !~ "c.*"`,
				`{scrape_job="audit-logs"} |~ "d.*"`,
			}},
		},
		{
			name:     "line filters keep the customer's order",
			selector: `{scrape_job="audit-logs"} |= "first" |= "second"`,
			want: pipeline{StreamDrops: []string{
				dropAuditJob,
				`{scrape_job="audit-logs"} != "first"`,
				`{scrape_job="audit-logs"} != "second"`,
			}},
		},
		{
			// `or` on a negative operator is flattened by the parser into an AND-chain of
			// nots, so it negates term by term: keep lines with neither == drop lines with
			// either. On a positive operator it stays an alternation and is rejected.
			name:     "or on a negative line filter",
			selector: `{scrape_job="audit-logs"} != "delete" or "create"`,
			want: pipeline{StreamDrops: []string{
				dropAuditJob,
				`{scrape_job="audit-logs"} |= "delete"`,
				`{scrape_job="audit-logs"} |= "create"`,
			}},
		},
		{
			name:     "parse and filter",
			selector: `{scrape_job="audit-logs"} | json | verb="delete"`,
			want: pipeline{
				StreamDrops: []string{dropAuditJob},
				JSONFields:  []string{fieldVerb},
				FieldDrops:  []string{`{verb!="delete"}`},
			},
		},
		{
			name:     "chained label filters",
			selector: `{scrape_job="audit-logs"} | json | verb="delete" | user=~"admin.*"`,
			want: pipeline{
				StreamDrops: []string{dropAuditJob},
				JSONFields:  []string{fieldVerb, "user"},
				FieldDrops:  []string{`{verb!="delete"}`, `{user!~"admin.*"}`},
			},
		},
		{
			name:     "a field filtered twice is extracted once",
			selector: `{scrape_job="audit-logs"} | json | verb!="get" | verb!="list"`,
			want: pipeline{
				StreamDrops: []string{dropAuditJob},
				JSONFields:  []string{fieldVerb},
				FieldDrops:  []string{`{verb="get"}`, `{verb="list"}`},
			},
		},
		{
			// Before a parser a label filter acts on stream labels, which are already
			// there, so it needs no extraction and drops with the rest.
			name:     "label filter without a parser",
			selector: `{scrape_job="audit-logs"} | cluster_id="wc01"`,
			want: pipeline{StreamDrops: []string{
				dropAuditJob,
				`{cluster_id!="wc01"}`,
			}},
		},
		{
			name:     "json parser with no filter renders no stages",
			selector: `{scrape_job="audit-logs"} | json`,
			want:     pipeline{StreamDrops: []string{dropAuditJob}},
		},
		{
			name:     "teleport",
			selector: `{scrape_job="teleport.giantswarm.io"}`,
			want:     pipeline{StreamDrops: []string{`{scrape_job!="teleport.giantswarm.io"}`}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translate(tt.selector)
			if err != nil {
				t.Fatalf("translate(%q) failed: %v", tt.selector, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("translate(%q) mismatch (-want +got):\n%s", tt.selector, diff)
			}
		})
	}
}

func TestTranslateRejects(t *testing.T) {
	// Every rejection here comes out of validation.ParseSelector, not from this package.
	tests := []struct {
		name     string
		selector string
		wantErr  error
	}{
		{
			name:     "aggregation",
			selector: `sum by (verb) (rate({scrape_job="audit-logs"}[5m]))`,
			wantErr:  validation.ErrCodeAggregation,
		},
		{
			name:     "unsupported parser",
			selector: `{scrape_job="audit-logs"} | logfmt`,
			wantErr:  validation.ErrCodeStage,
		},
		{
			name:     "unsupported stage",
			selector: `{scrape_job="audit-logs"} | line_format "{{.verb}}"`,
			wantErr:  validation.ErrCodeStage,
		},
		{
			name:     "numeric label filter",
			selector: `{scrape_job="audit-logs"} | json | duration > 10s`,
			wantErr:  validation.ErrCodeLabelFilter,
		},
		{
			name:     "or line filter",
			selector: `{scrape_job="audit-logs"} |= "a" or "b"`,
			wantErr:  validation.ErrCodeLineFilterOr,
		},
		{
			name:     "ip line filter",
			selector: `{scrape_job="audit-logs"} |= ip("1.2.3.4")`,
			wantErr:  validation.ErrCodeLineFilterOp,
		},
		{
			name:     "match everything",
			selector: `{}`,
			wantErr:  validation.ErrCodeSyntax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := translate(tt.selector)
			if err == nil {
				t.Fatalf("translate(%q) succeeded, expected %v", tt.selector, tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("translate(%q) error is not %v: %v", tt.selector, tt.wantErr, err)
			}
		})
	}
}

// TestLineFilterTermsGuards exercises the refusals directly. Validation rejects `or` and
// ip() before translate() ever sees them, so these are unreachable in production and this
// is the only thing keeping them honest if that changes.
func TestLineFilterTermsGuards(t *testing.T) {
	for _, tt := range []struct {
		selector string
		wantErr  error
	}{
		{`{job="x"} |= "a" or "b"`, errOrNotRewritable},
		{`{job="x"} |= ip("1.2.3.4")`, errIPNotSupported},
	} {
		expr, err := syntax.ParseExpr(tt.selector)
		if err != nil {
			t.Fatalf("ParseExpr(%q) failed: %v", tt.selector, err)
		}
		stages := expr.(*syntax.PipelineExpr).MultiStages
		filter, ok := stages[0].(*syntax.LineFilterExpr)
		if !ok {
			t.Fatalf("%q: first stage is %T, expected a line filter", tt.selector, stages[0])
		}
		if _, err := lineFilterTerms(filter); !errors.Is(err, tt.wantErr) {
			t.Errorf("lineFilterTerms(%q) error is not %v: %v", tt.selector, tt.wantErr, err)
		}
	}
}
