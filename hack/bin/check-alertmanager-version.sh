#!/usr/bin/env bash

set -euo pipefail

# --- Configuration ---
MIMIR_APP_REPO="giantswarm/mimir-app"
MIMIR_REPO="grafana/mimir"
# ALERTMANAGER_MODULE is provided by alertmanager-dependency.sh
GO_MOD_PATH="./go.mod"
TMP_DIR="/tmp/jq-install"

# --- Ensure jq is available ---
# This function checks if jq (a lightweight JSON processor) is available on the system.
# If not found, it automatically downloads and installs jq temporarily for this script.
# jq is required to parse GitHub API responses and extract version information from JSON.
require_jq() {
  # Check if jq is already installed and available in PATH
  if command -v jq >/dev/null 2>&1; then
    return 0
  fi

  echo "🔧 jq not found, installing via curl..."

  # Create temporary directory for jq installation
  mkdir -p "$TMP_DIR"
  cd "$TMP_DIR"

  # Determine the correct jq binary based on the operating system
  # Different OS architectures require different jq binaries
  case "$(uname -s)" in
    Linux)
      jq_bin="jq-linux64"
      ;;
    Darwin)
      jq_bin="jq-osx-amd64"
      ;;
    *)
      echo "❌ Unsupported OS: $(uname -s)"
      exit 1
      ;;
  esac

  # Download the appropriate jq binary from the official GitHub releases
  # Use silent mode (-s) and follow redirects (-L) for a clean download
  curl -sSL -o jq "https://github.com/stedolan/jq/releases/latest/download/${jq_bin}"

  # Make the downloaded binary executable
  chmod +x jq

  # Add the temporary directory to PATH so jq can be found by subsequent commands
  # This is only temporary for the duration of this script execution
  export PATH="$TMP_DIR:$PATH"

  echo "✅ jq installed temporarily in $TMP_DIR"
}

# --- Alertmanager dependency resolution ---
# shellcheck source=hack/bin/alertmanager-dependency.sh
source "$(dirname "${BASH_SOURCE[0]}")/alertmanager-dependency.sh"

require_jq

# --- Get latest mimir-app release and Mimir version ---
echo "🔍 Fetching latest mimir-app release..."

# Get the latest release tag from mimir-app repository
latest_mimir_app_tag="$(curl -s "https://api.github.com/repos/${MIMIR_APP_REPO}/releases/latest" | \
  jq -r '.tag_name')"

if [[ -z "${latest_mimir_app_tag}" || "${latest_mimir_app_tag}" == "null" ]]; then
  echo "❌ Could not find latest mimir-app release tag."
  exit 1
fi

echo "✅ Latest mimir-app release: ${latest_mimir_app_tag}"

# Download Chart.yaml from the latest mimir-app release
echo "📦 Downloading Chart.yaml from mimir-app @ ${latest_mimir_app_tag}..."
mimir_chart_yaml="$(curl -fsSL "https://raw.githubusercontent.com/${MIMIR_APP_REPO}/refs/tags/${latest_mimir_app_tag}/helm/mimir/Chart.yaml")"

# Extract appVersion from Chart.yaml
mimir_app_version="$(echo "$mimir_chart_yaml" | grep -E '^appVersion:' | awk '{ print $2 }' | tr -d '"')"

if [[ -z "$mimir_app_version" ]]; then
  echo "❌ Could not find appVersion in mimir-app Chart.yaml."
  exit 1
fi

echo "✅ Mimir app version: ${mimir_app_version}"

# Convert app version to tag format (e.g., "2.14.1" -> "mimir-2.14.1")
mimir_tag="mimir-${mimir_app_version}"

# --- Download go.mod from Mimir ---
echo "📦 Downloading go.mod from Mimir @ ${mimir_tag}..."
mimir_go_mod="$(curl -fsSL "https://raw.githubusercontent.com/${MIMIR_REPO}/refs/tags/${mimir_tag}/go.mod")"

# --- Extract Alertmanager version from Mimir's go.mod ---
echo "🔍 Extracting Alertmanager version from Mimir's go.mod..."
mimir_alertmanager="$(effective_alertmanager "$mimir_go_mod")"

if [[ -z "$mimir_alertmanager" ]]; then
  echo "❌ Could not find the ${ALERTMANAGER_MODULE} dependency in Mimir's go.mod."
  echo "   Looked for a go.mod replace directive and a require entry, found neither."
  exit 1
fi

read -r mimir_alertmanager_module mimir_alertmanager_version <<< "$mimir_alertmanager"
echo "✅ Mimir Alertmanager dependency: ${mimir_alertmanager_module} ${mimir_alertmanager_version}"

# --- Extract local Alertmanager version from your repo's go.mod ---
echo "🔍 Extracting Alertmanager version from local go.mod..."
local_alertmanager="$(effective_alertmanager "$(<"${GO_MOD_PATH}")")"

if [[ -z "$local_alertmanager" ]]; then
  echo "❌ Could not find the ${ALERTMANAGER_MODULE} dependency in local go.mod."
  echo "   Looked for a go.mod replace directive and a require entry in ${GO_MOD_PATH}, found neither."
  exit 1
fi

read -r local_alertmanager_module local_alertmanager_version <<< "$local_alertmanager"
echo "✅ Local Alertmanager dependency: ${local_alertmanager_module} ${local_alertmanager_version}"

# --- Compare dependencies ---
if [[ "${mimir_alertmanager_module}" != "${local_alertmanager_module}" || \
      "${mimir_alertmanager_version}" != "${local_alertmanager_version}" ]]; then
  echo ""
  echo "❌ ALERTMANAGER DEPENDENCY MISMATCH!"
  echo "   Mimir (${mimir_tag}) uses: ${mimir_alertmanager_module} ${mimir_alertmanager_version}"
  echo "   Your operator uses:        ${local_alertmanager_module} ${local_alertmanager_version}"
  echo ""
  echo "💡 Please update your go.mod to match the Alertmanager dependency from the Mimir release."
  echo ""
  echo "🔧 Run these commands to update your go.mod:"
  if [[ "${mimir_alertmanager_module}" == "${ALERTMANAGER_MODULE}" ]]; then
    # Mimir tracks upstream Alertmanager, so the fork replacement must be dropped.
    echo "   go mod edit -dropreplace=${ALERTMANAGER_MODULE}"
    echo "   go get ${ALERTMANAGER_MODULE}@${mimir_alertmanager_version}"
  else
    echo "   go mod edit -replace=${ALERTMANAGER_MODULE}=${mimir_alertmanager_module}@${mimir_alertmanager_version}"
  fi
  echo "   go mod tidy"
  exit 1
else
  echo ""
  echo "✅ SUCCESS: Alertmanager dependency matches Mimir (${mimir_tag}) 🎉"
fi
