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

	"github.com/grafana/loki/v3/pkg/logql/syntax"
)

// SupportedSelectorSubset describes, for humans, what ValidateSelector accepts.
const SupportedSelectorSubset = "a stream selector, optional line filters, and an optional parse-and-filter clause"

// ValidateSelector reports whether a LogExport selector is a LogQL expression the
// export pipeline can honour.
//
// The CRD already validates: minLength, maxLength. This adds: the expression is
// syntactically valid LogQL, and is a log selector rather than a metric query.
//
// Anything that renders the export config must be driven from this same function.
// The pipeline selects by dropping the *negated* selector, so an expression accepted
// here but translated imperfectly over-exports rather than under-exports — customer
// logs leaving the installation, not missing data.
func ValidateSelector(selector string) error {
	// ParseLogSelector rejects aggregations and time ranges for us: they parse to a
	// SampleExpr, and only a LogSelectorExpr is returned. It recovers parser panics
	// internally, so untrusted input is safe to pass.
	if _, err := syntax.ParseLogSelector(selector, true); err != nil {
		return fmt.Errorf("must be %s. Aggregations and time ranges are not supported, because an export is a continuous tee rather than a query: %w",
			SupportedSelectorSubset, err)
	}
	return nil
}
