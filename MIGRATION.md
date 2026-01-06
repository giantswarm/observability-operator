# Detailed Migration Plan: Consolidate logging-operator into observability-operator

## Current Status (January 5, 2026)

### ✅ Completed Phases

#### Phase 1: Configuration & Command-Line Flags
- ✅ All logging-related command-line flags implemented in [cmd/main.go](cmd/main.go)
  - `--logging-enabled`
  - `--logging-default-namespaces`
  - `--logging-enable-node-filtering`
  - `--logging-enable-network-monitoring`
  - `--logging-include-events-from-namespaces`
  - `--logging-exclude-events-from-namespaces`
- ✅ Configuration struct implemented in [pkg/config/logging.go](pkg/config/logging.go)
  - Includes validation logic
  - Includes `IsLoggingEnabled()` helper method
- ✅ Authentication integration in [internal/controller/cluster_controller.go](internal/controller/cluster_controller.go)
  - Loki auth manager configured
  - Auth manager map includes logs with feature check

#### Phase 2: Events Logger Services Implementation
- ✅ **EVENTS SERVICE COMPLETE** - Following monitoring/alloy pattern
  - ✅ Created `pkg/logging/alloy/events/` package structure
  - ✅ Implemented `service.go` with ReconcileCreate/ReconcileDelete
  - ✅ Implemented `configmap.go` with template generation
  - ✅ Implemented `secret.go` with credential management
  - ✅ Created template files:
    - `templates/events-logger.alloy.template` (Alloy River config)
    - `templates/events-logger-config.alloy.yaml.template` (Helm values)
    - `templates/events-logger-secret.yaml.template` (Secret values)
  - ✅ Created test file `configmap_test.go`
  - ✅ Created testdata golden files for verification
  - ✅ Updated `pkg/common/monitoring/monitoring.go` with Loki/Tempo URL formats
- ❌ **LOGS SERVICE PENDING** - Still needs implementation

#### Phase 4: Helm Chart Updates
- ✅ Helm chart values.yaml updated with logging section
- ✅ Deployment template updated with all logging flags
- ✅ All configuration options exposed via Helm

### 🚧 In Progress Phases

#### Phase 3: Controller Integration
- ⚠️ **NEEDS UPDATE** - Controller needs to use new events package structure
  - Current controller references old `pkg/logging/alloy` package
  - Needs to import and use `pkg/logging/alloy/events` package
  - Service initialization needs update
  - RBAC permissions already present

### ❌ Not Started Phases

#### Phase 5: Deprecate logging-operator
- ❌ Deprecation notices not added
- ❌ Cleanup controller not created
- ❌ README not updated with deprecation notice

#### Phase 6: Testing
- ⚠️ Unit test structure created but needs actual test runs
- ❌ Integration tests not created

### Next Steps

**COMPLETED: Events Service Implementation**
- ✅ Full events service implementation following observability-operator patterns
- ✅ Template files identical to logging-operator source
- ✅ Test structure in place with golden files
- ✅ Code follows monitoring/alloy structure conventions

**PR #1: Events Logger Service (Ready for Review)**
- ✅ Complete events service in `pkg/logging/alloy/events/`
- ✅ Template files copied and adapted from logging-operator
- ✅ Test structure with testdata golden files
- ✅ Common monitoring constants updated
- ⚠️ Controller integration needs update to use new package
- Next: Update controller to import and use events package

**PR #2: LogsService Implementation** (After Events PR)
1. Implement logs service following same pattern as events
2. Create `pkg/logging/alloy/logs/` package
3. Implement service.go, configmap.go, secret.go
4. Copy template files from logging-operator
5. Add unit tests and golden files

**PR #3: Controller Integration Update**
1. Update [internal/controller/cluster_controller.go](internal/controller/cluster_controller.go)
   - Import `pkg/logging/alloy/events` and `pkg/logging/alloy/logs` packages
   - Update service initialization to use new constructors
   - Ensure proper auth manager passing (logs vs traces)
2. Test on development cluster

**PR #4: Integration Testing** (After all services implemented)
1. Implement helper functions in [pkg/logging/alloy/helpers.go](pkg/logging/alloy/helpers.go)
   - Check if functions exist in pkg/common and reuse if available
   - Implement missing helper functions
2. Add unit tests

**PR #6: Integration Testing** (After PRs #3-5)
1. Test on development cluster with logging enabled
2. Verify ConfigMaps and Secrets are created
3. Verify logs flow to Loki
4. Update documentation

**PR #7: Deprecate logging-operator** (After successful deployment)
1. Add deprecation notices
2. Create cleanup controller
3. Update README

### Migration Progress: ~40% Complete (Skeleton + Integration)

---

## Context

The logging-operator currently handles Alloy logs and events configuration for workload clusters. Most authentication and secret management has already been migrated to observability-operator (v0.40.0+). This plan consolidates all remaining functionality into observability-operator and deprecates logging-operator.

## Prerequisites

- observability-operator v0.54.0 or later (current dependency)
- Access to both repositories
- Kubernetes cluster for testing
- Understanding of CAPI cluster reconciliation

## Phase 1: Extend observability-operator Configuration

### Task 1.1: Add Logging Configuration Flags

**File:** `observability-operator/cmd/main.go`

**Location:** After line 172 (after `--logging-enabled` flag)

**Add these flag definitions:**

```go
// Logging configuration flags
flag.BoolVar(&cfg.Logging.Enabled, "logging-enabled", false,
	"Enable logging at the installation level.")
flag.Var(&defaultLoggingNamespaces, "logging-default-namespaces",
	"Comma-separated list of namespaces to collect logs from by default on workload clusters")
flag.BoolVar(&cfg.Logging.EnableNodeFiltering, "logging-enable-node-filtering", false,
	"Enable/disable node filtering in Alloy logging configuration")
flag.BoolVar(&cfg.Logging.EnableNetworkMonitoring, "logging-enable-network-monitoring", false,
	"Enable/disable network monitoring in Alloy logging configuration")
flag.Var(&includeEventsFromNamespaces, "logging-include-events-from-namespaces",
	"Comma-separated list of namespaces to collect events from on workload clusters (if empty, collect from all)")
flag.Var(&excludeEventsFromNamespaces, "logging-exclude-events-from-namespaces",
	"Comma-separated list of namespaces to exclude events from on workload clusters")
```

**Before the flag definitions, add StringSliceVar type:**

```go
type StringSliceVar []string

func (s StringSliceVar) String() string {
	return strings.Join(s, ",")
}

func (s *StringSliceVar) Set(value string) error {
	*s = strings.Split(value, ",")
	return nil
}
```

**Declare the variables before parseFlags():**

```go
var (
	defaultLoggingNamespaces      StringSliceVar
	includeEventsFromNamespaces   StringSliceVar
	excludeEventsFromNamespaces   StringSliceVar
	// ... existing vars ...
)
```

**After flag.Parse(), assign to config:**

```go
cfg.Logging.DefaultNamespaces = defaultLoggingNamespaces
cfg.Logging.IncludeEventsNamespaces = includeEventsFromNamespaces
cfg.Logging.ExcludeEventsNamespaces = excludeEventsFromNamespaces
```

### Task 1.2: Update Logging Config Structure

**File:** `observability-operator/pkg/config/logging.go`

**Replace the LoggingConfig struct:**

```go
// LoggingConfig represents the configuration used by the logging package.
type LoggingConfig struct {
	// Enabled controls logging at the installation level
	Enabled bool

	// EnableNodeFiltering enables node filtering in Alloy logging configuration
	EnableNodeFiltering bool

	// EnableNetworkMonitoring enables network monitoring in Alloy logging configuration
	EnableNetworkMonitoring bool

	// DefaultNamespaces is the list of namespaces to collect logs from by default
	DefaultNamespaces []string

	// IncludeEventsNamespaces is the list of namespaces to collect events from
	// If empty, collect from all namespaces
	IncludeEventsNamespaces []string

	// ExcludeEventsNamespaces is the list of namespaces to exclude events from
	ExcludeEventsNamespaces []string
}
```

**Update the Validate method:**

```go
// Validate validates the logging configuration
func (l LoggingConfig) Validate() error {
	// Check for conflicting namespace configurations
	if len(l.IncludeEventsNamespaces) > 0 && len(l.ExcludeEventsNamespaces) > 0 {
		return fmt.Errorf("cannot specify both include and exclude events namespaces")
	}
	return nil
}
```

## Phase 2: Create Logging Services in observability-operator

### Task 2.1: Create Package Structure

**Create these directories:**

```bash
mkdir -p observability-operator/pkg/logging/alloy/templates
```

### Task 2.2: Copy and Adapt Alloy Logs Service

**File:** `observability-operator/pkg/logging/alloy/logs_service.go`

**Copy from:** `logging-operator/pkg/resource/logging-secret/` and `logging-operator/pkg/resource/logging-config/`

**Create new service combining both:**

```go
package alloy

import (
	"context"
	_ "embed"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	LogsConfigMapName = "alloy-logs-config"
	LogsSecretName    = "alloy-logs-secret"
)

var (
	//go:embed templates/logs-config.alloy.yaml.template
	logsConfigTemplate string
	
	//go:embed templates/logs.alloy.template
	logsAlloyTemplate string
	
	//go:embed templates/logs-secret.yaml.template
	logsSecretTemplate string
)

type LogsService struct {
	client.Client
	config.Config
	auth.AuthManager
}

func NewLogsService(client client.Client, cfg config.Config, authManager auth.AuthManager) *LogsService {
	return &LogsService{
		Client:      client,
		Config:      cfg,
		AuthManager: authManager,
	}
}

func (s *LogsService) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-logs-service - ensuring alloy logs is configured")

	// Create or update secret
	secret := s.logsSecret(cluster)
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, secret, func() error {
		data, err := s.generateLogsSecretData(ctx, cluster)
		if err != nil {
			return fmt.Errorf("failed to generate logs secret data: %w", err)
		}
		secret.Data = data
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update logs secret: %w", err)
	}

	// Create or update configmap
	configmap := s.logsConfigMap(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, s.Client, configmap, func() error {
		data, err := s.generateLogsConfigMapData(ctx, cluster, observabilityBundleVersion)
		if err != nil {
			return fmt.Errorf("failed to generate logs config data: %w", err)
		}
		configmap.Data = data
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update logs configmap: %w", err)
	}

	logger.Info("alloy-logs-service - ensured alloy logs is configured")
	return nil
}

func (s *LogsService) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-logs-service - ensuring alloy logs is removed")

	configmap := s.logsConfigMap(cluster)
	err := s.Delete(ctx, configmap)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete logs configmap: %w", err)
	}

	secret := s.logsSecret(cluster)
	err = s.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete logs secret: %w", err)
	}

	logger.Info("alloy-logs-service - ensured alloy logs is removed")
	return nil
}

func (s *LogsService) logsConfigMap(cluster *clusterv1.Cluster) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, LogsConfigMapName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}
}

func (s *LogsService) logsSecret(cluster *clusterv1.Cluster) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, LogsSecretName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}
}

func (s *LogsService) generateLogsSecretData(ctx context.Context, cluster *clusterv1.Cluster) (map[string][]byte, error) {
	// Copy implementation from logging-operator/pkg/resource/logging-secret/alloy-logging-secret.go
	// Update to use s.AuthManager instead of passed auth managers
	// Update to use s.Config for installation settings
	// See logging-operator implementation for details
	return nil, fmt.Errorf("not implemented - copy from logging-operator")
}

func (s *LogsService) generateLogsConfigMapData(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) (map[string]string, error) {
	// Copy implementation from logging-operator/pkg/resource/logging-config/alloy-logging-config.go
	// Update to use s.Config.Logging for all settings
	// See logging-operator implementation for details
	return nil, fmt.Errorf("not implemented - copy from logging-operator")
}
```

### Task 2.3: Copy and Adapt Alloy Events Service

**File:** `observability-operator/pkg/logging/alloy/events_service.go`

**Copy from:** `logging-operator/pkg/resource/events-logger-secret/` and `logging-operator/pkg/resource/events-logger-config/`

**Structure similar to logs_service.go:**

```go
package alloy

import (
	"context"
	_ "embed"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver/v4"

	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/common/labels"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const (
	EventsConfigMapName = "alloy-events-config"
	EventsSecretName    = "alloy-events-secret"
)

var (
	//go:embed templates/events-config.alloy.yaml.template
	eventsConfigTemplate string
	
	//go:embed templates/events.alloy.template
	eventsAlloyTemplate string
	
	//go:embed templates/events-secret.yaml.template
	eventsSecretTemplate string
)

type EventsService struct {
	client.Client
	config.Config
	LogsAuthManager   auth.AuthManager
	TracesAuthManager auth.AuthManager
}

func NewEventsService(client client.Client, cfg config.Config, logsAuth auth.AuthManager, tracesAuth auth.AuthManager) *EventsService {
	return &EventsService{
		Client:            client,
		Config:            cfg,
		LogsAuthManager:   logsAuth,
		TracesAuthManager: tracesAuth,
	}
}

func (s *EventsService) ReconcileCreate(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is configured")

	// Create or update secret
	secret := s.eventsSecret(cluster)
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, secret, func() error {
		data, err := s.generateEventsSecretData(ctx, cluster)
		if err != nil {
			return fmt.Errorf("failed to generate events secret data: %w", err)
		}
		secret.Data = data
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update events secret: %w", err)
	}

	// Create or update configmap
	configmap := s.eventsConfigMap(cluster)
	_, err = controllerutil.CreateOrUpdate(ctx, s.Client, configmap, func() error {
		data, err := s.generateEventsConfigMapData(ctx, cluster, observabilityBundleVersion)
		if err != nil {
			return fmt.Errorf("failed to generate events config data: %w", err)
		}
		configmap.Data = data
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update events configmap: %w", err)
	}

	logger.Info("alloy-events-service - ensured alloy events is configured")
	return nil
}

func (s *EventsService) ReconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("alloy-events-service - ensuring alloy events is removed")

	configmap := s.eventsConfigMap(cluster)
	err := s.Delete(ctx, configmap)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete events configmap: %w", err)
	}

	secret := s.eventsSecret(cluster)
	err = s.Delete(ctx, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete events secret: %w", err)
	}

	logger.Info("alloy-events-service - ensured alloy events is removed")
	return nil
}

func (s *EventsService) eventsConfigMap(cluster *clusterv1.Cluster) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, EventsConfigMapName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}
}

func (s *EventsService) eventsSecret(cluster *clusterv1.Cluster) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cluster.Name, EventsSecretName),
			Namespace: cluster.Namespace,
			Labels:    labels.Common,
		},
	}
}

func (s *EventsService) generateEventsSecretData(ctx context.Context, cluster *clusterv1.Cluster) (map[string][]byte, error) {
	// Copy implementation from logging-operator/pkg/resource/events-logger-secret/events-logger-secret.go
	// This reuses the logs secret generation but for events
	// See logging-operator implementation for details
	return nil, fmt.Errorf("not implemented - copy from logging-operator")
}

func (s *EventsService) generateEventsConfigMapData(ctx context.Context, cluster *clusterv1.Cluster, observabilityBundleVersion semver.Version) (map[string]string, error) {
	// Copy implementation from logging-operator/pkg/resource/events-logger-config/alloy-events-config.go
	// Update to use s.Config.Logging for all settings
	// See logging-operator implementation for details
	return nil, fmt.Errorf("not implemented - copy from logging-operator")
}
```

### Task 2.4: Copy Template Files

**Copy these files from logging-operator to observability-operator:**

```bash
# From logging-operator/pkg/resource/logging-config/alloy/
cp logging-config.alloy.yaml.template observability-operator/pkg/logging/alloy/templates/logs-config.alloy.yaml.template
cp logging.alloy.template observability-operator/pkg/logging/alloy/templates/logs.alloy.template

# From logging-operator/pkg/resource/logging-secret/alloy/
cp alloy-secret.yaml.template observability-operator/pkg/logging/alloy/templates/logs-secret.yaml.template

# From logging-operator/pkg/resource/events-logger-config/alloy/
cp events-logger-config.alloy.yaml.template observability-operator/pkg/logging/alloy/templates/events-config.alloy.yaml.template
cp events-logger.alloy.template observability-operator/pkg/logging/alloy/templates/events.alloy.template

# From logging-operator/pkg/resource/events-logger-secret/ (reuses logs secret template)
# The events secret uses the same template as logs
```

**Important:** When copying template files, no changes are needed - they're pure templates.

### Task 2.5: Copy Helper Functions

**File:** `observability-operator/pkg/logging/alloy/helpers.go`

**Copy these utility functions from logging-operator:**

1. From `pkg/common/common.go`:
   - `ReadLokiIngressURL()` - reads Loki ingress URL
   - `ReadTempoIngressURL()` - reads Tempo ingress URL
   - `GetClusterLabels()` - extracts cluster labels
   - `IsWorkloadCluster()` - determines if cluster is WC

2. From `pkg/common/observability_bundle.go`:
   - `GetObservabilityBundleAppVersion()` - gets bundle version

**Note:** Check if these already exist in observability-operator's `pkg/common/` before copying. If they exist, just import them.

## Phase 3: Integrate into Cluster Controller

### Task 3.1: Update Cluster Controller Setup

**File:** `observability-operator/internal/controller/cluster_controller.go`

**In the `ClusterMonitoringReconciler` struct, add:**

```go
type ClusterMonitoringReconciler struct {
	// ... existing fields ...
	AlloyLogsService   *alloy.LogsService
	AlloyEventsService *alloy.EventsService
}
```

**In `SetupClusterMonitoringReconciler()` function, initialize the services:**

```go
func SetupClusterMonitoringReconciler(mgr manager.Manager, cfg config.Config) error {
	// ... existing code ...
	
	// Initialize logging services
	logsAuthManager := authManagers[auth.AuthTypeLogs]
	tracesAuthManager := authManagers[auth.AuthTypeTraces]
	
	alloyLogsService := alloy.NewLogsService(managerClient, cfg, logsAuthManager)
	alloyEventsService := alloy.NewEventsService(managerClient, cfg, logsAuthManager, tracesAuthManager)

	r := &ClusterMonitoringReconciler{
		// ... existing fields ...
		AlloyLogsService:   alloyLogsService,
		AlloyEventsService: alloyEventsService,
	}

	return r.SetupWithManager(mgr)
}
```

### Task 3.2: Add Logging Reconciliation Logic

**File:** `observability-operator/internal/controller/cluster_controller.go`

**In the `reconcile()` method, after the monitoring reconciliation, add:**

```go
func (r *ClusterMonitoringReconciler) reconcile(ctx context.Context, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	// ... existing monitoring reconciliation ...

	// Get observability bundle version for compatibility checks
	observabilityBundleVersion, err := apps.GetObservabilityBundleVersion(ctx, r.Client, cluster)
	if err != nil {
		logger.Error(err, "failed to get observability bundle version")
		// Continue anyway with a default version
		observabilityBundleVersion = semver.MustParse("0.0.0")
	}

	// Reconcile logging configuration
	if r.Config.Logging.IsLoggingEnabled(cluster) {
		logger.Info("Logging enabled for cluster, configuring Alloy logs and events")
		
		// Configure Alloy logs
		err = r.AlloyLogsService.ReconcileCreate(ctx, cluster, observabilityBundleVersion)
		if err != nil {
			logger.Error(err, "failed to configure alloy logs")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
		
		// Configure Alloy events
		err = r.AlloyEventsService.ReconcileCreate(ctx, cluster, observabilityBundleVersion)
		if err != nil {
			logger.Error(err, "failed to configure alloy events")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
	} else {
		logger.Info("Logging disabled for cluster, removing Alloy logs and events")
		
		// Clean up when logging is disabled
		err = r.AlloyLogsService.ReconcileDelete(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to delete alloy logs config")
			// Don't fail reconciliation on cleanup errors
		}
		
		err = r.AlloyEventsService.ReconcileDelete(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to delete alloy events config")
			// Don't fail reconciliation on cleanup errors
		}
	}

	return ctrl.Result{}, nil
}
```

### Task 3.3: Add RBAC Permissions

**File:** `observability-operator/internal/controller/cluster_controller.go`

**Add these RBAC markers (if not already present):**

```go
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
```

## Phase 4: Update observability-operator Deployment

### Task 4.1: Update Helm Chart Values

**File:** `observability-operator/helm/observability-operator/values.yaml`

**Add logging configuration section:**

```yaml
logging:
  # Enable logging globally
  enabled: false
  
  # Enable node filtering
  enableNodeFiltering: false
  
  # Enable network monitoring
  enableNetworkMonitoring: false
  
  # Default namespaces to collect logs from
  defaultNamespaces: []
  # defaultNamespaces:
  #   - kube-system
  #   - default
  
  # Namespaces to include/exclude for events
  includeEventsFromNamespaces: []
  excludeEventsFromNamespaces: []
```

### Task 4.2: Update Helm Deployment Template

**File:** `observability-operator/helm/observability-operator/templates/deployment.yaml`

**Add logging flags to container args:**

```yaml
args:
  # ... existing args ...
  - --logging-enabled={{ .Values.logging.enabled }}
  {{- if .Values.logging.enableNodeFiltering }}
  - --logging-enable-node-filtering=true
  {{- end }}
  {{- if .Values.logging.enableNetworkMonitoring }}
  - --logging-enable-network-monitoring=true
  {{- end }}
  {{- if .Values.logging.defaultNamespaces }}
  - --logging-default-namespaces={{ .Values.logging.defaultNamespaces | join "," }}
  {{- end }}
  {{- if .Values.logging.includeEventsFromNamespaces }}
  - --logging-include-events-from-namespaces={{ .Values.logging.includeEventsFromNamespaces | join "," }}
  {{- end }}
  {{- if .Values.logging.excludeEventsFromNamespaces }}
  - --logging-exclude-events-from-namespaces={{ .Values.logging.excludeEventsFromNamespaces | join "," }}
  {{- end }}
```

## Phase 5: Deprecate logging-operator

### Task 5.1: Add Deprecation Notice to Main

**File:** `logging-operator/main.go`

**At the start of the main() function, add:**

```go
func main() {
	setupLog.Info("⚠️  ================================================")
	setupLog.Info("⚠️  WARNING: logging-operator is DEPRECATED")
	setupLog.Info("⚠️  ================================================")
	setupLog.Info("⚠️  All functionality has been migrated to:")
	setupLog.Info("⚠️  observability-operator v0.55.0+")
	setupLog.Info("⚠️  ")
	setupLog.Info("⚠️  This operator will be removed in a future release.")
	setupLog.Info("⚠️  Please migrate to observability-operator.")
	setupLog.Info("⚠️  ")
	setupLog.Info("⚠️  Migration guide:")
	setupLog.Info("⚠️  https://github.com/giantswarm/observability-operator/blob/main/docs/logging-migration.md")
	setupLog.Info("⚠️  ================================================")

	// Continue with existing code...
}
```

### Task 5.2: Create Finalizer Cleanup Controller

**File:** `logging-operator/internal/controller/cleanup_controller.go`

**Create new controller:**

```go
package controller

import (
	"context"

	"github.com/pkg/errors"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/logging-operator/pkg/key"
)

// CleanupReconciler removes deprecated logging-operator finalizers
type CleanupReconciler struct {
	Client client.Client
}

func (r *CleanupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := &capi.Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Remove our finalizer if present
	if controllerutil.ContainsFinalizer(cluster, key.Finalizer) {
		logger.Info("Removing deprecated logging-operator finalizer")
		logger.Info("⚠️  This cluster is now managed by observability-operator")
		
		controllerutil.RemoveFinalizer(cluster, key.Finalizer)
		err = r.Client.Update(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		
		logger.Info("Successfully removed logging-operator finalizer")
	}

	return ctrl.Result{}, nil
}

func (r *CleanupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capi.Cluster{}).
		Complete(r)
}
```

### Task 5.3: Update Main to Use Cleanup Controller

**File:** `logging-operator/main.go`

**Replace the existing CapiClusterReconciler setup with CleanupReconciler:**

```go
	// Setup cleanup controller to remove finalizers
	if err = (&controller.CleanupReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create cleanup controller", "controller", "Cleanup")
		os.Exit(1)
	}

	// Comment out or remove the old reconciler
	// if err = (&controller.CapiClusterReconciler{
	// 	...
	// }).SetupWithManager(mgr); err != nil {
	// 	...
	// }
```

### Task 5.4: Update README with Deprecation Notice

**File:** `logging-operator/README.md`

**Replace the entire content with:**

```markdown
# ⚠️ DEPRECATED: logging-operator

> **This operator has been deprecated and is no longer maintained.**
> 
> **All functionality has been migrated to [observability-operator](https://github.com/giantswarm/observability-operator) v0.55.0+**

## Migration Guide

The logging-operator has been fully integrated into the observability-operator. 

### For Users

If you're using logging-operator, you should migrate to observability-operator:

1. **Deploy observability-operator v0.55.0+** with logging enabled:
   ```yaml
   logging:
     enabled: true
     enableNodeFiltering: false  # optional
     enableNetworkMonitoring: false  # optional
     defaultNamespaces: []  # optional
   ```

2. **Label clusters for logging** (same as before):
   ```bash
   kubectl label cluster -n <namespace> <cluster-name> giantswarm.io/logging=true
   ```

3. **Scale down logging-operator**:
   ```bash
   kubectl scale deployment logging-operator -n monitoring --replicas=0
   ```

4. **Verify logging still works** - check that Alloy pods are running and logs are flowing to Loki

5. **Remove logging-operator** (after 1-2 weeks of stable operation):
   ```bash
   kubectl delete deployment logging-operator -n monitoring
   ```

### Configuration Mapping

| logging-operator flag | observability-operator flag |
|----------------------|----------------------------|
| `--enable-logging` | `--logging-enabled` |
| `--default-namespaces` | `--logging-default-namespaces` |
| `--enable-node-filtering` | `--logging-enable-node-filtering` |
| `--enable-tracing` | `--tracing-enabled` (already exists) |
| `--enable-network-monitoring` | `--logging-enable-network-monitoring` |
| `--include-events-from-namespaces` | `--logging-include-events-from-namespaces` |
| `--exclude-events-from-namespaces` | `--logging-exclude-events-from-namespaces` |
| `--installation-name` | `--management-cluster-name` (already exists) |
| `--insecure-ca` | `--management-cluster-insecure-ca` (already exists) |

### What Changed

- **Secret/ConfigMap names remain the same** - no changes needed to Alloy deployments
- **Authentication** is now managed by observability-operator's auth package
- **Labels** now use `observability.giantswarm.io/` prefix (but old resources are compatible)
- **Finalizers** are automatically cleaned up by the deprecation controller

### Resources Created

observability-operator creates the same resources as logging-operator:

- **Secrets:**
  - `<cluster>-alloy-logs-secret` - Loki write credentials
  - `<cluster>-alloy-events-secret` - Events logger credentials

- **ConfigMaps:**
  - `<cluster>-alloy-logs-config` - Logging configuration
  - `<cluster>-alloy-events-config` - Events logger configuration

### Support

For issues with the migration:
- Check [observability-operator documentation](https://github.com/giantswarm/observability-operator)
- Open an issue in [observability-operator](https://github.com/giantswarm/observability-operator/issues)

---

## Archive Notice

This repository is archived and will be removed after all installations have migrated to observability-operator.

**Last active version:** v0.40.1 (December 2025)
**Deprecation date:** December 2025
**Planned removal:** March 2026

---

For historical reference, see the [old README](README.old.md).
```

### Task 5.5: Rename Old README

**Command:**

```bash
cd logging-operator
mv README.md README.old.md
# Then create new README.md with deprecation notice above
```

## Phase 6: Testing

### Task 6.1: Unit Tests

**For observability-operator:**

1. **Create test file:** `observability-operator/pkg/logging/alloy/logs_service_test.go`

```go
package alloy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/observability-operator/pkg/logging/alloy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

func TestLogsServiceCreate(t *testing.T) {
	ctx := context.Background()
	
	// Create fake client
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	
	// Create service
	cfg := config.Config{
		Logging: config.LoggingConfig{
			Enabled: true,
			DefaultNamespaces: []string{"kube-system"},
		},
	}
	
	// Create mock auth manager
	mockAuth := &mockAuthManager{}
	
	service := alloy.NewLogsService(client, cfg, mockAuth)
	
	// Create test cluster
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "org-test",
		},
	}
	
	// Test ReconcileCreate
	err := service.ReconcileCreate(ctx, cluster, semver.MustParse("2.4.0"))
	require.NoError(t, err)
	
	// Verify configmap was created
	configmap := service.logsConfigMap(cluster)
	err = client.Get(ctx, client.ObjectKeyFromObject(configmap), configmap)
	assert.NoError(t, err)
	assert.NotEmpty(t, configmap.Data)
	
	// Verify secret was created
	secret := service.logsSecret(cluster)
	err = client.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	assert.NoError(t, err)
	assert.NotEmpty(t, secret.Data)
}

func TestLogsServiceDelete(t *testing.T) {
	// Similar test for deletion
	// Test that resources are properly cleaned up
}

// Mock auth manager for testing
type mockAuthManager struct{}

func (m *mockAuthManager) GetClusterPassword(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	return "test-password", nil
}

func (m *mockAuthManager) EnsureClusterAuth(ctx context.Context, cluster *clusterv1.Cluster) error {
	return nil
}

func (m *mockAuthManager) DeleteClusterAuth(ctx context.Context, cluster *clusterv1.Cluster) error {
	return nil
}

func (m *mockAuthManager) DeleteGatewaySecrets(ctx context.Context) error {
	return nil
}
```

2. **Run tests:**

```bash
cd observability-operator
go test ./pkg/logging/alloy/...
```

### Task 6.2: Integration Testing

**Create test environment:**

1. **Deploy both operators to test cluster:**

```bash
# Deploy observability-operator with logging enabled
helm upgrade --install observability-operator ./helm/observability-operator \
  --set logging.enabled=true \
  --set logging.defaultNamespaces="{kube-system,default}" \
  --namespace monitoring

# Deploy logging-operator (scaled to 0)
helm upgrade --install logging-operator ./helm/logging-operator \
  --set replicaCount=0 \
  --namespace monitoring
```

2. **Create test cluster:**

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-logging-cluster
  namespace: org-test
  labels:
    giantswarm.io/logging: "true"
spec:
  # ... cluster spec ...
```

3. **Verify resources created:**

```bash
# Check configmaps
kubectl get configmap -n org-test | grep alloy

# Expected output:
# test-logging-cluster-alloy-logs-config
# test-logging-cluster-alloy-events-config

# Check secrets
kubectl get secret -n org-test | grep alloy

# Expected output:
# test-logging-cluster-alloy-logs-secret
# test-logging-cluster-alloy-events-secret
```

4. **Verify Alloy pods can read configs:**

```bash
# If Alloy pods are deployed, check they mount the configs
kubectl get pod -n kube-system -l app=alloy-logs -o yaml | grep -A 10 volumes
```

5. **Test logging disabled:**

```bash
# Remove logging label
kubectl label cluster test-logging-cluster -n org-test giantswarm.io/logging-

# Verify resources are deleted
kubectl get configmap,secret -n org-test | grep alloy
# Should return nothing
```

### Task 6.3: Manual Verification Checklist

**Before deployment:**

- [ ] All unit tests pass
- [ ] observability-operator builds successfully
- [ ] Helm chart renders correctly
- [ ] RBAC permissions include ConfigMap/Secret access

**After deployment (dev/staging):**

- [ ] observability-operator pod starts successfully
- [ ] No errors in operator logs
- [ ] Test cluster gets logging configs created
- [ ] Alloy pods can mount and read configs
- [ ] Logs flow to Loki
- [ ] Events are captured
- [ ] Tracing works (if enabled)
- [ ] Disabling logging removes resources cleanly

**Migration testing:**

- [ ] logging-operator finalizers are removed
- [ ] No duplicate resources created
- [ ] Existing Alloy deployments continue working
- [ ] Can scale down logging-operator without issues

## Phase 7: Deployment and Rollout

### Task 7.1: Version and Release

**observability-operator:**

1. **Update version in Chart.yaml:**
   ```yaml
   version: 0.55.0  # or next appropriate version
   appVersion: 0.55.0
   ```

2. **Update CHANGELOG.md:**
   ```markdown
   ## [0.55.0] - 2025-12-XX
   
   ### Added
   
   - Integrated logging configuration from logging-operator
   - Add Alloy logs and events configuration services
   - Add logging-specific flags: `--logging-enabled`, `--logging-default-namespaces`, etc.
   
   ### Changed
   
   - Cluster controller now handles logging configuration
   - Logging services use observability-operator's auth manager
   
   ### Migration
   
   - logging-operator functionality is now included
   - See migration guide for upgrading from logging-operator
   ```

3. **Create git tag and release:**
   ```bash
   git tag v0.55.0
   git push origin v0.55.0
   ```

**logging-operator:**

1. **Update version to mark as deprecated:**
   ```yaml
   version: 0.40.2-deprecated
   appVersion: 0.40.2-deprecated
   ```

2. **Update CHANGELOG.md:**
   ```markdown
   ## [0.40.2-deprecated] - 2025-12-XX
   
   ### Deprecated
   
   - **This operator is deprecated** and replaced by observability-operator v0.55.0+
   - All functionality has been migrated to observability-operator
   - This version only removes finalizers and shows deprecation warnings
   - See README.md for migration guide
   
   ### Changed
   
   - Replaced reconciliation logic with cleanup controller
   - Added deprecation warnings to startup logs
   ```

### Task 7.2: Deployment Sequence

**Stage 1: Deploy observability-operator (Day 1)**

```bash
# Update observability-operator to v0.55.0
helm upgrade observability-operator giantswarm/observability-operator \
  --version 0.55.0 \
  --set logging.enabled=true \
  --set logging.defaultNamespaces="{kube-system,default}" \
  --namespace monitoring
```

**Stage 2: Monitor (Days 1-3)**

- Check operator logs for errors
- Verify new configs are created for labeled clusters
- Monitor existing logging functionality
- Check metrics/alerts for any issues

**Stage 3: Scale down logging-operator (Day 3)**

```bash
kubectl scale deployment logging-operator -n monitoring --replicas=0
```

**Stage 4: Deploy cleanup version (Day 4)**

```bash
# Deploy deprecated logging-operator version to clean finalizers
helm upgrade logging-operator giantswarm/logging-operator \
  --version 0.40.2-deprecated \
  --set replicaCount=1 \
  --namespace monitoring

# Wait for finalizers to be cleaned up (check cluster resources)
kubectl get clusters -A -o custom-columns=NAME:.metadata.name,FINALIZERS:.metadata.finalizers

# Scale back down
kubectl scale deployment logging-operator -n monitoring --replicas=0
```

**Stage 5: Monitor stability (Days 4-14)**

- Verify logging continues working
- No issues reported
- All clusters have clean finalizers

**Stage 6: Remove logging-operator (Day 14+)**

```bash
helm uninstall logging-operator -n monitoring
```

### Task 7.3: Rollback Plan

**If issues are found:**

1. **Immediate rollback:**
   ```bash
   # Scale up logging-operator
   kubectl scale deployment logging-operator -n monitoring --replicas=1
   
   # Scale down observability-operator (or revert to previous version)
   helm upgrade observability-operator giantswarm/observability-operator \
     --version 0.54.0 \
     --namespace monitoring
   ```

2. **Clean up resources created by observability-operator (if needed):**
   ```bash
   # Remove configs created by new operator
   kubectl delete configmap -l app.kubernetes.io/managed-by=observability-operator \
     -A --field-selector metadata.name~=alloy-logs-config
   
   kubectl delete secret -l app.kubernetes.io/managed-by=observability-operator \
     -A --field-selector metadata.name~=alloy-logs-secret
   ```

3. **Investigate issues and prepare fix**

## Phase 8: Documentation

### Task 8.1: Create Migration Guide

**File:** `observability-operator/docs/logging-migration.md`

```markdown
# Migration Guide: logging-operator to observability-operator

This guide explains how to migrate from the deprecated logging-operator to observability-operator v0.55.0+.

## Overview

The logging-operator has been fully integrated into observability-operator. All functionality remains the same, but configuration is now consolidated into a single operator.

## Prerequisites

- observability-operator v0.55.0 or later
- Access to Kubernetes cluster
- Helm 3.x

## Migration Steps

### 1. Deploy observability-operator with Logging Enabled

Update your observability-operator deployment:

\`\`\`yaml
# values.yaml
logging:
  enabled: true
  enableNodeFiltering: false  # Match your previous setting
  enableNetworkMonitoring: false  # Match your previous setting
  defaultNamespaces:
    - kube-system
    - default
\`\`\`

Deploy:

\`\`\`bash
helm upgrade observability-operator giantswarm/observability-operator \
  --version 0.55.0 \
  --values values.yaml \
  --namespace monitoring
\`\`\`

### 2. Verify Logging Configuration

Check that configs are created for labeled clusters:

\`\`\`bash
# List clusters with logging enabled
kubectl get clusters -A -l giantswarm.io/logging=true

# Check that configs exist for each cluster
kubectl get configmap,secret -n <cluster-namespace> | grep alloy
\`\`\`

### 3. Scale Down logging-operator

Once verified, scale down the old operator:

\`\`\`bash
kubectl scale deployment logging-operator -n monitoring --replicas=0
\`\`\`

### 4. Monitor for Issues

Watch for any problems for 24-48 hours:

\`\`\`bash
# Check operator logs
kubectl logs -n monitoring deployment/observability-operator -f

# Verify logs are flowing to Loki
# (use your Grafana/Loki interface)
\`\`\`

### 5. Remove logging-operator

After confirming everything works:

\`\`\`bash
helm uninstall logging-operator -n monitoring
\`\`\`

## Configuration Reference

### Flag Mapping

| logging-operator | observability-operator |
|-----------------|------------------------|
| `--enable-logging` | `--logging-enabled` |
| `--default-namespaces` | `--logging-default-namespaces` |
| `--enable-node-filtering` | `--logging-enable-node-filtering` |
| `--enable-network-monitoring` | `--logging-enable-network-monitoring` |
| `--include-events-from-namespaces` | `--logging-include-events-from-namespaces` |
| `--exclude-events-from-namespaces` | `--logging-exclude-events-from-namespaces` |

### Resource Names

No changes - resources keep the same names:

- Secrets: `<cluster>-alloy-logs-secret`, `<cluster>-alloy-events-secret`
- ConfigMaps: `<cluster>-alloy-logs-config`, `<cluster>-alloy-events-config`

## Troubleshooting

### Configs Not Created

**Issue:** observability-operator doesn't create logging configs

**Solutions:**

1. Check logging is enabled: `--logging-enabled=true`
2. Verify cluster has label: `kubectl get cluster <name> -o yaml | grep logging`
3. Check operator logs for errors

### Duplicate Resources

**Issue:** Both operators creating resources

**Solution:** Scale down logging-operator: `kubectl scale deployment logging-operator -n monitoring --replicas=0`

### Alloy Pods Can't Read Configs

**Issue:** Alloy pods report missing configs

**Solutions:**

1. Verify configmap/secret names match exactly
2. Check namespace is correct
3. Restart Alloy pods: `kubectl rollout restart daemonset/alloy-logs -n kube-system`

### Finalizer Issues

**Issue:** Clusters have old logging-operator finalizers

**Solution:** Deploy cleanup version of logging-operator:

\`\`\`bash
helm upgrade logging-operator giantswarm/logging-operator \
  --version 0.40.2-deprecated \
  --set replicaCount=1 \
  --namespace monitoring
\`\`\`

Wait for finalizers to be cleaned, then scale down.

## Rollback

If you need to rollback:

\`\`\`bash
# Scale up old operator
kubectl scale deployment logging-operator -n monitoring --replicas=1

# Revert observability-operator
helm upgrade observability-operator giantswarm/observability-operator \
  --version 0.54.0 \
  --namespace monitoring
\`\`\`

## Support

For issues, contact:
- Team Atlas
- #team-atlas Slack channel
- Open issue in observability-operator repository
```

### Task 8.2: Update observability-operator README

**File:** `observability-operator/README.md`

Add section about logging:

```markdown
### Logging Configuration

observability-operator manages Alloy configuration for log collection and Kubernetes events.

#### Enable Logging

\`\`\`yaml
logging:
  enabled: true
  defaultNamespaces:
    - kube-system
    - default
\`\`\`

#### Per-Cluster Control

Label clusters to enable/disable logging:

\`\`\`bash
# Enable logging for a cluster
kubectl label cluster my-cluster -n org-example giantswarm.io/logging=true

# Disable logging
kubectl label cluster my-cluster -n org-example giantswarm.io/logging=false
\`\`\`

#### Resources Created

For each cluster with logging enabled:

- `<cluster>-alloy-logs-config` - ConfigMap with Alloy logging configuration
- `<cluster>-alloy-logs-secret` - Secret with Loki credentials
- `<cluster>-alloy-events-config` - ConfigMap with events logger configuration
- `<cluster>-alloy-events-secret` - Secret with events logger credentials

See [Logging Migration Guide](docs/logging-migration.md) for details.
```

## Summary Checklist

Use this checklist to track progress:

### observability-operator Changes

- [ ] Phase 1: Add logging configuration flags
- [ ] Phase 1: Update LoggingConfig struct
- [ ] Phase 2: Create logging/alloy package
- [ ] Phase 2: Implement LogsService
- [ ] Phase 2: Implement EventsService
- [ ] Phase 2: Copy template files
- [ ] Phase 2: Copy helper functions
- [ ] Phase 3: Update cluster controller struct
- [ ] Phase 3: Initialize logging services
- [ ] Phase 3: Add reconciliation logic
- [ ] Phase 3: Add RBAC permissions
- [ ] Phase 4: Update Helm values
- [ ] Phase 4: Update deployment template
- [ ] Phase 6: Write unit tests
- [ ] Phase 6: Run integration tests
- [ ] Phase 7: Update version and changelog
- [ ] Phase 8: Create migration guide
- [ ] Phase 8: Update README

### logging-operator Changes

- [ ] Phase 5: Add deprecation notice to main
- [ ] Phase 5: Create cleanup controller
- [ ] Phase 5: Update main to use cleanup controller
- [ ] Phase 5: Update README with deprecation
- [ ] Phase 5: Rename old README
- [ ] Phase 7: Update version to deprecated
- [ ] Phase 7: Update changelog

### Deployment

- [ ] Phase 7: Deploy observability-operator v0.55.0
- [ ] Phase 7: Monitor for issues (3 days)
- [ ] Phase 7: Scale down logging-operator
- [ ] Phase 7: Deploy cleanup version
- [ ] Phase 7: Monitor stability (2 weeks)
- [ ] Phase 7: Remove logging-operator

### Documentation

- [ ] Phase 8: Migration guide complete
- [ ] Phase 8: README updates complete
- [ ] Phase 8: Team notified
- [ ] Phase 8: Slack announcement sent

## Timeline Estimate

| Phase | Tasks | Duration |
|-------|-------|----------|
| 1. Configuration | 1.1-1.2 | 4 hours |
| 2. Services | 2.1-2.5 | 2 days |
| 3. Integration | 3.1-3.3 | 1 day |
| 4. Deployment Config | 4.1-4.2 | 2 hours |
| 5. Deprecation | 5.1-5.5 | 4 hours |
| 6. Testing | 6.1-6.3 | 1 day |
| 7. Rollout | 7.1-7.3 | 2 weeks |
| 8. Documentation | 8.1-8.2 | 4 hours |
| **Total** | | **~3 weeks** |

**Development:** ~1 week
**Testing:** ~1 week
**Rollout & Monitoring:** ~2 weeks (overlaps with monitoring period)

## Success Criteria

Migration is successful when:

1. ✅ observability-operator creates logging configs for labeled clusters
2. ✅ Alloy pods can read and use the configurations
3. ✅ Logs flow to Loki without interruption
4. ✅ Events are captured correctly
5. ✅ Tracing works (if enabled)
6. ✅ No errors in operator logs
7. ✅ logging-operator finalizers are removed
8. ✅ logging-operator can be safely removed
9. ✅ All documentation is updated
10. ✅ Team is trained on new configuration

## Support and Escalation

For issues during migration:

1. Check logs: `kubectl logs -n monitoring deployment/observability-operator`
2. Verify config: Check Helm values and flags
3. Test on dev environment first
4. Contact Team Atlas for support
5. Have rollback plan ready

## References

- [observability-operator repo](https://github.com/giantswarm/observability-operator)
- [logging-operator repo](https://github.com/giantswarm/logging-operator)
- [Alloy documentation](https://grafana.com/docs/alloy/)
- [CAPI documentation](https://cluster-api.sigs.k8s.io/)
