# Alloy Logs Migration - TODO

This migration brings the alloy-logs configuration logic from the logging-operator repository into the observability-operator.

## ✅ Completed

- [x] Created service structure following alloy-events pattern
- [x] Implemented `ReconcileCreate` using `controllerutil.CreateOrUpdate`
- [x] Implemented `ReconcileDelete` 
- [x] Removed tracing support (only needed for events)
- [x] Removed `ClusterLabels` struct (not needed)
- [x] Moved constants to `pkg/common/monitoring` (except versioning semantics)
- [x] Created ConfigMap generation functions
- [x] Created Secret generation functions
- [x] Added proper error handling and logging
- [x] Used `labels.Common` for resource labels

## 📋 Remaining Tasks

### 1. Template Files (HIGH PRIORITY)
The template files currently contain placeholder content. They need to be copied from the logging-operator repository:

**Source Repository:** https://github.com/giantswarm/logging-operator

**Files to copy:**
- `pkg/resource/logging-config/alloy/logging.alloy.template` 
  → `pkg/logging/alloy/logs/templates/logging.alloy.template`
  
- `pkg/resource/logging-config/alloy/logging-config.alloy.yaml.template`
  → `pkg/logging/alloy/logs/templates/logging-config.yaml.template`
  
- `pkg/resource/logging-secret/alloy/alloy-secret.yaml.template`
  → `pkg/logging/alloy/logs/templates/alloy-secret.yaml.template`

### 2. Test Files
Create test files with golden file testing pattern:

**Required files:**
- `pkg/logging/alloy/logs/configmap_test.go` - Test configuration generation
- `pkg/logging/alloy/logs/testdata/` - Directory for golden files

**Test scenarios to cover:**
- Management cluster (MC) vs Workload cluster (WC)
- Default namespaces: nil vs empty vs populated
- Custom tenants vs default tenant only
- Node filtering enabled/disabled
- Network monitoring enabled/disabled  
- Version-specific image tag override (bundle < 2.4.0)
- Various observability bundle versions (1.7.0, 2.3.0, 2.4.0)

**Use environment variable:** Check `UPDATE_GOLDEN_FILES` instead of `-update` flag:
```go
var updateGoldenFiles = os.Getenv("UPDATE_GOLDEN_FILES") != ""
```

### 3. Integration with Controller
Wire up the service in the cluster controller:

**File:** `internal/controller/cluster_controller.go`

**Changes needed:**
1. Add AlloyLogsService to ClusterMonitoringReconciler struct
2. Initialize service in SetupClusterMonitoringReconciler
3. Call ReconcileCreate/ReconcileDelete in reconcile methods
4. Add proper error handling and requeue logic

**Pattern to follow:** Check how AlloyEventsService is integrated

### 4. Makefile Updates
Add target for updating golden files:

```makefile
.PHONY: update-golden-files-logs
update-golden-files-logs:
	UPDATE_GOLDEN_FILES=1 go test -v ./pkg/logging/alloy/logs
```

### 5. Documentation
- [ ] Update README.md with alloy-logs information
- [ ] Document version constraints and feature flags
- [ ] Add examples of generated configurations

## 🔍 Implementation Notes

### Version Constraints
- **Network Monitoring:** Requires observability-bundle >= 2.3.0
- **Node Filtering Fix:** observability-bundle >= 2.4.0, otherwise uses Alloy v1.12.0
- Network monitoring forces node filtering (Beyla incompatible with clustering)

### Configuration Flow
1. **River Config Generation** (`generateAlloyConfig`)
   - Generates Alloy's native River DSL configuration
   - Includes cluster metadata, tenants, credentials references
   
2. **Helm Values Wrapping** (`generateAlloyLoggingConfig`)
   - Wraps River config in Helm values structure
   - Handles version-specific image tag override
   
3. **ConfigMap Creation** (`GenerateAlloyLogsConfigMapData`)
   - Final wrapper in Kubernetes ConfigMap format
   - Used by observability-bundle to configure alloy-logs app

### Secret Generation
- Retrieves passwords via `LogsAuthManager.GetClusterPassword()`
- Formats Loki push URL: `https://write.loki.<basedomain>/loki/api/v1/push`
- Always uses "giantswarm" as default tenant
- No tracing credentials (unlike events logger)

## 📚 Reference Implementation

**Events Logger:** `pkg/logging/alloy/events/`
- Similar pattern for ConfigMap and Secret generation
- Uses `controllerutil.CreateOrUpdate`
- Good example for testing patterns

**Monitoring Alloy:** `pkg/monitoring/alloy/`  
- Shows integration with controller
- Demonstrates version-specific logic

## 🧪 Testing Checklist

Before marking this migration complete:
- [ ] Unit tests pass with golden files
- [ ] Integration test on dev cluster
- [ ] Verify ConfigMap generation for MC and WC
- [ ] Verify Secret generation with correct credentials
- [ ] Test with network monitoring enabled
- [ ] Test with node filtering enabled
- [ ] Verify version-specific image tag override
- [ ] Check that default tenant "giantswarm" is always included

## 🚀 Next Steps

1. **Immediate:** Copy template files from logging-operator
2. **High Priority:** Create tests with golden files  
3. **Medium Priority:** Integrate with controller
4. **Low Priority:** Documentation updates
