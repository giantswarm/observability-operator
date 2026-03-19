// Package ruler provides clients for deleting alerting and recording rules
// from ruler backends (Mimir and Loki) on cluster deletion.
//
// Tenants in this codebase map to Grafana organizations (e.g. "giantswarm").
// Use [NewMulti] to compose multiple backends; it implements [Client] so
// callers depend only on the interface.
package ruler
