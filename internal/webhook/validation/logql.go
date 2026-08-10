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
const SupportedSelectorSubset = `a stream selector with at least one exact match ` +
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
	// ParseExprWithoutValidation recovers parser panics internally, so untrusted input is
	// safe to pass. Loki's own validation is skipped on purpose: the expression type is
	// needed to say *why* an expression was rejected, and the matcher rule below is
	// stricter than its "at least one equality matcher" check anyway.
	expr, err := syntax.ParseExprWithoutValidation(selector)
	if err != nil {
		// LogQL allows a range only inside an aggregation, so a bare `{x="y"}[5m]` is a
		// syntax error with no AST to match on: it parses once wrapped in one. Message
		// only — the expression is rejected either way.
		if _, rangeErr := syntax.ParseExprWithoutValidation(fmt.Sprintf("count_over_time(%s)", selector)); rangeErr == nil {
			return unsupported("time ranges are not supported: an export starts when the LogExport is created and cannot backfill")
		}
		return unsupported("not a valid LogQL expression: %v", err)
	}

	switch e := expr.(type) {
	case *syntax.MatchersExpr:
		if err := checkMatchers(e.Mts); err != nil {
			return err
		}
	case *syntax.PipelineExpr:
		if err := checkMatchers(e.Left.Mts); err != nil {
			return err
		}
		if err := checkStages(e.MultiStages); err != nil {
			return err
		}
	case *syntax.LiteralExpr, *syntax.VectorExpr:
		// These satisfy LogSelectorExpr as well as SampleExpr, so they have to be caught
		// before the arm below.
		return unsupported("%q returns a value rather than log lines", spell(e))
	case syntax.SampleExpr:
		return unsupported("aggregations are not supported: an export is a continuous tee, not a query")
	default:
		return unsupported("not a log selector")
	}

	// Building the pipeline compiles the line filter regexps, which parsing alone does
	// not. An expression that parses but fails to build would reach Alloy and stop every
	// export on the installation.
	if _, err := expr.(syntax.LogSelectorExpr).Pipeline(); err != nil {
		return unsupported("not a usable log selector: %v", err)
	}
	return nil
}

// checkMatchers requires the stream selector to name what it exports. Loki only demands
// a matcher that cannot match empty, which `{job=~".+"}` satisfies — that would export
// every log line in the installation.
func checkMatchers(matchers []*labels.Matcher) error {
	hasEqual := false
	for _, m := range matchers {
		if err := checkLabelName(m.Name); err != nil {
			return err
		}
		if m.Type == labels.MatchEqual && m.Value != "" {
			hasEqual = true
		}
	}
	if !hasEqual {
		return unsupported(`the stream selector needs at least one exact match such as {scrape_job="audit-logs"}: a selector that matches every stream would export every log line in the installation`)
	}
	return nil
}

// checkStages allows only the stages the renderer can translate.
func checkStages(stages syntax.MultiStageExpr) error {
	for _, stage := range stages {
		switch s := stage.(type) {
		case *syntax.LineFilterExpr:
			// Passed verbatim into stage.match, so anything the parser accepts works.
		case *syntax.LineParserExpr:
			// Also where regexp, unpack and pattern land.
			if s.Op != syntax.OpParserTypeJSON || s.Param != "" {
				return unsupported("the parser %q is not supported: only \"| json\" is", spell(s))
			}
		case *syntax.LabelFilterExpr:
			m, ok := stringMatcher(s.LabelFilterer)
			if !ok {
				return unsupported("the label filter %q is not supported: only string comparisons (=, !=, =~, !~) on a single label are, because a stream selector cannot express anything else once negated", spell(s))
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

// stringMatcher reports the matcher behind a label filter, for the filterers that are a
// single string comparison.
//
// Note that `| verb="delete"` is not a StringLabelFilter: log.NewStringLabelFilter
// returns a LineFilterLabelFilter in the common case, a NoopLabelFilter when the matcher
// reduces to a match-all, and a StringLabelFilter only on its fallback path. All three
// carry a matcher; binary (and/or), numeric, duration, bytes and IP filters do not, and
// have no stream-selector spelling once negated.
func stringMatcher(f log.LabelFilterer) (*labels.Matcher, bool) {
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
