#!/bin/bash
set -euo pipefail

YQ="yq"

echo "=== Alertmanager Configuration Validation Script ==="
echo "Starting validation process..."

TMP_DIR="$(mktemp -d -t validate-alertmanager-config.XXXXXX)"
trap 'echo "Cleaning up temporary directory: $TMP_DIR"; rm -rf "$TMP_DIR"' EXIT

TARGET_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$TARGET_DIR/../.." && pwd -P)"

echo "Script directory: $TARGET_DIR"
echo "Project root: $PROJECT_ROOT"
echo "Temporary directory: $TMP_DIR"

# Extract the exact commit hash from go.mod replacement directive
echo "=== Extracting Alertmanager version from go.mod ==="
echo "Searching for prometheus/alertmanager replacement in: $PROJECT_ROOT/go.mod"

ALERTMANAGER_VERSION_LINE=$(grep "github.com/prometheus/alertmanager =>" "$PROJECT_ROOT/go.mod" || true)
if [ -z "$ALERTMANAGER_VERSION_LINE" ]; then
  echo "Error: Could not find prometheus/alertmanager replacement in go.mod"
  echo "Expected format: github.com/prometheus/alertmanager => github.com/grafana/prometheus-alertmanager v0.25.1-0.20250305143719-fa9fa7096626"
  exit 1
fi

echo "✓ Found replacement line: $ALERTMANAGER_VERSION_LINE"

# Extract commit hash from version string like v0.25.1-0.20250305143719-fa9fa7096626
ALERTMANAGER_COMMIT=$(echo "$ALERTMANAGER_VERSION_LINE" | sed -n 's/.*-\([a-f0-9]\{12\}\)$/\1/p')
if [ -z "$ALERTMANAGER_COMMIT" ]; then
  echo "Error: Could not extract commit hash from go.mod version: $ALERTMANAGER_VERSION_LINE"
  echo "Expected format: v0.25.1-0.20250305143719-fa9fa7096626 (with 12-character commit hash at the end)"
  exit 1
fi

echo "✓ Extracted commit hash: $ALERTMANAGER_COMMIT"
echo "This will be used to build amtool from the Grafana fork"
# Prepare amtool from Grafana fork
echo ""
echo "=== Preparing amtool from Grafana fork ==="
echo "Setting up amtool to run from commit $ALERTMANAGER_COMMIT"

# Clone the Grafana fork and checkout the specific commit
REPO_DIR="$TMP_DIR/prometheus-alertmanager"
echo "Cloning Grafana prometheus-alertmanager fork..."
echo "Repository: https://github.com/grafana/prometheus-alertmanager.git"
echo "Target directory: $REPO_DIR"

git clone https://github.com/grafana/prometheus-alertmanager.git "$REPO_DIR"
cd "$REPO_DIR"

echo "Checking out commit: $ALERTMANAGER_COMMIT"
git -c advice.detachedHead=false checkout "$ALERTMANAGER_COMMIT"

# Show some info about the current state
echo "✓ Repository ready for go run"
echo "Current commit info:"
echo "  Commit: $(git rev-parse HEAD)"
echo "  Short commit: $(git rev-parse --short HEAD)"
echo "Go environment:"
echo "  Go version: $(go version)"
echo "  Module path: $(pwd)"

# Verify go.mod exists and is valid
if [ ! -f "go.mod" ]; then
  echo "Error: go.mod not found in repository"
  exit 1
fi

echo "✓ Go module ready: $(head -n1 go.mod)"

# Template the helm chart
echo ""
echo "=== Rendering Helm Chart ==="
echo "Chart path: $PROJECT_ROOT/helm/observability-operator"
echo "Building helm dependencies..."
helm dependency build "$PROJECT_ROOT/helm/observability-operator"

echo "Templating helm chart with test values..."
echo "Namespace: alertmanager"
echo "Setting monitoring.opsgenieApiKey to placeholder value"
RENDERED_FILE="$TMP_DIR/rendered.yaml"
helm template observability-operator "$PROJECT_ROOT/helm/observability-operator" --namespace alertmanager --set monitoring.opsgenieApiKey="apikey" --set observability.pagerdutyToken="token" > "$RENDERED_FILE"

echo "✓ Helm chart rendered successfully"
echo "Output file: $RENDERED_FILE"
echo "File size: $(wc -l < "$RENDERED_FILE") lines"

# Extract the secret that contains the Alertmanager configuration
echo ""
echo "=== Extracting Alertmanager Configuration ==="
echo "Searching for secrets with label: observability.giantswarm.io/kind=alertmanager-config"

# This assumes that the secret's labels include observability.giantswarm.io/kind: alertmanager-config
SECRET_NAME="$($YQ eval 'select(.metadata.labels."observability.giantswarm.io/kind" == "alertmanager-config") | .metadata.name' "$RENDERED_FILE" | head -n1)"
if [ -z "$SECRET_NAME" ]; then
  echo "Error: Alertmanager secret not found in rendered templates."
  echo "Searched for secrets with label 'observability.giantswarm.io/kind: alertmanager-config'"
  echo ""
  echo "Available secrets in rendered template:"
  $YQ eval 'select(.kind == "Secret") | .metadata.name' "$RENDERED_FILE" | head -10
  exit 1
fi

echo "✓ Found Alertmanager secret: $SECRET_NAME"

# Assuming the alertmanager config is stored under the key "alertmanager.yaml"
echo "Extracting configuration from secret key: alertmanager.yaml"
CONFIG_B64="$($YQ eval 'select(.metadata.name == "'"$SECRET_NAME"'") | .data."alertmanager.yaml"' "$RENDERED_FILE" | head -n1)"
if [ -z "$CONFIG_B64" ]; then
  echo "Error: No alertmanager.yaml key found in secret $SECRET_NAME."
  echo ""
  echo "Available keys in secret $SECRET_NAME:"
  $YQ eval 'select(.metadata.name == "'"$SECRET_NAME"'") | .data | keys' "$RENDERED_FILE"
  exit 1
fi

echo "✓ Found alertmanager.yaml configuration data"
echo "Configuration size: $(echo "$CONFIG_B64" | wc -c) base64 characters"

# Decode the configuration
echo ""
echo "=== Decoding Configuration ==="
CONFIG_FILE="$TMP_DIR/alertmanager.yaml"
echo "Decoding base64 configuration..."
echo "$CONFIG_B64" | base64 -d > "$CONFIG_FILE"

echo "✓ Configuration decoded successfully"
echo "Configuration file: $CONFIG_FILE"
echo "Configuration size: $(wc -l < "$CONFIG_FILE") lines, $(wc -c < "$CONFIG_FILE") bytes"

echo ""
echo "Configuration preview (first 10 lines):"
head -n 10 "$CONFIG_FILE" || echo "Could not preview configuration file"

# Validate the configuration using amtool
echo ""
echo "=== Validating Configuration ==="
echo "Using go run to execute amtool from Grafana fork"
echo "Repository: $REPO_DIR"
echo "Configuration file: $CONFIG_FILE"
echo "Running: go run ./cmd/amtool check-config"

# Change to the repository directory and run amtool via go run
cd "$REPO_DIR"
go run ./cmd/amtool check-config "$CONFIG_FILE"

echo ""
echo "✓ SUCCESS: Alertmanager configuration is valid!"
echo "✓ The configuration uses the same validation logic as our webhook"
echo "✓ This configuration should work correctly with Mimir's Alertmanager"
echo "✓ Validation performed using commit $ALERTMANAGER_COMMIT from Grafana fork"
