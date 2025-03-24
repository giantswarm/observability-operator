#!/bin/bash
set -euo pipefail

# When developing the observability-operator, it is useful to run it locally against a real cluster.
# This script sets up the environment and runs the operator locally.

NAMESPACE="monitoring"
declare -a OLLYOPARGS
declare GRAFANAPORTFORWARDPID MIMIRPORTFORWARDPID ALERTMANAGERPORTFORWARDPID


# Define the arguments for the observability-operator
function defineOLLYOPARGS {
    # We take all args from the deployment, except the leader-elect one which we don't want for local development
    mapfile -t OLLYOPARGS < <(kubectl -n "$NAMESPACE" get deployment observability-operator -ojson | jq -Mr '.spec.template.spec.containers[0].args[]' | grep -v "leader-elect")
}

# Populate environment variables
function setEnvFromSecrets {
  local specenv envname secretname secretkey envvalue

  for specenv in $(kubectl get deployment -n "$NAMESPACE" observability-operator -ojson | jq -c -M '.spec.template.spec.containers[0].env[]') ; do
    envname=$(echo "$specenv" | jq -r '.name')
    secretname=$(echo "$specenv" | jq -r '.valueFrom.secretKeyRef.name')
    secretkey=$(echo "$specenv" | jq -r '.valueFrom.secretKeyRef.key')
    envvalue=$(kubectl get secret -n "$NAMESPACE" "$secretname" -ojson | jq -c -M -r '.data["'"$secretkey"'"]' | base64 -d)

    echo "### setting $envname"
    export "$envname"="$envvalue"

  done
}

# Port-forward the Grafana service
function grafanaPortForward {
  while true; do
    kubectl port-forward -n "$NAMESPACE" svc/grafana 3000:80 &>/dev/null || true
  done &
  GRAFANAPORTFORWARDPID="$!"
}

# Stop the Grafana service port-forward
function stopGrafanaPortForward {
  childpids=$(ps -o pid= --ppid "$GRAFANAPORTFORWARDPID")
  kill "$GRAFANAPORTFORWARDPID" || true
  kill $childpids || true
}

# Port-forward the mimir service
function mimirPortForward {
  while true; do
    kubectl port-forward -n mimir svc/mimir-gateway 8180:80 &>/dev/null || true
  done &
  MIMIRPORTFORWARDPID="$!"
}

# Stop the mimir service port-forward
function stopMimirPortForward {
  childpids=$(ps -o pid= --ppid "$MIMIRPORTFORWARDPID")
  kill "$MIMIRPORTFORWARDPID" || true
  kill $childpids || true
}

# Port-forward the alertmanager service
function alertmanagerPortForward {
  while true; do
    kubectl port-forward -n mimir svc/mimir-alertmanager-headless 8181:8080 &>/dev/null || true
  done &
  ALERTMANAGERPORTFORWARDPID="$!"
}

# Stop the alertmanager service port-forward
function stopAlertmanagerPortForward {
  childpids=$(ps -o pid= --ppid "$ALERTMANAGERPORTFORWARDPID")
  kill "$ALERTMANAGERPORTFORWARDPID" || true
  kill $childpids || true
}

# Pause the in-cluster operator
function pauseInClusterOperator {
    kubectl -n monitoring scale deployment observability-operator --replicas 0
}

# Resume the in-cluster operator
function resumeInClusterOperator {
    kubectl -n monitoring scale deployment observability-operator --replicas 1
}

# Cleanup function
function cleanupAtExit {
  stopGrafanaPortForward
  stopMimirPortForward
  stopAlertmanagerPortForward
  resumeInClusterOperator
}


function main {

  # make sure the script restores cluster at exit
  trap cleanupAtExit SIGINT SIGQUIT SIGABRT SIGTERM EXIT

  echo "### set env vars set"
  setEnvFromSecrets
  echo "### define ollyop args"
  defineOLLYOPARGS
  echo "### ollyorg args set"

  echo "### starting port-forward"
  grafanaPortForward
  mimirPortForward
  alertmanagerPortForward

  echo "### Pausing in-cluster operator"
  pauseInClusterOperator

  echo "### Running operator"
  go run . "${OLLYOPARGS[@]}" -grafana-url http://localhost:3000 -monitoring-metrics-query-url http://localhost:8180/prometheus -alertmanager-url http://localhost:8181

  echo "### Cleanup"
}

main "$@"
