// Package alertmanager provides a client for configuring and managing
// Alertmanager tenants on a Mimir instance.
//
// It implements the [Service] interface which exposes two operations:
//   - [Service.ConfigureFromSecret]: validate and push an Alertmanager
//     configuration stored in a Kubernetes Secret to Mimir.
//   - [Service.DeleteForTenant]: remove the Alertmanager configuration
//     for a given tenant from Mimir (idempotent).
//
// The [Validate] function provides lightweight config+template validation
// for use in admission webhooks without making any HTTP calls.
package alertmanager
