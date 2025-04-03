#!/bin/bash
set -euo pipefail

ALERTMANAGER_VERSION="0.28.1"

TARGET_DIR="$(dirname "$0")"
# Download amtool if not present
if ! command -v "$TARGET_DIR"/amtool >/dev/null 2>&1; then
  echo "amtool not found, downloading Alertmanager $ALERTMANAGER_VERSION release..."
  # Determine OS and architecture
  OS="$(uname | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"
  if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
  fi

  # Construct the download URL for Alertmanager
  DOWNLOAD_URL="https://github.com/prometheus/alertmanager/releases/download/v$ALERTMANAGER_VERSION/alertmanager-$ALERTMANAGER_VERSION.$OS-$ARCH.tar.gz"
  
  TMP_DIR="$(mktemp -d -t validate-alertmanager-config.XXXXXX)"
  trap 'rm -rf "$TMPDIR"' EXIT
  TAR_FILE="$TMP_DIR/alertmanager.tar.gz"
  
  echo "Downloading from $DOWNLOAD_URL..."
  curl -L "$DOWNLOAD_URL" -o "$TAR_FILE"
  
  # Extract the tarball
  tar -xzf "$TAR_FILE" -C "$TMP_DIR"
  
  # Find the amtool binary in the extracted contents
  AMTOOL_PATH=$(find "$TMP_DIR" -type f -name 'amtool' | head -n1)
  if [ -z "$AMTOOL_PATH" ]; then
    echo "amtool binary not found in the downloaded archive."
    rm -rf "$TMP_DIR"
    exit 1
  fi
  
  # Move the amtool binary to the hack/bin directory
  mv "$AMTOOL_PATH" "$TARGET_DIR/amtool"
  chmod +x "$TARGET_DIR/amtool"
  
  rm -rf "$TMP_DIR"
  echo "amtool downloaded and installed to $TARGET_DIR/amtool"
fi

# Template the helm chart
echo "Rendering helm chart..."
helm template observability-operator helm/observability-operator --namespace alertmanager --set alerting.slackAPIURL="https://gigantic.slack.com" --set monitoring.opsgenieApiKey="apikey" > rendered.yaml

# Extract the secret that contains the Alertmanager configuration
# This assumes that the secret's labels include observability.giantswarm.io/kind: alertmanager-config
SECRET_NAME="$(yq eval 'select(.metadata.labels."observability.giantswarm.io/kind" == "alertmanager-config") | .metadata.name' "$TMPDIR/rendered.yaml" | head -n1)"
if [ -z "$SECRET_NAME" ]; then
  echo "Alertmanager secret not found in rendered templates."
  exit 1
fi

# Assuming the alertmanager config is stored under the key "alertmanager.yaml"
CONFIG_B64="$(yq eval 'select(.metadata.name == "'"$SECRET_NAME"'") | .data."alertmanager.yaml"' "$TMPDIR/rendered.yaml" | head -n1)"
if [ -z "$CONFIG_B64" ]; then
  echo "No alertmanager.yaml key found in secret $SECRET_NAME."
  exit 1
fi

# Decode the configuration
echo "$CONFIG_B64" | base64 -d > "$TMPDIR/alertmanager.yaml"

# Validate the configuration using amtool
echo "Validating Alertmanager configuration..."
"$TARGET_DIR"/amtool check-config "$TMPDIR/alertmanager.yaml"

echo "Alertmanager configuration is valid."
