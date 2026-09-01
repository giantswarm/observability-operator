package logexporter

import (
	"fmt"
	"strings"

	"github.com/grafana/loki/v3/pkg/logql/log"
	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/giantswarm/observability-operator/internal/webhook/validation"
)

// pipeline is a customer selector translated into ordered loki.process stages.
//
// Selection is a DROP of the negated selector, never a keep: stage.match with the
// default action = "keep" does not drop non-matching entries, it only gates its nested
// stages, so a keep-based translation exports every log line in the installation.
// Sequential drops compose as an AND of keeps.
type pipeline struct {
	// StreamDrops are stage.match selectors dropping everything the customer did not
	// select, one per negated term.
	StreamDrops []string
	// JSONFields are the log-line fields a label filter needs, extracted and promoted
	// to labels before they can be matched on.
	JSONFields []string
	// FieldDrops are stage.match selectors over those promoted labels, negated.
	FieldDrops []string
}

// translate turns a validated selector into the stages that implement it.
func translate(selector string) (pipeline, error) {
	expr, err := validation.ParseSelector(selector)
	if err != nil {
		return pipeline{}, err
	}

	var matchers []*labels.Matcher
	var stages syntax.MultiStageExpr
	switch e := expr.(type) {
	case *syntax.MatchersExpr:
		matchers = e.Mts
	case *syntax.PipelineExpr:
		matchers, stages = e.Left.Mts, e.MultiStages
	default:
		// ParseSelector returns only these two.
		return pipeline{}, fmt.Errorf("selector %q is not renderable", selector)
	}

	var p pipeline
	for _, m := range matchers {
		negated, err := negateMatcher(m)
		if err != nil {
			return pipeline{}, err
		}
		p.StreamDrops = append(p.StreamDrops, "{"+negated.String()+"}")
	}

	// The stream selector prefixes every line-filter drop: a line filter is not a valid
	// selector on its own.
	streamSelector := "{" + matchersString(matchers) + "}"

	// Label filters before any parser act on stream labels, which are already there, so
	// they need no extraction. After a parser they act on fields of the line.
	parsed := false
	for _, stage := range stages {
		switch s := stage.(type) {
		case *syntax.LineFilterExpr:
			terms, err := lineFilterTerms(s)
			if err != nil {
				return pipeline{}, err
			}
			for _, t := range terms {
				p.StreamDrops = append(p.StreamDrops, streamSelector+" "+t)
			}
		case *syntax.LineParserExpr:
			// ParseSelector accepts only "| json" here.
			parsed = true
		case *syntax.LabelFilterExpr:
			m, ok := validation.StringMatcher(s.LabelFilterer)
			if !ok {
				// Validation rejects these, so this is a guard rather than a path.
				return pipeline{}, fmt.Errorf("label filter %q is not renderable", strings.TrimSpace(s.String()))
			}
			negated, err := negateMatcher(m)
			if err != nil {
				return pipeline{}, err
			}
			if parsed {
				p.JSONFields = appendUnique(p.JSONFields, m.Name)
				p.FieldDrops = append(p.FieldDrops, "{"+negated.String()+"}")
			} else {
				p.StreamDrops = append(p.StreamDrops, "{"+negated.String()+"}")
			}
		default:
			return pipeline{}, fmt.Errorf("stage %q is accepted by validation but not renderable", strings.TrimSpace(stage.String()))
		}
	}

	return p, nil
}

// negateMatcher flips a matcher so that it selects what must be dropped.
func negateMatcher(m *labels.Matcher) (*labels.Matcher, error) {
	var negated labels.MatchType
	switch m.Type {
	case labels.MatchEqual:
		negated = labels.MatchNotEqual
	case labels.MatchNotEqual:
		negated = labels.MatchEqual
	case labels.MatchRegexp:
		negated = labels.MatchNotRegexp
	case labels.MatchNotRegexp:
		negated = labels.MatchRegexp
	default:
		return nil, fmt.Errorf("matcher %q cannot be negated", m.String())
	}

	out, err := labels.NewMatcher(negated, m.Name, m.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to negate matcher %q: %w", m.String(), err)
	}
	return out, nil
}

// lineFilterTerms returns the opposite of each line filter in a chain.
//
// `or` chains and ip() filters are not rewritten here. Validation rejects them, so the
// errors below are unreachable through ParseSelector and exist to keep this honest if that
// ever loosens -- a mistranslation here exports the wrong lines.
func lineFilterTerms(expr *syntax.LineFilterExpr) ([]string, error) {
	var terms []string
	for e := expr; e != nil; e = e.Left {
		if e.Or != nil || e.IsOrChild {
			return nil, fmt.Errorf("line filter %q uses `or`, which is not rewritable", strings.TrimSpace(expr.String()))
		}
		if e.Op != "" {
			return nil, fmt.Errorf("line filter %q uses ip(), which is not supported", strings.TrimSpace(expr.String()))
		}

		negated, err := negateLineMatchType(e.Ty)
		if err != nil {
			return nil, err
		}
		// Left is the earlier filter, so prepend to keep the customer's order.
		terms = append([]string{fmt.Sprintf("%s %q", negated, e.Match)}, terms...)
	}
	return terms, nil
}

func negateLineMatchType(t log.LineMatchType) (string, error) {
	switch t {
	case log.LineMatchEqual:
		return "!=", nil
	case log.LineMatchNotEqual:
		return "|=", nil
	case log.LineMatchRegexp:
		return "!~", nil
	case log.LineMatchNotRegexp:
		return "|~", nil
	}
	return "", fmt.Errorf("line filter type %v cannot be negated", t)
}

func matchersString(matchers []*labels.Matcher) string {
	terms := make([]string, 0, len(matchers))
	for _, m := range matchers {
		terms = append(terms, m.String())
	}
	return strings.Join(terms, ", ")
}

func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}
