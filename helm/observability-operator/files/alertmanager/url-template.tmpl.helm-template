{{`
{{ define "__alerturl" }}
`}}{{ .GrafanaAddress }}{{`/alerting/Mimir/{{ .CommonLabels.alertname }}/find
{{ end }}

{{ define "__dashboardurl" -}}
{{ if match "^https://.+" (index .Alerts 0).Annotations.dashboard }}{{ (index .Alerts 0).Annotations.dashboard }}
{{ else }}
`}}{{ .GrafanaAddress }}{{`/d/{{ (index .Alerts 0).Annotations.dashboard }}
{{ end }}
{{- end }}

{{ define "__queryurl" }}
`}}{{ .GrafanaAddress }}{{`/alerting/Mimir/{{ .CommonLabels.alertname }}/find
{{ end }}

{{ define "__runbookurl" -}}https://intranet.giantswarm.io/docs/support-and-ops/ops-recipes/{{ (index .Alerts 0).Annotations.opsrecipe }}{{- end }}
`}}
