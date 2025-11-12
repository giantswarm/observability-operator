#!/bin/bash

# Manual End-to-End Testing Script for Observability Operator
#
# This script performs manual e2e testing by creating a test workload cluster (WC)
# and verifying that the metric collector is deployed and configured correctly.
#
# Usage:
#   ./hack/bin/manual-testing.sh <installation>
#   make manual-testing INSTALLATION=<installation-name>
#
# Prerequisites:
# - kubectl configured for the target installation
# - Flux reconciliation suspended for the observability-operator app
# - Custom branch version deployed to the testing installation
#
# The script will:
# 1. Create a test workload cluster named 'ollyoptest'
# 2. Verify metric collector deployment and configuration
# 3. Check that ConfigMaps and Secrets are created properly
#
# After running, wait ~10 minutes then verify:
# - Dashboards show data from all WCs including 'ollyoptest'
# - Check for new alerts on the 'Alerts timeline' dashboard

# Helper function - prints an error message and exits
exit_error() {
  echo "Error: $*"
  exit 1
}

# Helper function - clean up the WC
clean_wc() {
  kubectl delete -f grizzly-e2e-wc.yaml
  rm grizzly-e2e-wc.yaml
}

# Helper function - checks the existence of the cm and secret for alloy
check_configs() {
  echo "Checking if the corresponding $1-$2 has been created"
  local config

  [[ "$2" == "config" ]] \
    && config=$(kubectl get configmap -n org-giantswarm ollyoptest-$1-$2) || config=$(kubectl get secret -n org-giantswarm ollyoptest-$1-$2)

  [[ -z "$config" ]] && echo "$1-$2 not found" || echo "$1-$2 found"
}

main() {
  [[ -z "$1" ]] && exit_error "Please provide the installation name as an argument"

  # Logging into the specified installation to perform the tests
  tsh kube login $1

  echo "Checking if observability-operator app is in deployed state"

  status=$(kubectl get app -n giantswarm observability-operator -o yaml | yq .status.release.status)

  [[ "$status" != "deployed" ]] \
    && exit_error "observability-operator app is not in deployed state. Please fix the app before retrying" || echo "observability-operator app is indeed in deployed state"

  echo "Creating WC"

  # Getting latest Giant Swarm release version
  toUseRelease="$(kubectl gs get releases -o template='{{range .items}}{{.status.ready}}/{{.metadata.name}}
  {{end}}' | sed -ne 's/false\/aws-//p' | sort -V | tail -1)"

  # Creating WC template and applying it
  kubectl gs template cluster --provider capa --name ollyoptest --organization giantswarm --description "observability-operator e2e tests" --release $toUseRelease > grizzly-e2e-wc.yaml
  kubectl create -f grizzly-e2e-wc.yaml

  echo "WC named 'ollyoptest' created. Waiting for it and its apps to be ready"
  
  # Waiting for 1min for the cluster resource to be created
  sleep 60

  kubectl wait -n org-giantswarm --for=condition=Ready cluster/ollyoptest --timeout=10m
  kubectl wait -n org-giantswarm --for=jsonpath='{.status.release.status}'=deployed app/ollyoptest-observability-bundle --timeout=20m

  # Giving extra time for the Alloy app to be created
  sleep 60

  echo "Checking if the monitoring agent is up and running on the WC"

  # Logging into the WC to get the context into the kubeconfig
  tsh kube login $1-ollyoptest
  tsh kube login $1

  alloy=$(kubectl get apps -n org-giantswarm | grep ollyoptest-alloy-metrics)

  if [[ ! -z "$alloy" ]]; then
    local podStatus=$(kubectl get pods -n kube-system --context teleport.giantswarm.io-$1-ollyoptest alloy-metrics-0 -o yaml | yq .status.phase)

    [[ "$podStatus" != "Running" ]] && echo "alloy app deployed but pod isn't in a running state" || echo "alloy app is deployed and pods are running"

    check_configs "monitoring" "config"
    check_configs "monitoring" "secret"
  else
    echo "No monitoring agent app found. Cleaning the WC"
    clean_wc
    exit 1
  fi

  echo "Test succeeded, cleaning WC"

  clean_wc
}

main "$@"
