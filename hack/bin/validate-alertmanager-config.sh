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

# Resolve the Alertmanager dependency our webhook compiles against, so that
# amtool validates with exactly the same config parser.
# shellcheck source=hack/bin/alertmanager-dependency.sh
source "$TARGET_DIR/alertmanager-dependency.sh"

echo "=== Resolving Alertmanager dependency from go.mod ==="
echo "Reading: $PROJECT_ROOT/go.mod"

AMTOOL_PACKAGE="$(alertmanager_amtool_package "$PROJECT_ROOT/go.mod")"

echo "✓ amtool package: $AMTOOL_PACKAGE"
echo "This will be used to build amtool matching our webhook's Alertmanager version"
echo "Go environment:"
echo "  Go version: $(go version)"

# Template the helm chart
echo ""
echo "=== Rendering Helm Chart ==="
echo "Chart path: $PROJECT_ROOT/helm/observability-operator"
echo "Building helm dependencies..."
helm dependency build "$PROJECT_ROOT/helm/observability-operator"

echo "Templating helm chart with test values..."
echo "Namespace: alertmanager"
RENDERED_FILE="$TMP_DIR/rendered.yaml"
helm template observability-operator "$PROJECT_ROOT/helm/observability-operator" --namespace alertmanager --set alerting.teams[0].name="testteam" --set alerting.teams[0].pagerdutyToken="dummytoken" > "$RENDERED_FILE"

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
echo "Using go run to execute amtool at the version pinned in go.mod"
echo "amtool package: $AMTOOL_PACKAGE"
echo "Configuration file: $CONFIG_FILE"

go run "$AMTOOL_PACKAGE" check-config "$CONFIG_FILE"

echo ""
echo "✓ SUCCESS: Alertmanager configuration is valid!"
echo "✓ The configuration uses the same validation logic as our webhook"
echo "✓ This configuration should work correctly with Mimir's Alertmanager"
echo "✓ Validation performed using $AMTOOL_PACKAGE"
