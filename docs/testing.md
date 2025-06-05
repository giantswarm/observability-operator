# Testing Guide

This document explains how to run tests for the observability-operator using our comprehensive Ginkgo-based testing infrastructure.

## Overview

The project uses a modern testing stack with automatic tool management and parallel execution:

- **Testing Framework**: [Ginkgo v2.23.4](https://onsi.github.io/ginkgo/) with BDD-style testing and parallel execution
- **Unit Tests**: Pure Go logic tests (`./pkg/...`, `./internal/predicates`) - no Kubernetes dependencies
- **Integration Tests**: Controller and webhook tests using [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) for Kubernetes API simulation
- **Tool Management**: Automatic installation and version management of all testing tools
- **Parallel Execution**: Tests run in parallel with configurable node count for optimal performance
- **Coverage Reporting**: Comprehensive coverage analysis with HTML reports

## Quick Start

### Zero-Setup Testing

```bash
# Run all tests - tools auto-install if needed
make test

# Run specific test suites
go test ./pkg/...              # Unit tests
go test ./internal/controller  # Integration tests
```

### Prerequisites

All testing tools are automatically managed via Make targets:

```bash
# Install all development tools (including Ginkgo)
make install-tools

# Or install specific tools
make ginkgo        # Ginkgo testing framework v2.23.4
make envtest       # Kubernetes test environment
make controller-gen # Code generation tool
```

## Testing Workflow

### Main Test Command

The primary test command runs the full test suite with optimal configuration:

```bash
make test
```

**What it does:**
- Runs Ginkgo with parallel execution (4 nodes)
- Randomizes test and suite execution order
- Enables coverage reporting
- Automatically sets up envtest environment
- Uses Kubernetes version auto-detected from dependencies

**Ginkgo Configuration:**
- **Parallel**: `--nodes 4` for faster execution
- **Randomization**: `--randomize-all --randomize-suites` for robust testing
- **Coverage**: `--cover` for comprehensive coverage analysis
- **Environment**: Auto-configured `KUBEBUILDER_ASSETS` for Kubernetes testing

### Development Testing

```bash
# Quick unit tests (fastest)
go test ./pkg/...
go test ./internal/predicates

# Integration tests with controllers
go test ./internal/controller

# Golden file generation for test fixtures
make generate-golden-files

# Coverage with HTML report
make test
make coverage-html
```

## Available Make Targets

### Testing Targets

```bash
make test                    # Run all tests with Ginkgo and envtest (MAIN TARGET)
make generate-golden-files   # Generate golden files for tests
make coverage-html          # Generate HTML coverage report
```

### Tool Management

```bash
make install-tools          # Install all development tools
make ginkgo                # Install Ginkgo testing framework v2.23.4
make envtest               # Install setup-envtest tool
make setup-envtest         # Setup envtest binaries for Kubernetes testing
make controller-gen        # Install controller-gen for code generation
make clean-tools           # Clean all downloaded tools
```

### Code Generation (Required for Testing)

```bash
make generate              # Generate all code and manifests
make generate-all          # Generate everything (code + manifests)
make generate-deepcopy     # Generate deepcopy methods for API types
make generate-crds         # Generate Custom Resource Definitions
```

### Validation and Quality

```bash
make fmt                   # Format Go code
make vet                   # Run go vet
make lint                  # Run golangci-lint
make validate-crds         # Validate generated CRDs
make verify-generate       # Verify generated files are up to date
```

## Local Development & IDE Integration

### IDE Setup (VS Code, GoLand, etc.)

The testing infrastructure works seamlessly with IDEs:

```bash
# One-time setup for your development environment
make install-tools
make setup-envtest

# Now all tests work in your IDE - no additional configuration needed!
```

**What this provides:**
- ✅ Ginkgo test discovery and execution
- ✅ Kubernetes API server simulation via envtest
- ✅ Full integration test support
- ✅ Coverage analysis
- ✅ Debugging support

### Custom Ginkgo Options

```bash
# Run tests with custom Ginkgo flags
./bin/ginkgo -v --focus="specific test" ./internal/controller

# Parallel execution with custom node count
./bin/ginkgo --nodes=8 ./...

# Debug mode (serial execution)
./bin/ginkgo --nodes=1 -v ./internal/controller

# Specific test suites
./bin/ginkgo ./pkg/...                    # Unit tests only
./bin/ginkgo ./internal/controller        # Controller tests
```

### Performance Optimization

```bash
# Adjust parallel execution based on your system
GOMAXPROCS=8 make test               # Utilize more CPU cores
./bin/ginkgo --nodes=8 ./...         # More parallel test nodes

# For resource-constrained environments
./bin/ginkgo --nodes=1 ./...         # Serial execution
```

## Test Configuration

### Tool Versions

The project uses specific, tested versions of all tools:

- **Ginkgo**: v2.23.4 (BDD testing framework)
- **Controller-gen**: v0.17.2 (Code generation)
- **Kustomize**: v5.6.0 (Kubernetes configuration)
- **Setup-envtest**: Auto-detected from controller-runtime version
- **Kubernetes**: Auto-detected from k8s.io/api dependency (currently 1.33)

### Environment Variables

```bash
# Override Kubernetes version for testing
export ENVTEST_K8S_VERSION=1.31
make test

# Manual envtest assets path (rarely needed)
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/k8s/1.33.0-linux-amd64"
go test ./internal/controller

# Control Ginkgo behavior
export GINKGO_NODES=8                # Parallel execution
export GINKGO_RANDOMIZE=false       # Disable randomization
```

### Test Categories

**Unit Tests:**
- Location: `./pkg/...`, `./internal/predicates`
- Dependencies: None (pure Go)
- Execution: Fast, no external dependencies
- Coverage: Business logic and utilities

**Integration Tests:**
- Location: `./internal/controller`, `./internal/webhook/...`
- Dependencies: Kubernetes API server (via envtest)
- Execution: Moderate speed, requires envtest setup
- Coverage: Controller reconciliation, webhook validation, API interactions

## Writing Tests

### Ginkgo Test Structure

Ginkgo tests use a BDD (Behavior-Driven Development) style with descriptive test organization:

```go
package controller_test

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    // ... other imports
)

var _ = Describe("GrafanaOrganization Controller", func() {
    Context("When reconciling a GrafanaOrganization", func() {
        It("Should create the organization successfully", func() {
            // Arrange
            org := &observabilityv1alpha1.GrafanaOrganization{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-org",
                    Namespace: "default",
                },
                Spec: observabilityv1alpha1.GrafanaOrganizationSpec{
                    DisplayName: "Test Organization",
                },
            }

            // Act
            Expect(k8sClient.Create(ctx, org)).To(Succeed())

            // Assert
            Eventually(func() error {
                // Check reconciliation results
                return k8sClient.Get(ctx, client.ObjectKeyFromObject(org), org)
            }).Should(Succeed())
        })

        It("Should handle organization updates", func() {
            // Test update scenarios
        })
    })

    Context("When organization is deleted", func() {
        It("Should cleanup resources properly", func() {
            // Test cleanup logic
        })
    })
})
```

### Test Suite Setup

Controller tests have access to these pre-configured globals:

```go
var (
    cfg       *rest.Config
    k8sClient client.Client
    testEnv   *envtest.Environment
    ctx       context.Context
    cancel    context.CancelFunc
)
```

### Unit Test Examples

```go
func TestUtilityFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "valid input",
            input:    "test",
            expected: "processed-test",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ProcessInput(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Golden File Testing

Generate and update test fixtures:

```bash
# Update golden files when expected output changes
make generate-golden-files

# This runs: UPDATE_GOLDEN_FILES=true go test -v ./...
```

## Coverage Reporting

### Generate Coverage Reports

```bash
# Run tests and generate coverage
make test                    # Creates coverprofile.out

# Generate HTML coverage report
make coverage-html           # Opens in browser
```

### Coverage Analysis

The test suite provides comprehensive coverage analysis:

- **Package Coverage**: Individual package coverage metrics
- **Function Coverage**: Line-by-line function coverage
- **Integration Coverage**: Controller and webhook interaction coverage
- **Composite Coverage**: Combined view of all test coverage

**Coverage Files:**
- `coverprofile.out` - Main coverage profile from `make test`
- HTML report available via `make coverage-html`

### Coverage Thresholds

Current coverage metrics (as of latest run):
- **Statement Coverage**: 10.3%
- **Composite Coverage**: 19.7%

## Troubleshooting

### Common Issues

#### "Tools not found" or Installation Issues

```bash
# Clean and reinstall all tools
make clean-tools
make install-tools

# Verify tool installation
ls -la bin/
./bin/ginkgo version
./bin/controller-gen --version
```

#### "KUBEBUILDER_ASSETS not set"

```bash
# Setup envtest binaries
make setup-envtest

# Verify setup
ls -la bin/k8s/k8s/
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/k8s/$(ls bin/k8s/k8s/)"
echo $KUBEBUILDER_ASSETS
```

#### "Generated files out of date"

```bash
# Regenerate all code and manifests
make clean-generated
make generate-all

# Verify no uncommitted changes
git status
```

#### Test Performance Issues

```bash
# Reduce parallel execution
./bin/ginkgo --nodes=1 ./...

# Test specific packages
./bin/ginkgo ./pkg/...                    # Unit tests only
./bin/ginkgo ./internal/controller        # Controllers only

# Check system resources
make test > test-output.log 2>&1          # Capture full output
```

#### Missing Dependencies

```bash
# Ensure Go dependencies are current
go mod tidy
go mod download

# Verify project structure
make validate-crds
make verify-generate
```

## Testing Infrastructure

### Tool Management

The project uses a sophisticated tool management system:

```bash
# All tools are versioned and cached
bin/
├── ginkgo -> ginkgo-v2.23.4                    # Symlink to versioned binary
├── ginkgo-v2.23.4                              # Versioned Ginkgo installation
├── controller-gen -> controller-gen-v0.17.2    # Symlink to versioned binary  
├── controller-gen-v0.17.2                      # Versioned controller-gen
├── kustomize -> kustomize-v5.6.0               # Symlink to versioned binary
├── kustomize-v5.6.0                            # Versioned kustomize
└── k8s/k8s/1.33.0-linux-amd64/                # Kubernetes test binaries
    ├── kube-apiserver
    ├── etcd
    └── kubectl
```

**Benefits:**
- ✅ Reproducible builds across environments  
- ✅ Version pinning prevents tool drift
- ✅ Easy tool updates and rollbacks
- ✅ Parallel development with different tool versions

### Auto-Detection Features

The test infrastructure automatically:

1. **Detects Kubernetes versions** from `go.mod` dependencies
2. **Locates envtest binaries** in standard locations
3. **Configures environment variables** for test execution
4. **Skips gracefully** when dependencies are unavailable
5. **Provides helpful error messages** for setup

### Continuous Integration

For CI/CD environments:

```bash
# Complete CI testing workflow
make install-tools     # Install all required tools
make generate-all      # Generate code and manifests  
make verify-generate   # Verify generated files are current
make test             # Run comprehensive test suite
make coverage-html    # Generate coverage reports
```

## Best Practices

### Test Organization

- **Unit Tests**: Place in same package as code under test
- **Integration Tests**: Use `internal/controller/` for controller tests
- **Test Helpers**: Create reusable test utilities in `internal/testutil/`
- **Golden Files**: Store expected outputs in `testdata/` directories

### Performance Guidelines

- **Parallel Testing**: Use Ginkgo's parallel execution for faster feedback
- **Resource Cleanup**: Always clean up test resources in `AfterEach` blocks
- **Test Isolation**: Ensure tests don't depend on each other's state
- **Timeout Handling**: Use `Eventually` and `Consistently` for async operations

### Development Workflow

1. **Write Tests First**: Use TDD approach with Ginkgo's descriptive structure
2. **Run Frequently**: Use `make test` during development
3. **Check Coverage**: Monitor coverage with `make coverage-html`
4. **Validate Generation**: Run `make verify-generate` before commits
5. **Clean State**: Use `make clean-generated` when changing APIs

## Additional Resources

- [Ginkgo Documentation](https://onsi.github.io/ginkgo/)
- [Gomega Matcher Reference](https://onsi.github.io/gomega/)
- [Controller Runtime Testing](https://book.kubebuilder.io/reference/testing.html)
- [Envtest Documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest)
