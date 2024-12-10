{{/* vim: set filetype=mustache: */}}

{{- define "alertmanager-secret.name" -}}
{{- include "resource.default.name" . -}}-alertmanager
{{- end }}
