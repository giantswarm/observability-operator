# Alloy Logs Migration - TODO

This package has been migrated from the logging-operator repository with the structure and implementation complete. However, the actual template files need to be copied from the source repository.

## Template Files to Copy

You need to copy the following template files from the logging-operator repository:

### 1. logging.alloy.template
**Source:** `https://github.com/giantswarm/logging-operator/blob/main/pkg/resource/logging-config/alloy/logging.alloy.template`
**Destination:** `pkg/logging/alloy/logs/templates/logging.alloy.template`

This is the main Alloy River configuration template containing:
- Log collection from Kubernetes pods
- Log forwarding to Loki with multi-tenant support
- Cluster labels and metadata
- Node filtering (conditional based on enableNodeFiltering flag)
- Network monitoring with Beyla (conditional based on enableNetworkMonitoring flag)

### 2. logging-config.yaml.template
**Source:** `https://github.com/giantswarm/logging-operator/blob/main/pkg/resource/logging-config/alloy/logging-config.alloy.yaml.template`
**Destination:** `pkg/logging/alloy/logs/templates/logging-config.yaml.template`

This is the Helm values wrapper template containing:
- The River configuration (from logging.alloy.template)
- Default workload cluster namespaces
- Image tag override (for bundle versions < 2.4.0)
- Priority class configuration
- Network monitoring and node filtering flags

### 3. alloy-secret.yaml.template
**Source:** `https://github.com/giantswarm/logging-operator/blob/main/pkg/resource/logging-secret/alloy/alloy-secret.yaml.template`
**Destination:** `pkg/logging/alloy/logs/templates/alloy-secret.yaml.template`

This is the Helm values template for secrets containing:
- Loki write URL (https://host/loki/api/v1/push)
- Loki tenant ID (giantswarm)
- Loki username (cluster name)
- Loki password (from auth manager)
- Loki ruler API URL
- Tempo tracing credentials (conditional)

## How to Copy Templates

### Option 1: Clone the logging-operator repository
```bash
# Clone the logging-operator repository
git clone https://github.com/giantswarm/logging-operator.git /tmp/logging-operator

# Copy template files
cp /tmp/logging-operator/pkg/resource/logging-config/alloy/logging.alloy.template \
   pkg/logging/alloy/logs/templates/logging.alloy.template

cp /tmp/logging-operator/pkg/resource/logging-config/alloy/logging-config.alloy.yaml.template \
   pkg/logging/alloy/logs/templates/logging-config.yaml.template

cp /tmp/logging-operator/pkg/resource/logging-secret/alloy/alloy-secret.yaml.template \
   pkg/logging/alloy/logs/templates/alloy-secret.yaml.template

# Clean up
rm -rf /tmp/logging-operator
```

### Option 2: Download files directly
```bash
# Download using curl or wget
curl -o pkg/logging/alloy/logs/templates/logging.alloy.template \
  https://raw.githubusercontent.com/giantswarm/logging-operator/main/pkg/resource/logging-config/alloy/logging.alloy.template

curl -o pkg/logging/alloy/logs/templates/logging-config.yaml.template \
  https://raw.githubusercontent.com/giantswarm/logging-operator/main/pkg/resource/logging-config/alloy/logging-config.alloy.yaml.template

curl -o pkg/logging/alloy/logs/templates/alloy-secret.yaml.template \
  https://raw.githubusercontent.com/giantswarm/logging-operator/main/pkg/resource/logging-secret/alloy/alloy-secret.yaml.template
```

## Golden Test Files

If you want to add the golden test files for testing, you should also copy:

```bash
# Create test directory
mkdir -p pkg/logging/alloy/logs/testdata

# Copy golden files (11 files total)
for file in \
  logging-config.alloy.170_MC.yaml \
  logging-config.alloy.170_WC.yaml \
  logging-config.alloy.170_WC_default_namespaces_nil.yaml \
  logging-config.alloy.170_WC_default_namespaces_empty.yaml \
  logging-config.alloy.170_WC_custom_tenants.yaml \
  logging-config.alloy.170_MC_node_filtering.yaml \
  logging-config.alloy.170_WC_node_filtering.yaml \
  logging-config.alloy.240_WC_node_filtering.yaml \
  logging-config.alloy.230_MC_network_monitoring.yaml \
  logging-config.alloy.230_WC_network_monitoring.yaml \
  logging-config.alloy.230_WC_network_monitoring_node_filtering.yaml
do
  curl -o "pkg/logging/alloy/logs/testdata/$file" \
    "https://raw.githubusercontent.com/giantswarm/logging-operator/main/pkg/resource/logging-config/alloy/test/$file"
done
```

## Package Structure

The migrated package has the following structure:

```
pkg/logging/alloy/logs/
├── service.go              # Main service with ReconcileCreate/ReconcileDelete
├── configmap.go            # ConfigMap generation functions
├── secret.go               # Secret generation functions
├── constants.go            # Constants (timeouts, URLs, keys, version constraints)
├── templates/              # Template files (need to be copied)
│   ├── logging.alloy.template
│   ├── logging-config.yaml.template
│   └── alloy-secret.yaml.template
└── testdata/               # Golden test files (optional, for testing)
    ├── logging-config.alloy.170_MC.yaml
    ├── logging-config.alloy.170_WC.yaml
    └── ... (11 files total)
```

## Integration with observability-operator

The service is ready to be integrated into the cluster controller:

1. **Add to cluster_controller.go:**
```go
// In SetupClusterMonitoringReconciler function
alloyLogsService := &logs.Service{
    Client:                 managerClient,
    OrganizationRepository: orgRepo,
    Config:                 cfg,
    LogsAuthManager:        lokiAuthManager,
    TracesAuthManager:      tempoAuthManager,
}

reconciler := &ClusterMonitoringReconciler{
    // ... existing fields
    AlloyLogsService: alloyLogsService,
}
```

2. **In reconcile function:**
```go
// Logs-specific: Alloy logging configuration
if r.Config.Logging.IsLoggingEnabled(cluster) {
    err = r.AlloyLogsService.ReconcileCreate(ctx, cluster, observabilityBundleVersion)
    if err != nil {
        logger.Error(err, "failed to create or update alloy logging config")
        return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
    }
} else {
    err = r.AlloyLogsService.ReconcileDelete(ctx, cluster)
    if err != nil {
        logger.Error(err, "failed to delete alloy logging config")
        return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
    }
}
```

## Features Implemented

- ✅ ConfigMap generation with multi-layered approach (River → Helm → ConfigMap)
- ✅ Secret generation with Loki and Tempo credentials
- ✅ Network monitoring support (Beyla, requires bundle >= 2.3.0)
- ✅ Node filtering support (requires Alloy 1.12.0, bundle >= 2.4.0)
- ✅ Tracing support (Tempo, requires bundle >= 1.11.0)
- ✅ Multi-tenancy with default "giantswarm" tenant
- ✅ Version-aware feature enablement
- ✅ Image tag override for older bundle versions
- ✅ Reconciliation with create/update/delete logic
- ✅ Deep equality checks for update detection
- ⏳ Template files (need to be copied)
- ⏳ Golden test files (optional, for testing)

## Testing

After copying the templates, you can test the implementation:

```bash
# Build the operator
make build

# Run tests (once golden files are copied)
go test -v ./pkg/logging/alloy/logs/...

# Update golden files if needed
make update-golden-files
```

## References

- [logging-operator repository](https://github.com/giantswarm/logging-operator)
- [Alloy documentation](https://grafana.com/docs/alloy/latest/)
- [Loki documentation](https://grafana.com/docs/loki/latest/)
- [River configuration syntax](https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/syntax/)
