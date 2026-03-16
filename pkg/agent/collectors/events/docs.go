// Package events provides the events collector implementation for observability.
// It manages the configuration of Alloy agents that handle four signal pipelines:
//   - Kubernetes events → Loki (native log ingestion)
//   - OTLP traces → Tempo (distributed tracing)
//   - OTLP metrics → Mimir (metrics ingestion via OTLP HTTP)
//   - OTLP logs → Loki (log ingestion via OTLP HTTP)
//
// Management clusters use in-cluster service URLs without authentication.
// Workload clusters use external URLs and credentials stored in a per-cluster Secret.
package events
