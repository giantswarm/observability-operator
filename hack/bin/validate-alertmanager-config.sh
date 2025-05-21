#!/bin/bash
set -euo pipefail

ALERTMANAGER_VERSION="0.28.1"
YQ="yq"

TMP_DIR="$(mktemp -d -t validate-alertmanager-config.XXXXXX)"
trap 'rm -rf "$TMP_DIR"' EXIT

TARGET_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
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
  
  TAR_FILE="$TMP_DIR/alertmanager.tar.gz"
  ARCHIVE_DIRECTORY="$TMP_DIR/alertmanager"
  
  echo "Downloading from $DOWNLOAD_URL..."
  curl -L "$DOWNLOAD_URL" -o "$TAR_FILE"
  
  # Extract the tarball
  mkdir -p "$TMP_DIR/alertmanager"
  tar -xzf "$TAR_FILE" -C "$ARCHIVE_DIRECTORY"
  # Move the amtool binary to the hack/bin directory
  mv "$ARCHIVE_DIRECTORY/alertmanager-$ALERTMANAGER_VERSION.$OS-$ARCH/amtool" "$TARGET_DIR/amtool"
  chmod +x "$TARGET_DIR/amtool"
  
  echo "amtool downloaded and installed to $TARGET_DIR/amtool"
fi

# Template the helm chart
echo "Rendering helm chart..."
helm dependency build helm/observability-operator
helm template observability-operator helm/observability-operator --namespace alertmanager --set monitoring.opsgenieApiKey="apikey" > "$TMP_DIR/rendered.yaml"

# Extract the secret that contains the Alertmanager configuration
# This assumes that the secret's labels include observability.giantswarm.io/kind: alertmanager-config
SECRET_NAME="$($YQ eval 'select(.metadata.labels."observability.giantswarm.io/kind" == "alertmanager-config") | .metadata.name' "$TMP_DIR/rendered.yaml" | head -n1)"
if [ -z "$SECRET_NAME" ]; then
  echo "Alertmanager secret not found in rendered templates."
  exit 1
fi

# Assuming the alertmanager config is stored under the key "alertmanager.yaml"
CONFIG_B64="$($YQ eval 'select(.metadata.name == "'"$SECRET_NAME"'") | .data."alertmanager.yaml"' "$TMP_DIR/rendered.yaml" | head -n1)"
if [ -z "$CONFIG_B64" ]; then
  echo "No alertmanager.yaml key found in secret $SECRET_NAME."
  exit 1
fi

# Decode the configuration
echo "$CONFIG_B64" | base64 -d > "$TMP_DIR/alertmanager.yaml"

# Validate the configuration using amtool
echo "Validating Alertmanager configuration..."
"$TARGET_DIR"/amtool check-config "$TMP_DIR/alertmanager.yaml"

echo "Alertmanager configuration is valid."
