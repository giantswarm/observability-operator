// Package logexporter renders the configuration for alloy-logexporter, the
// installation-wide app that archives selected logs to destinations outside the
// observability platform.
//
// Rendering is pure: LogExport resources and already-resolved credentials in, Helm values
// out. The controller owns reading credentials and persisting the result, the same split
// the collectors use.
package logexporter
