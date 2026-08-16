#!/usr/bin/env bash

# shellcheck shell=bash
#
# Library — source this file, do not execute it:
#
#   source "$(dirname "${BASH_SOURCE[0]}")/alertmanager-dependency.sh"
#
# Resolves which Alertmanager module our webhook actually compiles against, so
# that tooling (amtool, version checks) can be built from the very same source
# and stay in sync with go.mod automatically.

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  echo "❌ ${BASH_SOURCE[0]} is a library and must be sourced, not executed." >&2
  exit 1
fi

ALERTMANAGER_MODULE="${ALERTMANAGER_MODULE:-github.com/prometheus/alertmanager}"

ALERTMANAGER_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# effective_alertmanager <go.mod contents>
#
# Prints "<module> <version>", or nothing when the module is absent.
#
# A replace directive wins over the require entry: Mimir pinned Grafana's fork
# via a replace directive up to mimir-3.0.x, and depends on upstream
# prometheus/alertmanager directly since mimir-3.1.0.
effective_alertmanager() {
  local go_mod="$1"
  local replaced

  # `|| true` keeps a non-matching grep from aborting the caller under pipefail.
  replaced="$(echo "$go_mod" | { grep -E "${ALERTMANAGER_MODULE}[[:space:]]+=>" || true; })"
  if [[ -n "$replaced" ]]; then
    echo "$replaced" | awk '{ print $(NF-1), $NF }'
    return
  fi

  echo "$go_mod" | { grep -E "^[[:space:]]*${ALERTMANAGER_MODULE} v" || true; } | awk '{ print $1, $2 }'
}

# alertmanager_amtool_package [go.mod path]
#
# Prints the versioned amtool package to hand to `go run` / `go install`, e.g.
# "github.com/prometheus/alertmanager/cmd/amtool@v0.31.1". Defaults to this
# repository's go.mod. Fails when the dependency cannot be resolved.
alertmanager_amtool_package() {
  local go_mod_path="${1:-${ALERTMANAGER_REPO_ROOT}/go.mod}"
  local module version

  read -r module version <<< "$(effective_alertmanager "$(<"$go_mod_path")")"

  if [[ -z "${version:-}" ]]; then
    echo "❌ Could not resolve the ${ALERTMANAGER_MODULE} dependency from ${go_mod_path}." >&2
    echo "   Looked for a replace directive and a require entry, found neither." >&2
    return 1
  fi

  echo "${module}/cmd/amtool@${version}"
}
