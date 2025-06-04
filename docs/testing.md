# Testing Guide

This document explains how to run tests for the observability-operator.

## Overview

The project uses a comprehensive testing infrastructure with automatic envtest binary detection:

- **Unit Tests**: Pure Go logic tests (`./pkg/...`, `./internal/predicates`) - no Kubernetes dependencies
- **Integration Tests**: Controller and webhook tests using [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) for Kubernetes API simulation
- **Auto-Detection**: Tests automatically find envtest binaries and skip gracefully when unavailable
- **Ginkgo**: BDD-style testing framework with parallel execution and detailed reporting

## Prerequisites

### Install Test Dependencies

The Makefile automatically downloads required tools when needed:

```bash
# Download ginkgo testing framework
make ginkgo

# Download envtest setup tool  
make envtest

# Setup Kubernetes test binaries (one-time setup)
make setup-envtest
```

**Note**: With the new auto-detection features, you can often run tests directly without manual setup!

## Running Tests

### Quick Development Workflow

```bash
# Zero-setup testing - works immediately 
go test ./internal/controller  # Auto-detects envtest binaries
go test ./pkg/...             # Unit tests only

# Or use Make targets
make test-unit                # Fast unit tests only
make test-integration         # Full integration tests (auto-setup)
```

### Comprehensive Testing

```bash
# Run all tests with merged coverage
make test-all
```

Includes:
- Unit tests (`./pkg/...`, `./internal/predicates`)
- Integration tests (`./internal/controller`, `./internal/webhook/...`)
- Merged coverage report in `coverprofile.out`

### Granular Testing Options

```bash
# Unit tests only - no Kubernetes dependencies
make test-unit

# Integration tests only - envtest-based controllers & webhooks  
make test-integration

# Specific integration test categories
make test-controllers         # Controller tests only
make test-webhooks           # Webhook tests only
```

## Local Development

### Zero-Setup Testing

The test infrastructure now provides seamless local development:

```bash
# Works immediately - no setup required!
go test -v ./internal/controller
go test -v ./internal/webhook/...
go test -v ./pkg/...

# Tests automatically:
# 1. Search for envtest binaries in standard locations
# 2. Skip gracefully if binaries unavailable  
# 3. Provide helpful setup messages
```

### IDE Integration (VS Code, GoLand, etc.)

**Just works!** The test suites automatically:
- ✅ Detect envtest binaries in `bin/k8s/` directories
- ✅ Skip gracefully when dependencies missing
- ✅ Provide clear error messages for setup

**Optional one-time setup for full integration testing:**
```bash
make setup-envtest
# Now all tests work in your IDE
```

### Manual Control (Advanced)

```bash
# Explicit setup with specific Kubernetes version
make setup-envtest
ENVTEST_K8S_VERSION=1.33.0 make setup-envtest

# Manual environment override (rarely needed)
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/k8s/1.33.0-linux-amd64"
go test -v ./internal/controller
```

## Test Configuration

### Kubernetes Version

Tests use Kubernetes version `1.33.0` by default. Override with:

```bash
ENVTEST_K8S_VERSION=1.31.0 make setup-envtest
ENVTEST_K8S_VERSION=1.31.0 make test-integration
```

### Test Categories & Behavior

**Unit Tests** (`make test-unit`):
- Packages: `./pkg/...`, `./internal/predicates`
- No Kubernetes dependencies
- Fast execution, perfect for TDD

**Integration Tests** (`make test-integration`):  
- Packages: `./internal/controller`, `./internal/webhook/...`
- Real Kubernetes API server via envtest
- Auto-setup of envtest binaries

**Auto-Detection Logic**:
1. Check `KUBEBUILDER_ASSETS` environment variable
2. Search `bin/k8s/` and `bin/k8s/k8s/` directories
3. Validate required binaries exist (`kube-apiserver`, `etcd`, `kubectl`)
4. Skip gracefully with helpful message if unavailable

## Writing Tests

### Controller Tests

Controller tests should be placed in `internal/controller/` and use the following pattern:

```go
var _ = Describe("My Controller", func() {
    Context("When reconciling a resource", func() {
        It("should do something", func() {
            // Use k8sClient to interact with the test Kubernetes API
            // Available globals: ctx, k8sClient, cfg, testEnv
        })
    })
})
```

### Unit Tests

Standard Go unit tests can be placed anywhere and follow normal Go testing conventions:

```go
func TestMyFunction(t *testing.T) {
    // Standard Go test
}
```

## Troubleshooting

### "KUBEBUILDER_ASSETS not set and envtest binaries not found"

This informational message means tests are skipping integration tests gracefully.

**For full testing capabilities:**
```bash
make setup-envtest
# Now run: go test ./internal/controller (works automatically)
```

### Tests Skip or Don't Run

**Check auto-detection:**
```bash
# Verify binary locations
ls -la bin/k8s/
find bin/k8s -name "kube-apiserver"

# Expected structure:
# bin/k8s/k8s/1.33.0-linux-amd64/kube-apiserver
# bin/k8s/k8s/1.33.0-linux-amd64/etcd  
# bin/k8s/k8s/1.33.0-linux-amd64/kubectl
```

**Manual override if needed:**
```bash
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/k8s/1.33.0-linux-amd64"
go test -v ./internal/controller
```

### Performance Issues

```bash
# Reduce parallel execution
GINKGO_NODES=1 make test-integration

# Test specific components
make test-controllers  # Controllers only
make test-webhooks     # Webhooks only

# Check system resources
make test-unit         # Fastest option
```

### Coverage Issues

```bash
# Generate merged coverage report
make test-all
make coverage-html

# Check individual coverage
make test-unit         # Creates coverage-unit.out
make test-integration  # Creates coverage-integration.out
```

## Coverage

Generate comprehensive HTML coverage reports:

```bash
# All tests with merged coverage
make test-all
make coverage-html

# Individual coverage profiles
make test-unit         # → coverage-unit.out  
make test-integration  # → coverage-integration.out
make test-all          # → coverprofile.out (merged)
```

Coverage includes:
- **Unit coverage**: `./pkg/...`, `./internal/predicates`  
- **Integration coverage**: `./internal/controller`, `./internal/webhook/...`
- **Merged reporting**: Combined view of all test coverage
