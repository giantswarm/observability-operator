global:
  resolve_timeout: 5m
{{- if .ProxyURL }}
  http_config:
    proxy_url:  {{ .ProxyURL }}
{{- end }}
{{- if .SlackApiToken }}
  slack_api_url: "https://slack.com/api/chat.postMessage"
{{- else }}
  slack_api_url: {{ .SlackApiURL }}
{{- end }}

route:
  group_by: [alertname, cluster_id, installation, status, team]
  group_interval: 15m
  group_wait: 5m
  repeat_interval: 4h
  receiver: root

  routes:
  - receiver: heartbeat
    matchers:
    - alertname="Heartbeat"
    # TODO(@team-atlas): This label is needed for now because we support legacy and new heartbeat alerts. Lets remove it when we remove https://github.com/giantswarm/prometheus-rules/pull/1094/files#diff-2cc8f3328e242b6c86769bfb52b286c7146e074dd244ebb76a0025c838f33d0fR21
    - type="mimir-heartbeat"
    continue: true
    group_wait: 30s
    group_interval: 30s
    repeat_interval: 15m
  {{- if eq .Pipeline "stable-testing" }}
  - receiver: blackhole
    matchers:
    - cluster_type="workload_cluster"
    continue: false
  - receiver: blackhole
    matchers:
    - cluster_id=~"t-.*"
    continue: false
  - receiver: blackhole
    matchers:
    - alertname=~"WorkloadClusterApp.*"
    continue: false
  - receiver: blackhole
    matchers:
    - alertname="PrometheusMetaOperatorReconcileErrors"
    continue: false
  - receiver: blackhole
    matchers:
    - alertname="ClusterUnhealthyPhase"
    continue: false
  - receiver: blackhole
    matchers:
    - alertname="ClusterUnhealthyPhase"
    - name=~"t-.*"
    continue: false
  # We don't want to get alerts by workload cluster apps that are failing.
  # We select those by checking if the App CR is in a namespace starting with 'org-'.
  - receiver: blackhole
    matchers:
    - alertname="ManagementClusterAppFailed"
    - namespace=~"org-.*"
    continue: false
  {{- end }}

  # Falco noise Slack
  - receiver: falco_noise_slack
    matchers:
    - alertname=~"Falco.*"
    continue: false

  - receiver: team_tenet_slack
    repeat_interval: 14d
    matchers:
    - severity=~"page|notify"
    - team=~"tenet|tinkerers"
    continue: false

  # Team Ops Opsgenie
  - receiver: opsgenie_router
    matchers:
    - severity="page"
    continue: true

  # Team Atlas Slack
  - receiver: team_atlas_slack
    matchers:
    {{- if eq .Pipeline "stable" }}
    - severity="notify"
    {{- else }}
    - severity=~"page|notify"
    {{- end }}
    - team="atlas"
    - type!="heartbeat"
    - alertname!~"Inhibition.*"
    - alertname!="Heartbeat"
    continue: false

  # Team Celestial Slack
  - receiver: team_phoenix_slack
    matchers:
    - severity=~"page|notify"
    - team="celestial"
    - sloth_severity=~"page|ticket"
    continue: false

  # Team Firecracker Slack
  - receiver: team_phoenix_slack
    matchers:
    - severity=~"page|notify"
    - team="firecracker"
    - sloth_severity=~"page|ticket"
    continue: false

  # Team Phoenix Slack
  - receiver: team_phoenix_slack
    matchers:
    - team="phoenix"
    - sloth_severity="page"
    - silence="true"
    continue: false

  # Team Shield Slack
  - receiver: team_shield_slack
    matchers:
    - severity=~"page|notify"
    - team="shield"
    continue: false

  # Team BigMac Slack
  - receiver: team_bigmac_slack
    matchers:
    - severity=~"page|notify"
    - team="bigmac"
    continue: false

  # Team Clippy Slack
  # ReRoute to `phoenix` until we change all team ownership labels
  - receiver: team_phoenix_slack
    matchers:
    - severity=~"page|notify"
    - team="clippy"
    continue: false

  # Team Rocket Slack
  - receiver: team_rocket_slack
    matchers:
    - severity=~"page|notify"
    - team="rocket"
    continue: false

  # Team Turtles Slack
  - receiver: team_turtles_slack
    matchers:
    - severity=~"page|notify"
    - team="turtles"
    continue: false

  # Team Honeybadger Slack
  - receiver: team_honeybadger_slack
    matchers:
    - severity=~"page|notify"
    - team="honeybadger"
    continue: false

receivers:
- name: root

- name: heartbeat
  webhook_configs:
  - send_resolved: false
    http_config:
      authorization:
        type: GenieKey
        credentials: {{ .OpsgenieKey }}
      follow_redirects: true
      enable_http2: true
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    url: https://api.opsgenie.com/v2/heartbeats/{{ .Installation }}/ping

{{- if eq .Pipeline "stable-testing" }}
- name: blackhole
{{- end }}

- name: falco_noise_slack
  slack_configs:
  - channel: '#noise-falco'
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: &slack-actions
    - type: button
      text: ':green_book: OpsRecipe'
      url: {{`{{ template "__runbookurl" . }}`}}
      style: {{`{{ if eq .Status "firing" }}primary{{ else }}default{{ end }}`}}
    - type: button
      text: ':coffin: Linked PMs'
      url: {{`{{ template "__alert_linked_postmortems" . }}`}}
    - type: button
      text: ':mag: Query'
      url: {{`{{ template "__alerturl" . }}`}}
    - type: button
      text: ':grafana: Dashboard'
      url: {{`{{ template "__dashboardurl" . }}`}}
    - type: button
      text: ':no_bell: Silence'
      url: {{`{{ template "__alert_silence_link" .}}`}}
      style: {{`{{ if eq .Status "firing" }}danger{{ else }}default{{ end }}`}}

- name: team_atlas_slack
  slack_configs:
  {{- if eq .Pipeline "stable" }}
  - channel: '#alert-atlas'
  {{- else }}
  - channel: '#alert-atlas-test'
  {{- end }}
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: team_phoenix_slack
  slack_configs:
  {{- if eq .Pipeline "stable" }}
  - channel: '#alert-phoenix'
  {{- else }}
  - channel: '#alert-phoenix-test'
  {{- end }}
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: team_bigmac_slack
  slack_configs:
  {{- if eq .Pipeline "stable" }}
  - channel: '#alert-bigmac'
  {{- else }}
  - channel: '#alert-bigmac-test'
  {{- end }}
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: team_rocket_slack
  slack_configs:
  {{- if eq .Pipeline "stable" }}
  - channel: '#alert-rocket'
  {{- else }}
  - channel: '#alert-rocket-test'
  {{- end }}
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: team_shield_slack
  slack_configs:
  - channel: '#alert-shield'
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: team_turtles_slack
  slack_configs:
  {{- if eq .Pipeline "stable" }}
  - channel: '#alert-turtles'
  {{- else }}
  - channel: '#alert-turtles-test'
  {{- end }}
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: team_tenet_slack
  slack_configs:
  - channel: '#alert-tenet'
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: team_honeybadger_slack
  slack_configs:
  - channel: '#alert-honeybadger'
    {{- if .SlackApiToken }}
    http_config:
      authorization:
        type: Bearer
        credentials: {{ .SlackApiToken }}
      {{- if .ProxyURL }}
      proxy_url:  {{ .ProxyURL }}
      {{- end }}
    {{- end }}
    send_resolved: true
    actions: *slack-actions

- name: opsgenie_router
  opsgenie_configs:
  - api_key: {{ .OpsgenieKey }}
    tags: {{`{{ (index .Alerts 0).Labels.alertname }},{{ (index .Alerts 0).Labels.cluster_type }},{{ (index .Alerts 0).Labels.severity }},{{ (index .Alerts 0).Labels.team }},{{ (index .Alerts 0).Labels.area }},{{ (index .Alerts 0).Labels.service_priority }},{{ (index .Alerts 0).Labels.provider }},{{ (index .Alerts 0).Labels.installation }},{{ (index .Alerts 0).Labels.pipeline }},{{ (index .Alerts 0).Labels.customer }}`}}

inhibit_rules:
- source_matchers:
  - inhibit_kube_state_metrics_down=true
  target_matchers:
  - cancel_if_kube_state_metrics_down=true
  equal: [cluster_id]

- source_matchers:
  - inhibit_kube_state_metrics_down=true
  - cluster_id={{ .Installation }}
  target_matchers:
  - cancel_if_mc_kube_state_metrics_down=true

- source_matchers:
  - inhibit_kube_state_metrics_down=true
  target_matchers:
  - cancel_if_any_kube_state_metrics_down=true

- source_matchers:
  - cluster_status_creating=true
  target_matchers:
  - cancel_if_cluster_status_creating=true
  equal: [cluster_id]

- source_matchers:
  - cluster_status_created=true
  target_matchers:
  - cancel_if_cluster_status_created=true
  equal: [cluster_id]

- source_matchers:
  - cluster_status_updating=true
  target_matchers:
  - cancel_if_cluster_status_updating=true
  equal: [cluster_id]

- source_matchers:
  - cluster_status_updated=true
  target_matchers:
  - cancel_if_cluster_status_updated=true
  equal: [cluster_id]

- source_matchers:
  - cluster_status_deleting=true
  target_matchers:
  - cancel_if_cluster_status_deleting=true
  equal: [cluster_id]

- source_matchers:
  - cluster_with_no_nodepools=true
  target_matchers:
  - cancel_if_cluster_with_no_nodepools=true
  equal: [cluster_id]

- source_matchers:
  - cluster_with_scaling_nodepools=true
  target_matchers:
  - cancel_if_cluster_with_scaling_nodepools=true
  equal: [cluster_id]

- source_matchers:
  - cluster_with_notready_nodepools=true
  target_matchers:
  - cancel_if_cluster_with_notready_nodepools=true
  equal: [cluster_id]

- source_matchers:
  - cluster_control_plane_unhealthy=true
  target_matchers:
  - cancel_if_cluster_control_plane_unhealthy=true
  equal: [cluster_id]

- source_matchers:
  - cluster_control_plane_unhealthy=true
  target_matchers:
  - cancel_if_any_cluster_control_plane_unhealthy=true

- source_matchers:
  - instance_state_not_running=true
  target_matchers:
  - cancel_if_instance_state_not_running=true
  equal: [node]

- source_matchers:
  - kiam_has_errors=true
  target_matchers:
  - cancel_if_kiam_has_errors=true
  equal: [cluster_id]

- source_matchers:
  - kubelet_down=true
  target_matchers:
  - cancel_if_kubelet_down=true
  equal: [cluster_id, ip]

- source_matchers:
  - control_plane_node_down=true
  target_matchers:
  - cancel_if_control_plane_node_down=true
  equal: [cluster_id]

- source_matchers:
  - outside_working_hours=true
  target_matchers:
  - cancel_if_outside_working_hours=true

- source_matchers:
  - has_worker_nodes=false
  target_matchers:
  - cancel_if_cluster_has_no_workers=true
  equal: [cluster_id]

- source_matchers:
    - cluster_is_not_running_monitoring_agent=true
  target_matchers:
    - cancel_if_cluster_is_not_running_monitoring_agent=true
  equal: [cluster_id]

- source_matchers:
    - inhibit_monitoring_agent_down=true
  target_matchers:
    - cancel_if_monitoring_agent_down=true
  equal: [cluster_id]

- source_matchers:
    - stack_failed=true
  target_matchers:
    - cancel_if_stack_failed=true

# Source: https://github.com/giantswarm/prometheus-rules/blob/main/helm/prometheus-rules/templates/kaas/turtles/alerting-rules/inhibit.nodes.rules.yml
- source_matchers:
    - node_not_ready=true
  target_matchers:
    - cancel_if_node_not_ready=true
  equal: [cluster_id, node]

# Source: https://github.com/giantswarm/prometheus-rules/blob/main/helm/prometheus-rules/templates/kaas/turtles/alerting-rules/inhibit.nodes.rules.yml
- source_matchers:
    - node_unschedulable=true
  target_matchers:
    - cancel_if_node_unschedulable=true
  equal: [cluster_id, node]
