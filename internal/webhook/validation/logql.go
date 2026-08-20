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
	"fmt"
	"strings"

	"github.com/grafana/loki/v3/pkg/logql/log"
	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/prometheus/prometheus/model/labels"
)

// SupportedSelectorSubset describes, for humans, what ValidateSelector accepts.
const SupportedSelectorSubset = `a stream selector ` +
	`(e.g. {scrape_job="audit-logs"}), optional line filters (|=, !=, |~, !~), ` +
	`an optional "| json", and optional label filters (e.g. | verb="delete")`

// ValidateSelector reports whether a LogExport selector is a LogQL expression the
// export pipeline can honour.
//
// The CRD already validates: minLength, maxLength. This adds: the expression is
// syntactically valid LogQL, and is restricted to the subset the exporter renders.
//
// The accepted subset is deliberately narrower than "any log selector", because it is
// bounded by what the config rendering can translate into Alloy stages:
//
//   - stream selector + line filters -> stage.match { selector = ... }, verbatim
//   - "| json" + label filters       -> stage.json plus a drop of the negated filter
//
// The pipeline selects by dropping the *negated* selector, so an expression accepted
// here but translated imperfectly over-exports rather than under-exports — customer
// logs leaving the installation, not missing data. Anything accepted here therefore has
// to be renderable; widen this only together with the renderer.
//
// Rendering must be driven from the parsed expression (syntax.Expr.String()), never from
// the raw field value: the parser tolerates trailing "# comment" and non-canonical
// whitespace, so a negated term appended to the raw string can be commented out.
func ValidateSelector(selector string) error {
	_, err := ParseSelector(selector)
	return err
}

// ParseSelector validates a selector and returns the parsed expression, so that config
// rendering is driven from the same parse admission accepts and the two cannot drift.
func ParseSelector(selector string) (syntax.LogSelectorExpr, error) {
	// ParseExpr both parses and validates. Validation is what rejects a selector that
	// names no stream -- `{}`, `{job=~".*"}`, `{job!="x"}` -- since Loki requires at least
	// one matcher that cannot match empty. The parse phase recovers its own panics, so
	// untrusted input is safe to pass; validation only walks the AST the parse returned.
	expr, err := syntax.ParseExpr(selector)
	if err != nil {
		// LogQL allows a range only inside an aggregation, so a bare `{x="y"}[5m]` is a
		// syntax error with no AST to match on: it parses once wrapped in one. Message
		// only — the expression is rejected either way.
		// ParseExpr here too, not ParseExprWithoutValidation: `{}` fails the parse above
		// but `count_over_time({})` would still parse, so an unvalidated probe would
		// blame a time range that the selector does not contain.
		if _, rangeErr := syntax.ParseExpr(fmt.Sprintf("count_over_time(%s)", selector)); rangeErr == nil {
			return nil, unsupported("time ranges are not supported: an export forwards log lines as they arrive, so there is no past window to select from")
		}
		return nil, unsupported("not a valid LogQL expression: %v", err)
	}

	switch e := expr.(type) {
	case *syntax.MatchersExpr:
		if err := checkMatchers(e.Mts); err != nil {
			return nil, err
		}
	case *syntax.PipelineExpr:
		if err := checkMatchers(e.Left.Mts); err != nil {
			return nil, err
		}
		if err := checkStages(e.MultiStages); err != nil {
			return nil, err
		}
	case *syntax.LiteralExpr, *syntax.VectorExpr:
		// These satisfy LogSelectorExpr as well as SampleExpr, so they have to be caught
		// before the case below.
		return nil, unsupported("%q returns a value rather than log lines", spell(e))
	case syntax.SampleExpr:
		return nil, unsupported("aggregations are not supported: an export is a continuous tee, not a query")
	default:
		return nil, unsupported("not a log selector")
	}

	logSelector := expr.(syntax.LogSelectorExpr)

	// Building the pipeline compiles the line filter regexps, which parsing alone does
	// not. An expression that parses but fails to build would reach Alloy and stop every
	// export on the installation.
	if _, err := logSelector.Pipeline(); err != nil {
		return nil, unsupported("not a usable log selector: %v", err)
	}
	return logSelector, nil
}

// checkMatchers rejects Loki-internal labels in the stream selector. How much the
// selector narrows is left to Loki's own validation, which ParseExpr applies.
func checkMatchers(matchers []*labels.Matcher) error {
	for _, m := range matchers {
		if err := checkLabelName(m.Name); err != nil {
			return err
		}
	}
	return nil
}

// checkStages allows only the stages the renderer can translate.
func checkStages(stages syntax.MultiStageExpr) error {
	for _, stage := range stages {
		switch s := stage.(type) {
		case *syntax.LineFilterExpr:
			// Passed verbatim into stage.match, except that selection is rendered as a drop
			// of the *negated* filter. On a positive operator `or` is a real alternation,
			// which negates only by De Morgan across the chain; ip() has no negated spelling
			// at all. Neither becomes a drop term by term, and a mistranslation over-exports.
			// `or` on a negative operator is fine: Loki flattens it into an AND-chain.
			if err := checkLineFilters(s); err != nil {
				return err
			}
		case *syntax.LineParserExpr:
			// Also where regexp, unpack and pattern land.
			if s.Op != syntax.OpParserTypeJSON || s.Param != "" {
				return unsupported("the parser %q is not supported: only \"| json\" is", spell(s))
			}
		case *syntax.LabelFilterExpr:
			// Selection is rendered as a drop of the *negated* filter, and a drop can only
			// spell a string comparison. `| duration > 10s` has no negated stream-selector
			// form, so there is nothing to render it into.
			m, ok := StringMatcher(s.LabelFilterer)
			if !ok {
				return unsupported("the label filter %q is not supported: only string comparisons (=, !=, =~, !~) on a single label are", spell(s))
			}
			if err := checkLabelName(m.Name); err != nil {
				return err
			}
		default:
			return unsupported("the stage %q is not supported", spell(s))
		}
	}
	return nil
}

// checkLineFilters walks a line filter chain, rejecting the forms that have no negated
// spelling. Left is the previous filter in the chain; Or is set only where the parser kept
// an alternation, which it does for `|=` and `|~` but not for `!=` and `!~`.
func checkLineFilters(expr *syntax.LineFilterExpr) error {
	for e := expr; e != nil; e = e.Left {
		if e.Or != nil || e.IsOrChild {
			return unsupported("the line filter %q uses `or`, which cannot be negated term by term", spell(expr))
		}
		if e.Op != "" {
			return unsupported("the line filter %q uses %s(), which has no negated spelling", spell(expr), e.Op)
		}
	}
	return nil
}

// StringMatcher reports the matcher behind a label filter, for the filterers that are a
// single string comparison.
//
// Note that `| verb="delete"` is not a StringLabelFilter: log.NewStringLabelFilter
// returns a LineFilterLabelFilter in the common case, a NoopLabelFilter when the matcher
// reduces to a match-all, and a StringLabelFilter only on its fallback path. All three
// carry a matcher; binary (and/or), numeric, duration, bytes and IP filters do not, and
// have no stream-selector spelling once negated.
func StringMatcher(f log.LabelFilterer) (*labels.Matcher, bool) {
	switch t := f.(type) {
	case *log.LineFilterLabelFilter:
		return t.Matcher, true
	case *log.StringLabelFilter:
		return t.Matcher, true
	case *log.NoopLabelFilter:
		return t.Matcher, true
	}
	return nil, false
}

// checkLabelName rejects Loki's internal labels. `__error__` and friends are produced by
// Loki's query-time parsers and do not exist in the exporter's pipeline, so filtering on
// them would silently export more than the customer asked for.
func checkLabelName(name string) error {
	if strings.HasPrefix(name, "__") {
		return unsupported("the label %q is reserved for Loki internals and is not visible to the exporter", name)
	}
	return nil
}

// spell renders a fragment the way the user wrote it, so the error names the offender.
func spell(e syntax.Expr) string {
	return strings.TrimSpace(e.String())
}

func unsupported(format string, args ...any) error {
	return fmt.Errorf("%s; supported is %s", fmt.Sprintf(format, args...), SupportedSelectorSubset)
}
