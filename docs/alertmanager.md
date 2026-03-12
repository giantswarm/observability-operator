# Alertmanager Configuration

The operator watches for Kubernetes `Secrets` containing Alertmanager configurations and pushes them to the Mimir Alertmanager API for the appropriate tenant.

## How it works

1. Create a `Secret` with the required labels and your Alertmanager config as the secret data
2. The operator detects the secret, validates it, assembles the final config (merging with the installation-level config), and pushes it to Mimir Alertmanager for the specified tenant
3. On deletion, the operator removes the config from Mimir Alertmanager

## Required labels

| Label/Annotation | Where | Description |
|---|---|---|
| `observability.giantswarm.io/kind: alertmanager-config` | label | Marks the secret as an Alertmanager config |
| `observability.giantswarm.io/tenant: <tenant>` | label or annotation | The tenant this config belongs to |

## Limitations

- One secret per tenant. If two secrets target the same tenant, behaviour is undefined — the operator may overwrite one with the other.
- The tenant name must match a tenant configured in a `GrafanaOrganization`. See [grafana-organization.md](grafana-organization.md).

## Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-team-alertmanager-config
  namespace: monitoring
  labels:
    observability.giantswarm.io/kind: alertmanager-config
    observability.giantswarm.io/tenant: my_team
stringData:
  alertmanager.yaml: |
    route:
      receiver: my-team-slack
      group_by: [alertname, cluster]
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 4h

    receivers:
      - name: my-team-slack
        slack_configs:
          - api_url: https://hooks.slack.com/services/...
            channel: '#my-team-alerts'
            title: '{{ .GroupLabels.alertname }}'
            text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

## Validation

A validating webhook checks the secret before it is accepted. It verifies:
- The `observability.giantswarm.io/tenant` label or annotation is present
- The Alertmanager YAML is syntactically valid

## Testing

Use the BATS and integration test suite to validate routing logic without a live cluster. See [tests/alertmanager-routes/README.md](../tests/alertmanager-routes/README.md) for details.
