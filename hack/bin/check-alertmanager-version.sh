#!/bin/bash
set -euo pipefail

# --- Configuration ---
MIMIR_REPO="grafana/mimir"
ALERTMANAGER_MODULE="github.com/prometheus/alertmanager"
GO_MOD_PATH="./go.mod"
TMP_DIR="/tmp/jq-install"

# --- Ensure jq is available ---
require_jq() {
  if command -v jq >/dev/null 2>&1; then
    return 0
  fi

  echo "🔧 jq not found, installing via curl..."

  mkdir -p "$TMP_DIR"
  cd "$TMP_DIR"

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

  curl -sSL -o jq "https://github.com/stedolan/jq/releases/latest/download/${jq_bin}"
  chmod +x jq
  export PATH="$TMP_DIR:$PATH"

  echo "✅ jq installed temporarily in $TMP_DIR"
}

require_jq

# --- Get latest stable Mimir release (no -rc, no beta) ---
echo "🔍 Fetching latest stable Mimir release tag..."

latest_mimir_tag="$(curl -s "https://api.github.com/repos/${MIMIR_REPO}/releases?per_page=50" | \
  jq -r '.[].tag_name' | \
  grep -E '^mimir-[0-9]+\.[0-9]+\.[0-9]+$' | \
  sort -V | \
  tail -n1)"

if [[ -z "${latest_mimir_tag}" ]]; then
  echo "❌ Could not find a stable Mimir release tag."
  exit 1
fi

echo "✅ Latest stable Mimir release: ${latest_mimir_tag}"

# --- Download go.mod from Mimir ---
echo "📦 Downloading go.mod from Mimir @ ${latest_mimir_tag}..."
mimir_go_mod="$(curl -fsSL "https://raw.githubusercontent.com/${MIMIR_REPO}/${latest_mimir_tag}/go.mod")"

# --- Extract Alertmanager version from Mimir's go.mod ---
echo "🔍 Extracting Alertmanager version from Mimir's go.mod..."
# Look for the replace directive, not the require directive
mimir_alertmanager_version="$(echo "$mimir_go_mod" | grep -E "${ALERTMANAGER_MODULE} =>" | awk '{ print $NF }')"

if [[ -z "$mimir_alertmanager_version" ]]; then
  echo "❌ Could not find Alertmanager replace directive in Mimir's go.mod."
  exit 1
fi

echo "✅ Mimir Alertmanager version: ${mimir_alertmanager_version}"

# --- Extract local Alertmanager version from your repo's go.mod ---
echo "🔍 Extracting Alertmanager version from local go.mod..."
# Handle tabs and spaces in the replace directive
local_alertmanager_version="$(grep -E "${ALERTMANAGER_MODULE}.*=>" "${GO_MOD_PATH}" | awk '{ print $NF }')"

if [[ -z "$local_alertmanager_version" ]]; then
  echo "❌ Could not find Alertmanager replace directive in local go.mod."
  exit 1
fi

echo "✅ Local Alertmanager version: ${local_alertmanager_version}"

# --- Compare versions ---
if [[ "${mimir_alertmanager_version}" != "${local_alertmanager_version}" ]]; then
  echo ""
  echo "❌ ALERTMANAGER VERSION MISMATCH!"
  echo "   Mimir (${latest_mimir_tag}) uses: ${mimir_alertmanager_version}"
  echo "   Your operator uses:               ${local_alertmanager_version}"
  echo ""
  echo "💡 Please update your go.mod to match the Alertmanager version from the latest stable Mimir release."
  exit 1
else
  echo ""
  echo "✅ SUCCESS: Alertmanager version matches Mimir (${latest_mimir_tag}) 🎉"
fi
