#!/usr/bin/env bash

set -euo pipefail

# Verifies one installation's alerting state and exits non-zero on any failure.
#
# Used at every step of the alerting-config cutover, where the failure mode is
# silent-then-paging: a mismatched monitor key or a missing configuration means the
# Cronitor monitor exists but is never pinged, and pages once its grace period expires.
#
# Everything is discovered from the cluster the kube context points at: the installation
# name and the Alertmanager URL from the operator Deployment's args, the Cronitor
# management key from its credentials Secret.
#
# Prerequisites: kubectl (logged in), curl, jq, yq v4.

NAMESPACE="monitoring"
DEPLOYMENT="observability-operator"
TENANT="giantswarm"
KUBE_CONTEXT=""
LOCAL_PORT="18181"

# yq v4 (github.com/mikefarah/yq). `make verify-alerting` passes the pinned bin/yq; a plain
# `yq` on PATH is often the unrelated Python one, which requireTools rejects.
YQ="${YQ:-yq}"

# Discovered from the cluster.
declare INSTALLATION ALERTMANAGER_URL MONITORING_ENABLED ALERTMANAGER_BASE_URL
declare SECRET_COUNT MIMIR_HTTP_CODE
SECRET_JSON=""
MONITOR_JSON=""
PINGED_MONITOR_KEY=""

TMP_DIR="$(mktemp -d -t verify-alerting.XXXXXX)"
PORT_FORWARD_PID=""

declare -a RESULTS=()
FAILURES=0

function usage {
  cat <<'EOF'
Verify one installation's alerting state: the Alertmanager configuration Mimir actually
serves, the Cronitor heartbeat monitor, and the link between them. Exits non-zero on any
failure.

Usage:
  ./hack/bin/verify-alerting.sh [options]
  make verify-alerting

Options:
  -c, --context <name>     kube context to use (default: the current one)
  -n, --namespace <ns>     namespace of the operator Deployment (default: monitoring)
  -t, --tenant <tenant>    tenant whose configuration to verify (default: giantswarm)
  -h, --help               show this help

Environment:
  CRONITOR_HEARTBEAT_MANAGEMENT_KEY   Cronitor API key, when it is no longer readable from
                                      the operator's credentials Secret
  YQ                                  path to yq v4 (default: yq on PATH)
EOF
}

function die {
  echo "Error: $*" >&2
  exit 2
}

function kc {
  if [[ -n "$KUBE_CONTEXT" ]]; then
    kubectl --context "$KUBE_CONTEXT" "$@"
  else
    kubectl "$@"
  fi
}

function pass {
  RESULTS+=("PASS $1")
  echo "  ✓ PASS  $1"
}

function fail {
  RESULTS+=("FAIL $1")
  FAILURES=$((FAILURES + 1))
  echo "  ✗ FAIL  $1"
}

function skip {
  RESULTS+=("SKIP $1")
  echo "  - SKIP  $1"
}

function info {
  echo "          $*"
}

function cleanupAtExit {
  if [[ -n "$PORT_FORWARD_PID" ]]; then
    kill "$PORT_FORWARD_PID" &> /dev/null || true
  fi
  rm -rf "$TMP_DIR"
}

function parseArgs {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -c | --context)
        KUBE_CONTEXT="${2:-}"
        [[ -n "$KUBE_CONTEXT" ]] || die "--context requires a value"
        shift 2
        ;;
      -n | --namespace)
        NAMESPACE="${2:-}"
        [[ -n "$NAMESPACE" ]] || die "--namespace requires a value"
        shift 2
        ;;
      -t | --tenant)
        TENANT="${2:-}"
        [[ -n "$TENANT" ]] || die "--tenant requires a value"
        shift 2
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *)
        echo "Error: unknown argument: $1" >&2
        echo >&2
        usage >&2
        exit 2
        ;;
    esac
  done
}

function requireTools {
  local tool missing=()
  for tool in kubectl curl jq diff "$YQ"; do
    command -v "$tool" > /dev/null 2>&1 || missing+=("$tool")
  done
  [[ ${#missing[@]} -eq 0 ]] || die "missing required tools: ${missing[*]}"

  # The Python yq and yq v3 take different expressions and would silently produce nothing
  # rather than fail.
  "$YQ" --version 2>&1 | grep -q "version v4" \
    || die "$YQ is not yq v4 (github.com/mikefarah/yq) — run 'make verify-alerting', which uses the pinned bin/yq, or set YQ"
}

# argValue prints the value of a --flag=value style argument, empty if absent.
function argValue {
  local flag="$1" arg
  shift
  for arg in "$@"; do
    if [[ "$arg" == "$flag="* ]]; then
      printf '%s' "${arg#*=}"
      return
    fi
  done
}

function discoverFromDeployment {
  local json
  local -a args

  json="$(kc get deployment -n "$NAMESPACE" "$DEPLOYMENT" -o json 2> /dev/null)" \
    || die "deployment $DEPLOYMENT not found in namespace $NAMESPACE — is the kube context pointing at a management cluster?"

  mapfile -t args < <(jq -r '.spec.template.spec.containers[0].args[]' <<< "$json")

  INSTALLATION="$(argValue "--management-cluster-name" "${args[@]}")"
  ALERTMANAGER_URL="$(argValue "--alertmanager-url" "${args[@]}")"
  MONITORING_ENABLED="$(argValue "--monitoring-enabled" "${args[@]}")"

  [[ -n "$INSTALLATION" ]] || die "could not read --management-cluster-name from the operator Deployment"
  [[ -n "$ALERTMANAGER_URL" ]] || die "could not read --alertmanager-url from the operator Deployment"

  info "installation:       $INSTALLATION"
  info "alertmanager URL:   $ALERTMANAGER_URL"
  info "monitoring enabled: ${MONITORING_ENABLED:-<unset>}"
  info "tenant:             $TENANT"
}

# discoverCronitorManagementKey is best-effort: the key leaves the operator's credentials
# Secret once alerting-config owns the heartbeat, and the Alertmanager checks do not need it.
function discoverCronitorManagementKey {
  if [[ -n "${CRONITOR_HEARTBEAT_MANAGEMENT_KEY:-}" ]]; then
    info "cronitor key:       CRONITOR_HEARTBEAT_MANAGEMENT_KEY"
    return
  fi

  CRONITOR_HEARTBEAT_MANAGEMENT_KEY="$(
    kc get secret -n "$NAMESPACE" "$DEPLOYMENT-credentials" -o json 2> /dev/null \
      | jq -r '.data.cronitorHeartbeatManagementKey // "" | @base64d'
  )" || true

  if [[ -n "$CRONITOR_HEARTBEAT_MANAGEMENT_KEY" ]]; then
    info "cronitor key:       $NAMESPACE/$DEPLOYMENT-credentials"
  else
    info "cronitor key:       not found in $NAMESPACE/$DEPLOYMENT-credentials"
  fi
}

# discoverConfigSecrets finds every Secret the Alertmanager controller reconciles and
# narrows them to this tenant. The tenant is read from a label or an annotation, matching
# predicates.NewAlertmanagerConfigSecretsPredicate.
function discoverConfigSecrets {
  local all found matching
  all="$(kc get secrets --all-namespaces -l observability.giantswarm.io/kind=alertmanager-config -o json)"

  found="$(jq -r '.items[] | "\(.metadata.namespace)/\(.metadata.name) tenant=\((.metadata.labels["observability.giantswarm.io/tenant"] // .metadata.annotations["observability.giantswarm.io/tenant"]) // "<none>")"' <<< "$all")"
  if [[ -n "$found" ]]; then
    info "config Secrets:"
    while IFS= read -r line; do info "  $line"; done <<< "$found"
  fi

  matching="$(jq -c --arg tenant "$TENANT" \
    '[.items[] | select(((.metadata.labels["observability.giantswarm.io/tenant"] // .metadata.annotations["observability.giantswarm.io/tenant"]) // "") == $tenant)]' <<< "$all")"

  SECRET_COUNT="$(jq 'length' <<< "$matching")"
  if [[ "$SECRET_COUNT" -eq 1 ]]; then
    SECRET_JSON="$(jq -c '.[0]' <<< "$matching")"
  fi
}

# startPortForward tunnels an in-cluster Service to LOCAL_PORT.
function startPortForward {
  local namespace="$1" service="$2" port="$3" attempt=0

  kc port-forward -n "$namespace" "svc/$service" "$LOCAL_PORT:$port" &> /dev/null &
  PORT_FORWARD_PID="$!"

  while [[ "$attempt" -lt 20 ]]; do
    attempt=$((attempt + 1))
    if curl -s -o /dev/null --max-time 1 "http://127.0.0.1:$LOCAL_PORT"; then
      return
    fi
    sleep 0.5
  done
  die "port-forward to svc/$service in namespace $namespace did not become ready"
}

# resolveAlertmanagerBaseURL makes the operator's --alertmanager-url reachable from here.
# In-cluster Service addresses are port-forwarded; anything else is used as-is.
function resolveAlertmanagerBaseURL {
  local rest path host port service namespace

  rest="${ALERTMANAGER_URL#*://}"
  path=""
  if [[ "$rest" == */* ]]; then
    path="/${rest#*/}"
    path="${path%/}"
    rest="${rest%%/*}"
  fi

  host="${rest%%:*}"
  port="8080"
  [[ "$rest" == *:* ]] && port="${rest##*:}"

  if [[ "$host" != *.svc* ]]; then
    ALERTMANAGER_BASE_URL="${ALERTMANAGER_URL%/}"
    info "alertmanager:       querying $ALERTMANAGER_BASE_URL directly"
    return
  fi

  service="${host%%.*}"
  namespace="${host#*.}"
  namespace="${namespace%%.*}"

  info "alertmanager:       port-forwarding svc/$service in namespace $namespace"
  startPortForward "$namespace" "$service" "$port"
  ALERTMANAGER_BASE_URL="http://127.0.0.1:$LOCAL_PORT$path"
}

# fetchMimirConfig retrieves the configuration Mimir Alertmanager currently serves for the
# tenant. The response is converted to JSON once so every check reads it with jq.
# https://grafana.com/docs/mimir/latest/references/http-api/#get-alertmanager-configuration
function fetchMimirConfig {
  MIMIR_HTTP_CODE="$(
    curl -s -o "$TMP_DIR/response.yaml" -w '%{http_code}' --max-time 30 \
      -H "X-Scope-OrgID: $TENANT" "$ALERTMANAGER_BASE_URL/api/v1/alerts"
  )"

  [[ "$MIMIR_HTTP_CODE" == "200" ]] || return 0

  "$YQ" -o=json '.' "$TMP_DIR/response.yaml" > "$TMP_DIR/response.json" \
    || die "Mimir returned a body that is not YAML: $(head -c 200 "$TMP_DIR/response.yaml")"

  jq -r '.alertmanager_config' "$TMP_DIR/response.json" > "$TMP_DIR/mimir.alertmanager.yaml"
  "$YQ" -o=json '.' "$TMP_DIR/mimir.alertmanager.yaml" > "$TMP_DIR/mimir.alertmanager.json" \
    || die "the configuration Mimir serves is not valid YAML"
}

# normalized prints a file with trailing newlines collapsed to exactly one, so a difference
# in trailing whitespace is not reported as configuration drift.
function normalized {
  printf '%s\n' "$(cat "$1")"
}

# templateNames prints the basenames of the *.tmpl keys of the Secret. Alertmanager
# templates share one namespace and are keyed by basename, as the operator sends them
# (path.Base in pkg/alerting/alertmanager).
function secretTemplateNames {
  jq -r '.data | keys[] | select(endswith(".tmpl")) | split("/") | last' <<< "$SECRET_JSON" | sort
}

function checkConfigReachedMimir {
  echo "1. The configuration Mimir serves matches the Secret on the cluster"

  if [[ "$MIMIR_HTTP_CODE" != "200" ]]; then
    fail "Mimir Alertmanager has no configuration for tenant $TENANT (HTTP $MIMIR_HTTP_CODE)"
    return
  fi

  if [[ "$SECRET_COUNT" -ne 1 ]]; then
    skip "cannot compare — $SECRET_COUNT Secrets carry tenant $TENANT (see check 2)"
    return
  fi

  local secret_ref drifted=0
  secret_ref="$(jq -r '"\(.metadata.namespace)/\(.metadata.name)"' <<< "$SECRET_JSON")"

  jq -r '.data["alertmanager.yaml"] // "" | @base64d' <<< "$SECRET_JSON" > "$TMP_DIR/secret.alertmanager.yaml"
  if [[ ! -s "$TMP_DIR/secret.alertmanager.yaml" ]]; then
    fail "$secret_ref has no alertmanager.yaml key"
    return
  fi

  if ! diff -u <(normalized "$TMP_DIR/secret.alertmanager.yaml") <(normalized "$TMP_DIR/mimir.alertmanager.yaml") > "$TMP_DIR/config.diff"; then
    drifted=1
    fail "alertmanager.yaml in Mimir differs from $secret_ref"
    info "--- $secret_ref   +++ Mimir"
    while IFS= read -r line; do info "$line"; done < <(tail -n +3 "$TMP_DIR/config.diff" | head -n 40)
  fi

  local -a secret_templates mimir_templates
  mapfile -t secret_templates < <(secretTemplateNames)
  mapfile -t mimir_templates < <(jq -r '.template_files // {} | keys[]' "$TMP_DIR/response.json" | sort)

  if [[ "${secret_templates[*]-}" != "${mimir_templates[*]-}" ]]; then
    drifted=1
    fail "the template files in Mimir are not the ones in $secret_ref"
    info "$secret_ref: ${secret_templates[*]-<none>}"
    info "Mimir:       ${mimir_templates[*]-<none>}"
  else
    local template
    for template in "${secret_templates[@]-}"; do
      [[ -n "$template" ]] || continue
      jq -r --arg key "$template" '.data | to_entries[] | select((.key | split("/") | last) == $key) | .value | @base64d' <<< "$SECRET_JSON" > "$TMP_DIR/secret.template"
      jq -r --arg key "$template" '.template_files[$key]' "$TMP_DIR/response.json" > "$TMP_DIR/mimir.template"
      if ! diff -q <(normalized "$TMP_DIR/secret.template") <(normalized "$TMP_DIR/mimir.template") > /dev/null; then
        drifted=1
        fail "template $template in Mimir differs from $secret_ref"
      fi
    done
  fi

  if [[ "$drifted" -eq 0 ]]; then
    pass "Mimir serves exactly what $secret_ref contains"
  fi
}

function checkSingleConfigSource {
  echo "2. Exactly one Secret ships the configuration for tenant $TENANT"

  case "$SECRET_COUNT" in
    0)
      fail "no Secret carries kind=alertmanager-config and tenant=$TENANT — nothing is shipping this tenant's configuration"
      ;;
    1)
      pass "$(jq -r '"\(.metadata.namespace)/\(.metadata.name)"' <<< "$SECRET_JSON")"
      ;;
    *)
      fail "$SECRET_COUNT Secrets carry tenant=$TENANT and will fight over one Mimir configuration, last writer wins"
      ;;
  esac
}

# pingedMonitorKeys prints the Cronitor monitor keys the live configuration pings, taken
# from https://cronitor.link/p/<ping-key>/<monitor-key>?env=<pipeline>. This is the ground
# truth for which monitor must exist: the operator writes the key in Go, the configuration
# writes it again in YAML, and the two can drift once they no longer share a chart.
function pingedMonitorKeys {
  local url rest
  local -a keys=()

  while IFS= read -r url; do
    [[ -n "$url" ]] || continue
    rest="${url#*cronitor.link/p/}" # <ping-key>/<monitor-key>?env=...
    rest="${rest#*/}"               # <monitor-key>?env=...
    keys+=("${rest%%\?*}")
  done < <(jq -r '[.receivers[]?.webhook_configs[]?.url // empty] | .[]' "$TMP_DIR/mimir.alertmanager.json" | grep -F "cronitor.link/p/" || true)

  [[ ${#keys[@]} -gt 0 ]] || return 0
  printf '%s\n' "${keys[@]}" | sort -u
}

# cronitorMonitor prints a monitor's JSON, or its HTTP status code if it could not be read.
function cronitorMonitor {
  local key="$1" code
  code="$(
    curl -s -o "$TMP_DIR/monitor.json" -w '%{http_code}' --max-time 30 \
      --user "$CRONITOR_HEARTBEAT_MANAGEMENT_KEY:" \
      -H "Accept: application/json" \
      "https://cronitor.io/api/monitors/$key"
  )"
  if [[ "$code" != "200" ]]; then
    printf '%s' "$code"
    return 1
  fi
  cat "$TMP_DIR/monitor.json"
}

function checkHeartbeatWiring {
  echo "3. The Cronitor monitor the configuration pings exists"

  if [[ "$MIMIR_HTTP_CODE" != "200" ]]; then
    skip "cannot read the live configuration from Mimir (see check 1)"
    return
  fi

  local -a keys
  mapfile -t keys < <(pingedMonitorKeys)

  if [[ ${#keys[@]} -eq 0 ]]; then
    fail "the configuration in Mimir pings no Cronitor monitor — no receiver has a cronitor.link/p/ webhook URL"
    return
  fi
  if [[ ${#keys[@]} -gt 1 ]]; then
    fail "the configuration pings ${#keys[@]} different Cronitor monitors: ${keys[*]}"
    return
  fi

  PINGED_MONITOR_KEY="${keys[0]}"
  info "pinged monitor:     $PINGED_MONITOR_KEY"
  if [[ "$PINGED_MONITOR_KEY" != "mimir-$INSTALLATION" ]]; then
    info "note: the operator's own derivation is mimir-$INSTALLATION, expected to differ only once a Heartbeat CR sets the key"
  fi

  if [[ -z "${CRONITOR_HEARTBEAT_MANAGEMENT_KEY:-}" ]]; then
    skip "no Cronitor management key available — set CRONITOR_HEARTBEAT_MANAGEMENT_KEY"
    return
  fi

  local result
  if ! result="$(cronitorMonitor "$PINGED_MONITOR_KEY")"; then
    if [[ "$result" == "404" ]]; then
      fail "the configuration pings $PINGED_MONITOR_KEY, which does not exist in Cronitor — every ping is discarded and the real monitor pages once its grace period expires"
    else
      fail "could not read monitor $PINGED_MONITOR_KEY from Cronitor (HTTP $result)"
    fi
    return
  fi

  MONITOR_JSON="$result"
  pass "monitor $PINGED_MONITOR_KEY exists and is the one the configuration pings"
}

function checkHeartbeatPinged {
  echo "4. The Cronitor monitor is receiving pings"

  if [[ -z "$MONITOR_JSON" ]]; then
    skip "no monitor to inspect (see check 3)"
    return
  fi

  local initialized passing paused disabled stamp
  initialized="$(jq -r '.initialized // false' <<< "$MONITOR_JSON")"
  passing="$(jq -r '.passing // false' <<< "$MONITOR_JSON")"
  paused="$(jq -r '.paused // false' <<< "$MONITOR_JSON")"
  disabled="$(jq -r '.disabled // false' <<< "$MONITOR_JSON")"
  stamp="$(jq -r '.latest_event.stamp // empty' <<< "$MONITOR_JSON")"

  info "schedule:           $(jq -r '.schedule // "<unset>"' <<< "$MONITOR_JSON"), grace $(jq -r '.grace_seconds // "<unset>"' <<< "$MONITOR_JSON")s"
  if [[ -n "$stamp" ]]; then
    info "last event:         $(awk -v s="$stamp" 'BEGIN { printf "%d seconds ago", systime() - s }')"
  fi

  # Cronitor already decides "pinged recently" from the monitor's own schedule and grace
  # period and reports it as passing. Recomputing that here could only disagree with the
  # thing that actually pages.
  if [[ "$initialized" != "true" ]]; then
    fail "monitor $PINGED_MONITOR_KEY has never received a ping"
    return
  fi
  if [[ "$paused" == "true" || "$disabled" == "true" ]]; then
    fail "monitor $PINGED_MONITOR_KEY will not alert (paused=$paused disabled=$disabled)"
    return
  fi
  if [[ "$passing" != "true" ]]; then
    fail "monitor $PINGED_MONITOR_KEY is failing — its last ping is older than its schedule plus grace period"
    return
  fi

  pass "monitor $PINGED_MONITOR_KEY is passing"
}

function checkHeartbeatCR {
  echo "5. The Heartbeat resource reports Ready=True"

  if ! kc get crd heartbeats.observability.giantswarm.io &> /dev/null; then
    skip "the Heartbeat CRD is not registered on this cluster"
    return
  fi
  if [[ -z "$PINGED_MONITOR_KEY" ]]; then
    skip "no monitor key to match a Heartbeat against (see check 3)"
    return
  fi

  local matching count ref ready
  matching="$(
    kc get heartbeats.observability.giantswarm.io --all-namespaces -o json \
      | jq -c --arg key "$PINGED_MONITOR_KEY" \
        '[.items[] | select((.spec.provider.cronitor.key // .metadata.name) == $key)]'
  )"
  count="$(jq 'length' <<< "$matching")"

  if [[ "$count" -eq 0 ]]; then
    fail "no Heartbeat resource owns monitor key $PINGED_MONITOR_KEY"
    return
  fi
  if [[ "$count" -gt 1 ]]; then
    fail "$count Heartbeat resources claim monitor key $PINGED_MONITOR_KEY and will fight over it, last writer wins"
    while IFS= read -r line; do info "$line"; done < <(jq -r '.[] | "\(.metadata.namespace)/\(.metadata.name)"' <<< "$matching")
    return
  fi

  ref="$(jq -r '.[0] | "\(.metadata.namespace)/\(.metadata.name)"' <<< "$matching")"
  ready="$(jq -r '.[0].status.conditions[]? | select(.type == "Ready") | .status' <<< "$matching")"

  case "$ready" in
    True) pass "$ref is Ready" ;;
    "") fail "$ref has no Ready condition yet" ;;
    *) fail "$ref is Ready=$ready — $(jq -r '.[0].status.conditions[]? | select(.type == "Ready") | "\(.reason): \(.message)"' <<< "$matching")" ;;
  esac
}

function report {
  local result
  echo
  echo "=============================================================="
  if [[ "$FAILURES" -eq 0 ]]; then
    echo " PASSED — ${#RESULTS[@]} checks, no failures"
    echo "=============================================================="
    return 0
  fi

  echo " FAILED — $FAILURES of ${#RESULTS[@]} checks"
  echo "=============================================================="
  for result in "${RESULTS[@]}"; do
    [[ "$result" == FAIL* ]] && echo " $result"
  done
  return 1
}

function main {
  trap cleanupAtExit EXIT

  parseArgs "$@"
  requireTools

  echo "=== Verifying alerting state"
  info "kube context:       ${KUBE_CONTEXT:-$(kc config current-context)}"
  discoverFromDeployment
  discoverCronitorManagementKey
  discoverConfigSecrets
  resolveAlertmanagerBaseURL
  fetchMimirConfig
  echo

  checkConfigReachedMimir
  checkSingleConfigSource

  # The operator deletes the monitor when monitoring is switched off for the installation,
  # so an absent heartbeat is then the expected state rather than a failure.
  if [[ "$MONITORING_ENABLED" == "false" ]]; then
    echo "3-5. Heartbeat"
    skip "the operator runs with --monitoring-enabled=false, which deletes the monitor"
  else
    checkHeartbeatWiring
    checkHeartbeatPinged
    checkHeartbeatCR
  fi

  report
}

main "$@"
